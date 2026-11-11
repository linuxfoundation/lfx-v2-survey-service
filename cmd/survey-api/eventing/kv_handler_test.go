// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

// fakeMsg is a jetstream.Msg that records the acknowledgement it receives.
// Unimplemented methods panic (nil embedded interface).
type fakeMsg struct {
	jetstream.Msg
	subject     string
	data        []byte
	headers     nats.Header
	delivered   uint64
	metadataErr error

	acked     bool
	naked     bool
	nakDelay  time.Duration
	ackErr    error
	nakErr    error
	metaCalls int
}

func (m *fakeMsg) Subject() string      { return m.subject }
func (m *fakeMsg) Data() []byte         { return m.data }
func (m *fakeMsg) Headers() nats.Header { return m.headers }
func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) {
	m.metaCalls++
	if m.metadataErr != nil {
		return nil, m.metadataErr
	}
	return &jetstream.MsgMetadata{NumDelivered: m.delivered}, nil
}
func (m *fakeMsg) Ack() error { m.acked = true; return m.ackErr }
func (m *fakeMsg) NakWithDelay(d time.Duration) error {
	m.naked = true
	m.nakDelay = d
	return m.nakErr
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// ─── decodeKVValue ───────────────────────────────────────────────────────────

func TestDecodeKVData(t *testing.T) {
	t.Run("decodes JSON", func(t *testing.T) {
		payload := map[string]any{"name": "Member Survey 2025"}
		raw, err := json.Marshal(payload)
		require.NoError(t, err)

		got, err := decodeKVValue(raw)
		require.NoError(t, err)
		assert.Equal(t, "Member Survey 2025", got["name"])
	})

	t.Run("decodes msgpack", func(t *testing.T) {
		payload := map[string]any{"name": "Member Survey 2025"}
		raw, err := msgpack.Marshal(payload)
		require.NoError(t, err)

		got, err := decodeKVValue(raw)
		require.NoError(t, err)
		assert.Equal(t, "Member Survey 2025", got["name"])
	})

	t.Run("returns combined error when both decoders fail", func(t *testing.T) {
		_, err := decodeKVValue([]byte("not-json-or-msgpack"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "json:")
		assert.Contains(t, err.Error(), "msgpack:")
	})
}

// ─── isTombstonedMapping / kvEntry ───────────────────────────────────────────

func TestIsTombstonedMapping(t *testing.T) {
	assert.True(t, isTombstonedMapping([]byte("!del")))
	assert.False(t, isTombstonedMapping([]byte("1")))
	assert.False(t, isTombstonedMapping([]byte("!del ")))
	assert.False(t, isTombstonedMapping(nil))
}

func TestKVEntry(t *testing.T) {
	e := &kvEntry{key: "itx-surveys.s1", value: []byte("v"), operation: jetstream.KeyValueDelete}

	assert.Equal(t, "itx-surveys.s1", e.Key())
	assert.Equal(t, []byte("v"), e.Value())
	assert.Equal(t, jetstream.KeyValueDelete, e.Operation())
	assert.Equal(t, V1ObjectsBucket, e.Bucket())
	assert.WithinDuration(t, time.Now(), e.Created(), time.Minute)
	assert.Zero(t, e.Delta())
	assert.Zero(t, e.Revision())
}

// ─── kvHandler ───────────────────────────────────────────────────────────────

func TestKVHandler_RoutesByOperation(t *testing.T) {
	ctx := context.Background()
	template := mustJSON(t, templateV1Data())

	tests := []struct {
		name       string
		op         jetstream.KeyValueOp
		wantAction string // "" means nothing published
	}{
		{"put publishes created", jetstream.KeyValuePut, "created"},
		{"delete publishes deleted", jetstream.KeyValueDelete, "deleted"},
		{"purge publishes deleted", jetstream.KeyValuePurge, "deleted"},
		{"unknown operation is ignored", jetstream.KeyValueOp(99), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, events := newRecordingPublisher(nil)
			entry := &kvEntry{key: "surveymonkey-surveys.412349483", value: template, operation: tt.op}

			retry := kvHandler(ctx, entry, pub, &mocks.MockIDMapper{}, newFakeKV(nil), nil, nil, discardLogger())

			assert.False(t, retry)
			if tt.wantAction == "" {
				assert.Empty(t, *events)
				return
			}
			require.Len(t, *events, 1)
			assert.Equal(t, tt.wantAction, (*events)[0].Action)
		})
	}
}

// ─── handleKVPut ─────────────────────────────────────────────────────────────

func TestHandleKVPut(t *testing.T) {
	ctx := context.Background()
	put := func(key string, value []byte) *kvEntry {
		return &kvEntry{key: key, value: value, operation: jetstream.KeyValuePut}
	}

	t.Run("an undecodable value is acknowledged without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		retry := handleKVPut(ctx, put("itx-surveys.s1", []byte("not-json-or-msgpack")), pub,
			v1IDMapper(), newFakeKV(nil), nil, nil, discardLogger())

		assert.False(t, retry)
		assert.Empty(t, *events)
	})

	t.Run("itx-surveys routes to the survey handler", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		handleKVPut(ctx, put("itx-surveys.survey-1", mustJSON(t, surveyV1Data())), pub,
			v1IDMapper(), newFakeKV(nil), nil, nil, discardLogger())

		require.Len(t, *events, 1)
		require.NotNil(t, (*events)[0].Survey)
		assert.Equal(t, "survey-1", (*events)[0].Survey.UID)
	})

	t.Run("surveymonkey-surveys routes to the template handler", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		handleKVPut(ctx, put("surveymonkey-surveys.412349483", mustJSON(t, templateV1Data())), pub,
			&mocks.MockIDMapper{}, newFakeKV(nil), nil, nil, discardLogger())

		require.Len(t, *events, 1)
		require.NotNil(t, (*events)[0].Template)
		assert.Equal(t, "412349483", (*events)[0].Template.ID)
	})

	t.Run("a msgpack value is decoded and routed", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		raw, err := msgpack.Marshal(templateV1Data())
		require.NoError(t, err)

		handleKVPut(ctx, put("surveymonkey-surveys.412349483", raw), pub,
			&mocks.MockIDMapper{}, newFakeKV(nil), nil, nil, discardLogger())

		require.Len(t, *events, 1)
		assert.Equal(t, "Board Member NPS", (*events)[0].Template.Title)
	})

	t.Run("an unsupported prefix is skipped", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		retry := handleKVPut(ctx, put("itx-meetings.m1", mustJSON(t, map[string]any{"id": "m1"})), pub,
			&mocks.MockIDMapper{}, newFakeKV(nil), nil, nil, discardLogger())

		assert.False(t, retry)
		assert.Empty(t, *events)
	})

	t.Run("a key without a period uses the whole key as the prefix", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		handleKVPut(ctx, put("itx-surveys", mustJSON(t, surveyV1Data())), pub,
			v1IDMapper(), newFakeKV(nil), nil, nil, discardLogger())
		assert.Len(t, *events, 1, "the bare prefix still routes, since prefix == key")
	})

	t.Run("a soft-deleted record is routed to the delete handler", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)
		data := surveyV1Data()
		data["_sdc_deleted_at"] = "2030-02-01T00:00:00Z"

		retry := handleKVPut(ctx, put("itx-surveys.survey-1", mustJSON(t, data)), pub,
			v1IDMapper(), kv, nil, nil, discardLogger())

		assert.False(t, retry)
		require.Len(t, *events, 1)
		assert.Equal(t, "deleted", (*events)[0].Action)
		assert.Equal(t, "survey-1", (*events)[0].Survey.UID)
		assert.Equal(t, tombstoneMarker, kv.value("survey.survey-1"))
	})

	t.Run("an empty or null _sdc_deleted_at is not a soft delete", func(t *testing.T) {
		for name, value := range map[string]any{"empty string": "", "null": nil} {
			t.Run(name, func(t *testing.T) {
				pub, events := newRecordingPublisher(nil)
				data := surveyV1Data()
				data["_sdc_deleted_at"] = value

				handleKVPut(ctx, put("itx-surveys.survey-1", mustJSON(t, data)), pub,
					v1IDMapper(), newFakeKV(nil), nil, nil, discardLogger())

				require.Len(t, *events, 1)
				assert.Equal(t, "created", (*events)[0].Action)
			})
		}
	})

	t.Run("the retry decision of the routed handler is returned", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nats.ErrTimeout)

		retry := handleKVPut(ctx, put("surveymonkey-surveys.412349483", mustJSON(t, templateV1Data())), pub,
			&mocks.MockIDMapper{}, newFakeKV(nil), nil, nil, discardLogger())

		assert.True(t, retry)
	})
}

// ─── handleKVDelete / handleResourceDelete ───────────────────────────────────

func TestHandleKVDelete(t *testing.T) {
	ctx := context.Background()
	del := func(key string) *kvEntry {
		return &kvEntry{key: key, operation: jetstream.KeyValueDelete}
	}

	t.Run("itx-surveys deletes publish a survey deleted event", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		assert.False(t, handleKVDelete(ctx, del("itx-surveys.survey-1"), pub, newFakeKV(nil), discardLogger()))

		require.Len(t, *events, 1)
		assert.Equal(t, "deleted", (*events)[0].Action)
		assert.Equal(t, "survey-1", (*events)[0].Survey.UID)
	})

	t.Run("surveymonkey-surveys deletes publish a template deleted event", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		handleKVDelete(ctx, del("surveymonkey-surveys.t1"), pub, newFakeKV(nil), discardLogger())

		require.Len(t, *events, 1)
		assert.Equal(t, "t1", (*events)[0].Template.ID)
	})

	t.Run("everything after the first period is the uid", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		handleKVDelete(ctx, del("itx-surveys.a.b.c"), pub, newFakeKV(nil), discardLogger())

		require.Len(t, *events, 1)
		assert.Equal(t, "a.b.c", (*events)[0].Survey.UID)
	})

	t.Run("a key with no uid is acknowledged without publishing", func(t *testing.T) {
		for _, key := range []string{"itx-surveys", "itx-surveys.", ""} {
			t.Run("key="+key, func(t *testing.T) {
				pub, events := newRecordingPublisher(nil)

				assert.False(t, handleKVDelete(ctx, del(key), pub, newFakeKV(nil), discardLogger()))
				assert.Empty(t, *events)
			})
		}
	})

	t.Run("an unsupported prefix is acknowledged without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		assert.False(t, handleKVDelete(ctx, del("itx-meetings.m1"), pub, newFakeKV(nil), discardLogger()))
		assert.Empty(t, *events)
	})

	t.Run("a transient publish failure asks for a retry", func(t *testing.T) {
		pub, _ := newRecordingPublisher(errors.New("request timeout"))

		assert.True(t, handleKVDelete(ctx, del("itx-surveys.survey-1"), pub, newFakeKV(nil), discardLogger()))
	})
}

// ─── kvMessageHandler ────────────────────────────────────────────────────────

func TestKVMessageHandler(t *testing.T) {
	ctx := context.Background()
	template := mustJSON(t, templateV1Data())
	const templateSubject = "$KV.v1-objects.surveymonkey-surveys.412349483"

	run := func(msg *fakeMsg, pubErr error) *[]publishedEvent {
		pub, events := newRecordingPublisher(pubErr)
		kvMessageHandler(ctx, msg, pub, &mocks.MockIDMapper{}, newFakeKV(nil), nil, nil, discardLogger())
		return events
	}

	t.Run("a PUT is processed, keyed from the subject, and acknowledged", func(t *testing.T) {
		msg := &fakeMsg{subject: templateSubject, data: template, headers: nats.Header{}}

		events := run(msg, nil)

		require.Len(t, *events, 1)
		assert.Equal(t, "created", (*events)[0].Action)
		assert.Equal(t, "412349483", (*events)[0].Template.ID)
		assert.True(t, msg.acked)
		assert.False(t, msg.naked)
	})

	t.Run("KV-Operation DEL and PURGE are treated as deletes", func(t *testing.T) {
		for _, op := range []string{"DEL", "PURGE"} {
			t.Run(op, func(t *testing.T) {
				msg := &fakeMsg{subject: templateSubject, headers: nats.Header{"KV-Operation": []string{op}}}

				events := run(msg, nil)

				require.Len(t, *events, 1)
				assert.Equal(t, "deleted", (*events)[0].Action)
				assert.Equal(t, "412349483", (*events)[0].Template.ID)
				assert.True(t, msg.acked)
			})
		}
	})

	t.Run("an unrecognised KV-Operation header falls back to PUT", func(t *testing.T) {
		msg := &fakeMsg{subject: templateSubject, data: template, headers: nats.Header{"KV-Operation": []string{"WAT"}}}

		events := run(msg, nil)

		require.Len(t, *events, 1)
		assert.Equal(t, "created", (*events)[0].Action)
	})

	t.Run("a subject without a key is acknowledged and publishes nothing", func(t *testing.T) {
		msg := &fakeMsg{subject: "$KV.v1-objects.", data: template, headers: nats.Header{}}

		events := run(msg, nil)

		assert.Empty(t, *events)
		assert.True(t, msg.acked)
	})

	t.Run("a permanent failure is acknowledged, never retried", func(t *testing.T) {
		msg := &fakeMsg{subject: templateSubject, data: template, headers: nats.Header{}}

		run(msg, errors.New("invalid payload"))

		assert.True(t, msg.acked)
		assert.False(t, msg.naked)
		assert.Zero(t, msg.metaCalls, "metadata is only read when retrying")
	})

	t.Run("a transient failure is NAKed with exponential backoff by delivery count", func(t *testing.T) {
		tests := []struct {
			delivered uint64
			wantDelay time.Duration
		}{
			{1, 2 * time.Second},
			{2, 10 * time.Second},
			{3, 20 * time.Second},
			{9, 20 * time.Second},
		}
		for _, tt := range tests {
			msg := &fakeMsg{subject: templateSubject, data: template, headers: nats.Header{}, delivered: tt.delivered}

			run(msg, nats.ErrTimeout)

			assert.True(t, msg.naked, "delivery %d", tt.delivered)
			assert.False(t, msg.acked, "delivery %d", tt.delivered)
			assert.Equal(t, tt.wantDelay, msg.nakDelay, "delivery %d", tt.delivered)
		}
	})

	t.Run("unreadable metadata falls back to the first-retry delay", func(t *testing.T) {
		msg := &fakeMsg{subject: templateSubject, data: template, headers: nats.Header{}, metadataErr: errors.New("no metadata")}

		run(msg, nats.ErrTimeout)

		assert.True(t, msg.naked)
		assert.Equal(t, 2*time.Second, msg.nakDelay)
	})

	t.Run("ack and nak failures are logged, not propagated", func(t *testing.T) {
		ackFail := &fakeMsg{subject: templateSubject, data: template, headers: nats.Header{}, ackErr: errors.New("ack failed")}
		assert.NotPanics(t, func() { run(ackFail, nil) })
		assert.True(t, ackFail.acked)

		nakFail := &fakeMsg{subject: templateSubject, data: template, headers: nats.Header{}, delivered: 1, nakErr: errors.New("nak failed")}
		assert.NotPanics(t, func() { run(nakFail, nats.ErrTimeout) })
		assert.True(t, nakFail.naked)
	})
}
