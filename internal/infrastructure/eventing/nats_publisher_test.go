// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	indexerTypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func startTestNATSServer(t *testing.T) (*server.Server, string) {
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
	}

	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()

	if !ns.ReadyForConnections(4 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	return ns, ns.ClientURL()
}

func setupTestPublisher(t *testing.T) (*NATSPublisher, *nats.Conn, func()) {
	ns, url := startTestNATSServer(t)

	nc, err := nats.Connect(url)
	require.NoError(t, err)

	publisher := NewNATSPublisher(nc, slog.Default())

	cleanup := func() {
		nc.Close()
		ns.Shutdown()
	}

	return publisher, nc, cleanup
}

func TestIsValidLFXUsername(t *testing.T) {
	tests := []struct {
		username string
		want     bool
	}{
		{username: "testuser", want: true},
		{username: "user.name-123", want: true},
		{username: "", want: false},
		{username: "auth0|user", want: false},
		{username: "bad:user", want: false},
		{username: "has*star", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidLFXUsername(tt.username))
		})
	}
}

func TestSendSurveyResponseAccessMessage(t *testing.T) {
	tests := []struct {
		name          string
		data          *domain.SurveyResponseData
		wantPublished bool
		wantOwner     []string
		wantSurveyRef []string
	}{
		{
			name: "sets owner relation for valid username",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Username:  "testuser",
				SurveyUID: "survey-1",
			},
			wantPublished: true,
			wantOwner:     []string{"testuser"},
			wantSurveyRef: []string{"survey-1"},
		},
		{
			name: "omits owner relation for empty username",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				SurveyUID: "survey-1",
			},
			wantPublished: true,
			wantOwner:     nil,
			wantSurveyRef: []string{"survey-1"},
		},
		{
			name: "skips owner relation for invalid username",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Username:  "auth0|legacy",
				SurveyUID: "survey-1",
			},
			wantPublished: true,
			wantOwner:     nil,
			wantSurveyRef: []string{"survey-1"},
		},
		{
			name: "skips publish when username and survey UID are empty",
			data: &domain.SurveyResponseData{
				UID: "sr-1",
			},
			wantPublished: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher, nc, cleanup := setupTestPublisher(t)
			defer cleanup()

			sub, err := nc.SubscribeSync(fgaconstants.GenericUpdateAccessSubject)
			require.NoError(t, err)

			err = publisher.sendSurveyResponseAccessMessage(context.Background(), tt.data)
			require.NoError(t, err)
			require.NoError(t, nc.Flush())

			msg, err := sub.NextMsg(250 * time.Millisecond)
			if !tt.wantPublished {
				assert.ErrorIs(t, err, nats.ErrTimeout)
				return
			}

			require.NoError(t, err)

			var accessMsg fgatypes.GenericFGAMessage
			err = json.Unmarshal(msg.Data, &accessMsg)
			require.NoError(t, err)

			assert.Equal(t, "survey_response", accessMsg.ObjectType)
			assert.Equal(t, "update_access", accessMsg.Operation)

			var accessData fgatypes.GenericAccessData
			err = accessMsg.UnmarshalData(&accessData)
			require.NoError(t, err)

			assert.Equal(t, tt.data.UID, accessData.UID)
			assert.Equal(t, tt.wantOwner, accessData.Relations["owner"])
			assert.Equal(t, tt.wantSurveyRef, accessData.References["survey"])
		})
	}
}

func TestBuildHeaders_TypedContextKeys(t *testing.T) {
	p := &NATSPublisher{}

	t.Run("fallback when context is empty", func(t *testing.T) {
		headers := p.buildHeaders(context.Background())
		assert.Equal(t, "Bearer survey-service", headers["authorization"])
		assert.Empty(t, headers["x-on-behalf-of"])
	})

	t.Run("extracts typed authorization and principal keys", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), constants.AuthorizationContextID, "Bearer real-token")
		ctx = context.WithValue(ctx, constants.PrincipalContextID, "user-123")

		headers := p.buildHeaders(ctx)
		assert.Equal(t, "Bearer real-token", headers["authorization"])
		assert.Equal(t, "user-123", headers["x-on-behalf-of"])
	})

	t.Run("untyped string keys do NOT match (regression guard)", func(t *testing.T) {
		// Storing under plain string must NOT be picked up — only the typed constant works.
		type plainKey string
		ctx := context.WithValue(context.Background(), plainKey("authorization"), "Bearer wrong")
		ctx = context.WithValue(ctx, plainKey("principal"), "wrong-user")

		headers := p.buildHeaders(ctx)
		assert.Equal(t, "Bearer survey-service", headers["authorization"], "plain-string key must not match typed contextKey")
		assert.Empty(t, headers["x-on-behalf-of"], "plain-string key must not match typed contextKey")
	})
}

// ─── harness ─────────────────────────────────────────────────────────────────

// publisherHarness runs a publisher against an in-process NATS server and
// captures what lands on each subject the publisher can write to.
type publisherHarness struct {
	t    *testing.T
	pub  *NATSPublisher
	nc   *nats.Conn
	subs map[string]*nats.Subscription
}

func newPublisherHarness(t *testing.T) *publisherHarness {
	t.Helper()
	ns, url := startTestNATSServer(t)
	t.Cleanup(ns.Shutdown)
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	h := &publisherHarness{t: t, pub: NewNATSPublisher(nc, slog.Default()), nc: nc, subs: map[string]*nats.Subscription{}}
	for _, subj := range []string{
		IndexSurveySubject, IndexSurveyResponseSubject, IndexSurveyTemplateSubject,
		fgaconstants.GenericUpdateAccessSubject, fgaconstants.GenericDeleteAccessSubject,
	} {
		sub, err := nc.SubscribeSync(subj)
		require.NoError(t, err)
		h.subs[subj] = sub
	}
	return h
}

// next returns the next message on subject, failing the test if none arrives.
func (h *publisherHarness) next(subject string) *nats.Msg {
	h.t.Helper()
	require.NoError(h.t, h.nc.Flush())
	msg, err := h.subs[subject].NextMsg(2 * time.Second)
	require.NoError(h.t, err, "expected a message on %s", subject)
	return msg
}

// expectNone asserts that nothing is published on subject.
func (h *publisherHarness) expectNone(subject string) {
	h.t.Helper()
	require.NoError(h.t, h.nc.Flush())
	_, err := h.subs[subject].NextMsg(150 * time.Millisecond)
	assert.ErrorIs(h.t, err, nats.ErrTimeout, "unexpected message on %s", subject)
}

// indexerMsg is the indexer envelope with the payload left undecoded.
type indexerMsg struct {
	Action         string                       `json:"action"`
	Headers        map[string]string            `json:"headers"`
	Data           json.RawMessage              `json:"data"`
	IndexingConfig *indexerTypes.IndexingConfig `json:"indexing_config"`
}

func decodeIndexerMsg(t *testing.T, msg *nats.Msg) indexerMsg {
	t.Helper()
	var env indexerMsg
	require.NoError(t, json.Unmarshal(msg.Data, &env))
	require.NotNil(t, env.IndexingConfig)
	return env
}

func decodeFGAMsg(t *testing.T, msg *nats.Msg) (fgatypes.GenericFGAMessage, fgatypes.GenericAccessData) {
	t.Helper()
	var m fgatypes.GenericFGAMessage
	require.NoError(t, json.Unmarshal(msg.Data, &m))
	var data fgatypes.GenericAccessData
	require.NoError(t, m.UnmarshalData(&data))
	return m, data
}

func boolPtr(b bool) *bool { return &b }

// ─── PublishSurveyEvent ──────────────────────────────────────────────────────

func TestPublishSurveyEvent(t *testing.T) {
	ctx := context.Background()
	survey := func() *domain.SurveyData {
		return &domain.SurveyData{
			UID: "survey-1", SurveyTitle: "Q4 NPS",
			Committees: []domain.SurveyCommitteeData{
				{CommitteeUID: "com-1", ProjectUID: "proj-1"},
				{CommitteeUID: "com-2", ProjectUID: "proj-1"}, // same project: referenced once
				{CommitteeUID: "com-3", ProjectUID: "proj-2"},
			},
		}
	}

	t.Run("created publishes the indexed document and the access references", func(t *testing.T) {
		h := newPublisherHarness(t)

		require.NoError(t, h.pub.PublishSurveyEvent(ctx, "created", survey()))

		env := decodeIndexerMsg(t, h.next(IndexSurveySubject))
		assert.Equal(t, "created", env.Action)
		var doc domain.SurveyData
		require.NoError(t, json.Unmarshal(env.Data, &doc))
		assert.Equal(t, *survey(), doc, "the full survey is the document body")
		assert.Equal(t, &indexerTypes.IndexingConfig{
			ObjectID:             "survey-1",
			Public:               boolPtr(false),
			AccessCheckObject:    "survey:survey-1",
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   "survey:survey-1",
			HistoryCheckRelation: "auditor",
			SortName:             "Q4 NPS",
			NameAndAliases:       []string{"Q4 NPS"},
			ParentRefs:           []string{"committee:com-1", "project:proj-1", "committee:com-2", "committee:com-3", "project:proj-2"},
			Tags:                 []string{"committee_uid:com-1", "project_uid:proj-1", "committee_uid:com-2", "committee_uid:com-3", "project_uid:proj-2"},
			Fulltext:             "Q4 NPS",
		}, env.IndexingConfig)

		fga, access := decodeFGAMsg(t, h.next(fgaconstants.GenericUpdateAccessSubject))
		assert.Equal(t, "survey", fga.ObjectType)
		assert.Equal(t, "update_access", fga.Operation)
		assert.Equal(t, "survey-1", access.UID)
		assert.False(t, access.Public)
		assert.Equal(t, map[string][]string{
			"committee": {"com-1", "com-2", "com-3"},
			"project":   {"proj-1", "proj-2"},
		}, access.References)
		h.expectNone(fgaconstants.GenericDeleteAccessSubject)
	})

	t.Run("updated uses the same payloads with the updated action", func(t *testing.T) {
		h := newPublisherHarness(t)

		require.NoError(t, h.pub.PublishSurveyEvent(ctx, "updated", survey()))

		assert.Equal(t, "updated", decodeIndexerMsg(t, h.next(IndexSurveySubject)).Action)
		h.next(fgaconstants.GenericUpdateAccessSubject)
	})

	t.Run("a survey with no committees is indexed without an access message", func(t *testing.T) {
		h := newPublisherHarness(t)
		s := &domain.SurveyData{UID: "survey-1"}

		require.NoError(t, h.pub.PublishSurveyEvent(ctx, "created", s))

		env := decodeIndexerMsg(t, h.next(IndexSurveySubject))
		assert.Empty(t, env.IndexingConfig.NameAndAliases, "no title means no alias")
		assert.Empty(t, env.IndexingConfig.ParentRefs)
		assert.Empty(t, env.IndexingConfig.Tags)
		h.expectNone(fgaconstants.GenericUpdateAccessSubject)
	})

	t.Run("deleted sends only the uid to the indexer and a delete_access message", func(t *testing.T) {
		h := newPublisherHarness(t)

		s := survey()
		s.ID = "v1-survey-1" // differs from UID so using the wrong one is caught
		require.NoError(t, h.pub.PublishSurveyEvent(ctx, "deleted", s))

		env := decodeIndexerMsg(t, h.next(IndexSurveySubject))
		assert.Equal(t, "deleted", env.Action)
		var uid string
		require.NoError(t, json.Unmarshal(env.Data, &uid))
		assert.Equal(t, "survey-1", uid)
		assert.Nil(t, env.IndexingConfig.Public, "delete messages carry no public flag")
		assert.Equal(t, "survey:survey-1", env.IndexingConfig.AccessCheckObject)

		var del fgatypes.GenericFGAMessage
		require.NoError(t, json.Unmarshal(h.next(fgaconstants.GenericDeleteAccessSubject).Data, &del))
		assert.Equal(t, "survey", del.ObjectType)
		assert.Equal(t, "delete_access", del.Operation)
		var data fgatypes.GenericDeleteData
		require.NoError(t, del.UnmarshalData(&data))
		assert.Equal(t, "survey-1", data.UID)
		h.expectNone(fgaconstants.GenericUpdateAccessSubject)
	})

	t.Run("an indexer publish failure is returned and nothing reaches FGA", func(t *testing.T) {
		h := newPublisherHarness(t)
		h.nc.Close()

		err := h.pub.PublishSurveyEvent(ctx, "created", survey())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send survey indexer message")
		assert.Contains(t, err.Error(), "failed to publish to subject "+IndexSurveySubject)
	})
}

// ─── PublishSurveyResponseEvent ──────────────────────────────────────────────

func TestPublishSurveyResponseEvent(t *testing.T) {
	ctx := context.Background()
	response := func() *domain.SurveyResponseData {
		return &domain.SurveyResponseData{
			UID: "resp-1", Email: "alice@example.com", FirstName: "Alice", LastName: "Example",
			Username: "alice", SurveyUID: "survey-1", CommitteeUID: "com-1",
			Project: domain.SurveyResponseProjectData{ProjectUID: "proj-1"},
		}
	}

	t.Run("created publishes the indexed document and the owner access message", func(t *testing.T) {
		h := newPublisherHarness(t)

		require.NoError(t, h.pub.PublishSurveyResponseEvent(ctx, "created", response()))

		env := decodeIndexerMsg(t, h.next(IndexSurveyResponseSubject))
		assert.Equal(t, "created", env.Action)
		var doc domain.SurveyResponseData
		require.NoError(t, json.Unmarshal(env.Data, &doc))
		assert.Equal(t, *response(), doc)
		assert.Equal(t, &indexerTypes.IndexingConfig{
			ObjectID:             "resp-1",
			Public:               boolPtr(false),
			AccessCheckObject:    "survey_response:resp-1",
			AccessCheckRelation:  "auditor",
			HistoryCheckObject:   "survey_response:resp-1",
			HistoryCheckRelation: "auditor",
			SortName:             "alice@example.com",
			NameAndAliases:       []string{"alice@example.com"},
			ParentRefs:           []string{"project:proj-1", "committee:com-1", "survey:survey-1"},
			Tags:                 []string{"project_uid:proj-1", "committee_uid:com-1", "survey_uid:survey-1"},
			Fulltext:             "alice@example.com Alice Example",
		}, env.IndexingConfig)

		fga, access := decodeFGAMsg(t, h.next(fgaconstants.GenericUpdateAccessSubject))
		assert.Equal(t, "survey_response", fga.ObjectType)
		assert.Equal(t, "resp-1", access.UID)
		assert.Equal(t, []string{"alice"}, access.Relations["owner"])
		assert.Equal(t, []string{"survey-1"}, access.References["survey"])
	})

	t.Run("only the references that are present are indexed", func(t *testing.T) {
		h := newPublisherHarness(t)
		r := &domain.SurveyResponseData{UID: "resp-1", SurveyUID: "survey-1"}

		require.NoError(t, h.pub.PublishSurveyResponseEvent(ctx, "updated", r))

		env := decodeIndexerMsg(t, h.next(IndexSurveyResponseSubject))
		assert.Equal(t, []string{"survey:survey-1"}, env.IndexingConfig.ParentRefs)
		assert.Equal(t, []string{"survey_uid:survey-1"}, env.IndexingConfig.Tags)
		assert.Empty(t, env.IndexingConfig.NameAndAliases, "no email means no alias")
	})

	t.Run("deleted sends only the uid to the indexer and a delete_access message", func(t *testing.T) {
		h := newPublisherHarness(t)

		// ID (v1) and UID (v2) differ so using the wrong one is caught.
		require.NoError(t, h.pub.PublishSurveyResponseEvent(ctx, "deleted", &domain.SurveyResponseData{UID: "resp-1", ID: "v1-resp-1"}))

		env := decodeIndexerMsg(t, h.next(IndexSurveyResponseSubject))
		assert.Equal(t, "deleted", env.Action)
		var uid string
		require.NoError(t, json.Unmarshal(env.Data, &uid))
		assert.Equal(t, "resp-1", uid)

		var del fgatypes.GenericFGAMessage
		require.NoError(t, json.Unmarshal(h.next(fgaconstants.GenericDeleteAccessSubject).Data, &del))
		assert.Equal(t, "survey_response", del.ObjectType)
		assert.Equal(t, "delete_access", del.Operation)
		var data fgatypes.GenericDeleteData
		require.NoError(t, del.UnmarshalData(&data))
		assert.Equal(t, "resp-1", data.UID)
		h.expectNone(fgaconstants.GenericUpdateAccessSubject)
	})

	t.Run("an indexer publish failure is returned", func(t *testing.T) {
		h := newPublisherHarness(t)
		h.nc.Close()

		err := h.pub.PublishSurveyResponseEvent(ctx, "created", response())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send survey response indexer message")
		assert.Contains(t, err.Error(), "failed to publish to subject "+IndexSurveyResponseSubject)
	})
}

// ─── PublishSurveyTemplateEvent ──────────────────────────────────────────────

func TestPublishSurveyTemplateEvent(t *testing.T) {
	ctx := context.Background()
	const adminTeam = "team:global_survey_platform_admins"

	t.Run("created indexes the template for platform admins only", func(t *testing.T) {
		h := newPublisherHarness(t)
		tpl := &domain.SurveyTemplateData{
			ID: "tpl-1", Title: "Board NPS", Nickname: "Board", Href: "https://example.com/t/1",
			QuestionCount: 12, Language: "en", Category: "nps", FolderID: "folder-1", PageCount: 3,
		}

		require.NoError(t, h.pub.PublishSurveyTemplateEvent(ctx, "created", tpl))

		env := decodeIndexerMsg(t, h.next(IndexSurveyTemplateSubject))
		assert.Equal(t, "created", env.Action)
		var doc domain.SurveyTemplateData
		require.NoError(t, json.Unmarshal(env.Data, &doc))
		assert.Equal(t, *tpl, doc, "the full template is the document body")
		assert.Equal(t, &indexerTypes.IndexingConfig{
			ObjectID:             "tpl-1",
			Public:               boolPtr(false),
			AccessCheckObject:    adminTeam,
			AccessCheckRelation:  "member",
			HistoryCheckObject:   adminTeam,
			HistoryCheckRelation: "member",
			SortName:             "Board NPS",
			NameAndAliases:       []string{"Board NPS", "Board"},
			Fulltext:             "Board NPS Board",
		}, env.IndexingConfig)
		h.expectNone(fgaconstants.GenericUpdateAccessSubject)
		h.expectNone(fgaconstants.GenericDeleteAccessSubject)
	})

	t.Run("a nickname equal to the title is not repeated as an alias", func(t *testing.T) {
		h := newPublisherHarness(t)

		require.NoError(t, h.pub.PublishSurveyTemplateEvent(ctx, "updated",
			&domain.SurveyTemplateData{ID: "tpl-1", Title: "Board NPS", Nickname: "Board NPS"}))

		env := decodeIndexerMsg(t, h.next(IndexSurveyTemplateSubject))
		assert.Equal(t, []string{"Board NPS"}, env.IndexingConfig.NameAndAliases)
	})

	t.Run("an empty title and nickname produce no aliases", func(t *testing.T) {
		h := newPublisherHarness(t)

		require.NoError(t, h.pub.PublishSurveyTemplateEvent(ctx, "created", &domain.SurveyTemplateData{ID: "tpl-1"}))

		env := decodeIndexerMsg(t, h.next(IndexSurveyTemplateSubject))
		assert.Empty(t, env.IndexingConfig.NameAndAliases)
	})

	t.Run("deleted sends only the id to the indexer", func(t *testing.T) {
		h := newPublisherHarness(t)

		require.NoError(t, h.pub.PublishSurveyTemplateEvent(ctx, "deleted", &domain.SurveyTemplateData{ID: "tpl-1"}))

		env := decodeIndexerMsg(t, h.next(IndexSurveyTemplateSubject))
		assert.Equal(t, "deleted", env.Action)
		var id string
		require.NoError(t, json.Unmarshal(env.Data, &id))
		assert.Equal(t, "tpl-1", id)
		h.expectNone(fgaconstants.GenericDeleteAccessSubject)
	})

	t.Run("a publish failure is returned", func(t *testing.T) {
		h := newPublisherHarness(t)
		h.nc.Close()

		err := h.pub.PublishSurveyTemplateEvent(ctx, "created", &domain.SurveyTemplateData{ID: "tpl-1"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send survey template indexer message")
	})
}

// ─── message headers and tracing ─────────────────────────────────────────────

func TestPublishedMessages_CarryCallerContext(t *testing.T) {
	t.Run("the caller's authorization and principal go in the indexer envelope", func(t *testing.T) {
		h := newPublisherHarness(t)
		ctx := context.WithValue(context.Background(), constants.AuthorizationContextID, "Bearer caller-token")
		ctx = context.WithValue(ctx, constants.PrincipalContextID, "alice")

		require.NoError(t, h.pub.PublishSurveyTemplateEvent(ctx, "created", &domain.SurveyTemplateData{ID: "tpl-1"}))

		env := decodeIndexerMsg(t, h.next(IndexSurveyTemplateSubject))
		assert.Equal(t, map[string]string{"authorization": "Bearer caller-token", "x-on-behalf-of": "alice"}, env.Headers)
	})

	t.Run("system events fall back to the service identity", func(t *testing.T) {
		h := newPublisherHarness(t)

		require.NoError(t, h.pub.PublishSurveyTemplateEvent(context.Background(), "created", &domain.SurveyTemplateData{ID: "tpl-1"}))

		env := decodeIndexerMsg(t, h.next(IndexSurveyTemplateSubject))
		assert.Equal(t, map[string]string{"authorization": "Bearer survey-service"}, env.Headers)
	})

	t.Run("the trace context is propagated on indexer and FGA messages", func(t *testing.T) {
		prev := otel.GetTextMapPropagator()
		otel.SetTextMapPropagator(propagation.TraceContext{})
		t.Cleanup(func() { otel.SetTextMapPropagator(prev) })

		traceID, err := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
		require.NoError(t, err)
		spanID, err := trace.SpanIDFromHex("b7ad6b7169203331")
		require.NoError(t, err)
		ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, Remote: true,
		}))
		h := newPublisherHarness(t)

		require.NoError(t, h.pub.PublishSurveyEvent(ctx, "created", &domain.SurveyData{
			UID: "survey-1", Committees: []domain.SurveyCommitteeData{{CommitteeUID: "com-1"}},
		}))

		for _, subj := range []string{IndexSurveySubject, fgaconstants.GenericUpdateAccessSubject} {
			assert.Contains(t, h.next(subj).Header.Get("traceparent"), traceID.String(), subj)
		}
	})
}

// ─── Close / appendIfNotExists ───────────────────────────────────────────────

func TestNATSPublisher_Close(t *testing.T) {
	h := newPublisherHarness(t)

	assert.NoError(t, h.pub.Close())
	assert.False(t, h.nc.IsClosed(), "the connection belongs to the event processor and stays open")
}

func TestAppendIfNotExists(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		value string
		want  []string
	}{
		{"appends a new value", []string{"a"}, "b", []string{"a", "b"}},
		{"keeps an existing value once", []string{"a", "b"}, "a", []string{"a", "b"}},
		{"appends to an empty slice", nil, "a", []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, appendIfNotExists(tt.slice, tt.value))
		})
	}
}
