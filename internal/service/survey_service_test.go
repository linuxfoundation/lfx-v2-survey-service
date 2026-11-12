// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"io"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/idmapper"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func f64Ptr(f float64) *float64 { return &f }

func newTestService(responseClient *mocks.MockSurveyResponseClient) *service.SurveyService {
	mapper := idmapper.NewNoOpMapper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return service.NewSurveyService(&mocks.MockSurveyClient{}, &mocks.MockExclusionClient{}, responseClient, mapper, logger)
}

func newTemplatesTestService(fn func(ctx context.Context) ([]itx.SurveyTemplate, error)) *service.SurveyService {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return service.NewSurveyService(
		&mocks.MockSurveyClient{ListSurveyTemplatesFunc: fn},
		&mocks.MockExclusionClient{},
		&mocks.MockSurveyResponseClient{},
		idmapper.NewNoOpMapper(),
		logger,
	)
}

func TestListSurveyTemplates_DoesNotLogPrincipal(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	svc := service.NewSurveyService(
		&mocks.MockSurveyClient{ListSurveyTemplatesFunc: func(context.Context) ([]itx.SurveyTemplate, error) {
			return []itx.SurveyTemplate{}, nil
		}},
		&mocks.MockExclusionClient{},
		&mocks.MockSurveyResponseClient{},
		idmapper.NewNoOpMapper(),
		logger,
	)
	const principal = "template-log-principal"
	const projectUID = "7cad5a8d-19d0-41a4-81a6-043453daf9ee"
	ctx := context.WithValue(context.Background(), constants.PrincipalContextID, principal)
	_, err := svc.ListSurveyTemplates(ctx, &survey.ListSurveyTemplatesPayload{ProjectUID: projectUID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(logs.String(), principal) {
		t.Fatal("template-list logs contain the caller's username")
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected request and success logs, got %d entries", len(lines))
	}
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid log entry: %v", err)
		}
		if _, ok := entry["principal"]; ok {
			t.Error("template-list logs contain a principal field")
		}
		if i == 0 && (entry["msg"] != "listing survey templates" || entry["project_uid"] != projectUID) {
			t.Error("request log must preserve the message and project context")
		}
		if i == 1 && (entry["msg"] != "survey templates listed successfully" || entry["count"] != float64(0)) {
			t.Error("success log must preserve the message and template count")
		}
	}
}

func TestListSurveyTemplates_MapsFields(t *testing.T) {
	svc := newTemplatesTestService(func(ctx context.Context) ([]itx.SurveyTemplate, error) {
		return []itx.SurveyTemplate{
			{ID: "412349483", Title: "Board Member NPS", Nickname: "Board NPS", Href: "https://api.surveymonkey.com/v3/surveys/412349483"},
			{ID: "512349484", Title: "Contributor NPS"},
		}, nil
	})

	result, err := svc.ListSurveyTemplates(context.Background(), &survey.ListSurveyTemplatesPayload{
		ProjectUID:   "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
		CommitteeUID: strPtr("b03cdbaf-53b1-4d47-bc04-dd7e459dd309"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(result))
	}

	first := result[0]
	if first.ID != "412349483" || first.Title != "Board Member NPS" {
		t.Errorf("unexpected first template id/title: %q / %q", first.ID, first.Title)
	}
	if first.Nickname == nil || *first.Nickname != "Board NPS" {
		t.Errorf("expected nickname Board NPS, got %v", first.Nickname)
	}
	if first.Href == nil || *first.Href != "https://api.surveymonkey.com/v3/surveys/412349483" {
		t.Errorf("unexpected href: %v", first.Href)
	}

	second := result[1]
	if second.ID != "512349484" || second.Title != "Contributor NPS" {
		t.Errorf("unexpected second template id/title: %q / %q", second.ID, second.Title)
	}
	if second.Nickname != nil || second.Href != nil {
		t.Errorf("expected nil nickname/href for empty ITX values, got %v / %v", second.Nickname, second.Href)
	}
}

func TestListSurveyTemplates_EmptyList(t *testing.T) {
	svc := newTemplatesTestService(func(ctx context.Context) ([]itx.SurveyTemplate, error) {
		return []itx.SurveyTemplate{}, nil
	})

	result, err := svc.ListSurveyTemplates(context.Background(), &survey.ListSurveyTemplatesPayload{
		ProjectUID: "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil empty result")
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestListSurveyTemplates_PropagatesProxyError(t *testing.T) {
	proxyErr := domain.NewUnavailableError("ITX service request failed")
	svc := newTemplatesTestService(func(ctx context.Context) ([]itx.SurveyTemplate, error) {
		return nil, proxyErr
	})

	result, err := svc.ListSurveyTemplates(context.Background(), &survey.ListSurveyTemplatesPayload{
		ProjectUID: "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
	})
	if result != nil {
		t.Errorf("expected nil result on error, got %+v", result)
	}
	if !errors.Is(err, proxyErr) {
		t.Fatalf("expected proxy error returned unchanged, got %T: %v", err, err)
	}
}

func ctxWithPrincipal(principal string) context.Context {
	return context.WithValue(context.Background(), constants.PrincipalContextID, principal)
}

func newCreatorTestService(client *mocks.MockSurveyClient, names domain.UserNameReader) *service.SurveyService {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := service.NewSurveyService(client, &mocks.MockExclusionClient{}, &mocks.MockSurveyResponseClient{}, idmapper.NewNoOpMapper(), logger)
	if names != nil {
		svc.WithUserNameReader(names)
	}
	return svc
}

func TestScheduleSurvey_CreatorFromPrincipal(t *testing.T) {
	forgedPayload := func() *survey.ScheduleSurveyPayload {
		return &survey.ScheduleSurveyPayload{
			CommitteeUID:    "committee-1",
			CreatorUsername: strPtr("victim"),
			CreatorName:     strPtr("Victim Fixture"),
			CreatorID:       strPtr("victim-id"),
		}
	}

	var sent *itx.ScheduleSurveyRequest
	client := &mocks.MockSurveyClient{
		ScheduleSurveyFunc: func(_ context.Context, req *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
			sent = req
			return &itx.SurveyScheduleResponse{}, nil
		},
	}

	t.Run("stamps username, id and looked-up name over forged body values", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, &mocks.MockUserNameReader{Name: "Alice Example"})

		_, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), forgedPayload())

		require.NoError(t, err)
		require.NotNil(t, sent)
		assert.Equal(t, "alice", *sent.CreatorUsername)
		assert.Equal(t, "alice", *sent.CreatorID)
		assert.Equal(t, "Alice Example", *sent.CreatorName)
	})

	t.Run("falls back to the username when the name lookup fails", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, &mocks.MockUserNameReader{Err: errors.New("nats unavailable")})

		_, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), forgedPayload())

		require.NoError(t, err)
		assert.Equal(t, "alice", *sent.CreatorName, "forged name must not be used when the lookup fails")
	})

	t.Run("falls back to the username when no name reader is configured", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, nil)

		_, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), forgedPayload())

		require.NoError(t, err)
		assert.Equal(t, "alice", *sent.CreatorName)
	})

	t.Run("rejects the request without an authenticated principal", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, &mocks.MockUserNameReader{Name: "Alice Example"})

		_, err := svc.ScheduleSurvey(context.Background(), forgedPayload())

		var domErr *domain.DomainError
		require.ErrorAs(t, err, &domErr)
		assert.Equal(t, domain.ErrorTypeValidation, domErr.Type)
		assert.Nil(t, sent, "ITX must not be called")
	})
}

func TestUpdateSurvey_CreatorFromPrincipal(t *testing.T) {
	var sent *itx.UpdateSurveyRequest
	client := &mocks.MockSurveyClient{
		UpdateSurveyFunc: func(_ context.Context, _ string, req *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
			sent = req
			return &itx.SurveyScheduleResponse{}, nil
		},
	}

	t.Run("stamps username and id from the principal over a forged creator_id", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, nil)

		_, err := svc.UpdateSurvey(ctxWithPrincipal("alice"), &survey.UpdateSurveyPayload{
			SurveyUID: "survey-1",
			CreatorID: strPtr("victim-id"),
		})

		require.NoError(t, err)
		require.NotNil(t, sent)
		assert.Equal(t, "alice", *sent.CreatorUsername)
		assert.Equal(t, "alice", *sent.CreatorID)
	})

	t.Run("does not look up the display name on update", func(t *testing.T) {
		sent = nil
		names := &mocks.MockUserNameReader{
			DisplayNameByUsernameFunc: func(context.Context, string) (string, error) {
				t.Error("display name lookup must not run on update")
				return "", nil
			},
		}
		svc := newCreatorTestService(client, names)

		_, err := svc.UpdateSurvey(ctxWithPrincipal("alice"), &survey.UpdateSurveyPayload{SurveyUID: "survey-1"})

		require.NoError(t, err)
		assert.Equal(t, "alice", *sent.CreatorUsername)
	})

	t.Run("rejects the request without an authenticated principal", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, nil)

		_, err := svc.UpdateSurvey(context.Background(), &survey.UpdateSurveyPayload{SurveyUID: "survey-1"})

		var domErr *domain.DomainError
		require.ErrorAs(t, err, &domErr)
		assert.Equal(t, domain.ErrorTypeValidation, domErr.Type)
		assert.Nil(t, sent, "ITX must not be called")
	})
}

func intPtr(i int) *int { return &i }

func boolPtr(b bool) *bool { return &b }

// prefixMapper returns an ID mapper whose project and committee methods produce distinct,
// direction-tagged values ("v1-project-<id>", "v1-committee-<id>", "v2-project-<id>",
// "v2-committee-<id>"). A test can therefore tell from the value alone which mapper method
// the service called, so swapping the project and committee calls fails the suite.
// failV2/failV1 name IDs whose mapping returns errMapper.
var errMapper = errors.New("mapper unavailable")

func prefixMapper(failV2, failV1 string) *mocks.MockIDMapper {
	mapper := func(prefix, failID string) func(context.Context, string) (string, error) {
		return func(_ context.Context, id string) (string, error) {
			if id == failID {
				return "", errMapper
			}
			return prefix + id, nil
		}
	}
	return &mocks.MockIDMapper{
		MapProjectV2ToV1Func:   mapper("v1-project-", failV2),
		MapCommitteeV2ToV1Func: mapper("v1-committee-", failV2),
		MapProjectV1ToV2Func:   mapper("v2-project-", failV1),
		MapCommitteeV1ToV2Func: mapper("v2-committee-", failV1),
	}
}

func newServiceWith(client domain.SurveyClient, mapper domain.IDMapper) *service.SurveyService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return service.NewSurveyService(client, &mocks.MockExclusionClient{}, &mocks.MockSurveyResponseClient{}, mapper, logger)
}

func requireErrorType(t *testing.T, err error, want domain.ErrorType) {
	t.Helper()
	var domErr *domain.DomainError
	require.ErrorAs(t, err, &domErr)
	assert.Equal(t, want, domErr.Type)
}

func TestSurveyService_ServiceReady(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sc, ec, rc, mapper := &mocks.MockSurveyClient{}, &mocks.MockExclusionClient{}, &mocks.MockSurveyResponseClient{}, &mocks.MockIDMapper{}

	t.Run("all dependencies present", func(t *testing.T) {
		assert.NoError(t, service.NewSurveyService(sc, ec, rc, mapper, logger).ServiceReady())
	})

	tests := []struct {
		name string
		svc  *service.SurveyService
		want string
	}{
		{"missing survey client", service.NewSurveyService(nil, ec, rc, mapper, logger), "surveyClient"},
		{"missing exclusion client", service.NewSurveyService(sc, nil, rc, mapper, logger), "exclusionClient"},
		{"missing response client", service.NewSurveyService(sc, ec, nil, mapper, logger), "responseClient"},
		{"missing id mapper", service.NewSurveyService(sc, ec, rc, nil, logger), "idMapper"},
		{"missing logger", service.NewSurveyService(sc, ec, rc, mapper, nil), "logger"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.svc.ServiceReady()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestScheduleSurvey_MapsRequestAndResponse(t *testing.T) {
	var sent *itx.ScheduleSurveyRequest
	client := &mocks.MockSurveyClient{
		ScheduleSurveyFunc: func(_ context.Context, req *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
			sent = req
			return &itx.SurveyScheduleResponse{
				ID:                            "survey-1",
				SurveyMonkeyID:                strPtr("sm-1"),
				IsProjectSurvey:               boolPtr(true),
				StageFilter:                   strPtr("Active"),
				CreatorUsername:               strPtr("alice"),
				CreatorName:                   strPtr("Alice Example"),
				CreatorID:                     strPtr("alice"),
				CreatedAt:                     strPtr("2026-01-01T00:00:00Z"),
				LastModifiedAt:                strPtr("2026-01-02T00:00:00Z"),
				LastModifiedBy:                strPtr("bob"),
				SurveyTitle:                   strPtr("Annual Survey"),
				SurveyStatus:                  "scheduled",
				ResponseStatus:                strPtr("open"),
				SurveySendDate:                strPtr("2026-02-01T00:00:00Z"),
				SurveyCutoffDate:              strPtr("2026-03-01T00:00:00Z"),
				SurveyReminderRateDays:        intPtr(7),
				EmailSubject:                  strPtr("Subject"),
				EmailBody:                     strPtr("<p>Body</p>"),
				EmailBodyText:                 strPtr("Body"),
				CommitteeCategory:             strPtr("Board"),
				Committees:                    []itx.SurveyCommittee{{CommitteeName: strPtr("TSC"), CommitteeID: strPtr("c1"), ProjectID: strPtr("p1"), ProjectName: strPtr("Project One"), SurveyURL: strPtr("https://example.com/s"), TotalRecipients: intPtr(10), TotalResponses: intPtr(4), NPSValue: f64Ptr(42.5)}},
				CommitteeVotingEnabled:        boolPtr(true),
				SurveyURL:                     strPtr("https://example.com/survey"),
				SendImmediately:               boolPtr(false),
				TotalRecipients:               intPtr(10),
				TotalResponses:                intPtr(4),
				IsNPSSurvey:                   boolPtr(true),
				NPSValue:                      f64Ptr(42.5),
				NumPromoters:                  intPtr(5),
				NumPassives:                   intPtr(3),
				NumDetractors:                 intPtr(2),
				TotalBouncedEmails:            intPtr(1),
				NumAutomatedRemindersToSend:   intPtr(3),
				NumAutomatedRemindersSent:     intPtr(1),
				NextAutomatedReminderAt:       strPtr("2026-02-08T00:00:00Z"),
				LatestAutomatedReminderSentAt: strPtr("2026-02-01T00:00:00Z"),
			}, nil
		},
	}
	svc := newServiceWith(client, prefixMapper("", ""))

	got, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), &survey.ScheduleSurveyPayload{
		CommitteeUID:           "c-uid",
		IsProjectSurvey:        boolPtr(true),
		StageFilter:            strPtr("Active"),
		SurveyMonkeyID:         strPtr("sm-1"),
		SurveyTitle:            strPtr("Annual Survey"),
		SendImmediately:        boolPtr(false),
		SurveySendDate:         strPtr("2026-02-01T00:00:00Z"),
		SurveyCutoffDate:       strPtr("2026-03-01T00:00:00Z"),
		SurveyReminderRateDays: intPtr(7),
		EmailSubject:           strPtr("Subject"),
		EmailBody:              strPtr("<p>Body</p>"),
		EmailBodyText:          strPtr("Body"),
		CommitteeVotingEnabled: boolPtr(true),
	})
	require.NoError(t, err)

	t.Run("request: committee is mapped v2 to v1 and wrapped in a list, other fields forwarded", func(t *testing.T) {
		require.NotNil(t, sent)
		assert.Equal(t, []string{"v1-committee-c-uid"}, sent.Committees)
		assert.Equal(t, boolPtr(true), sent.IsProjectSurvey)
		assert.Equal(t, strPtr("Active"), sent.StageFilter)
		assert.Equal(t, strPtr("sm-1"), sent.SurveyMonkeyID)
		assert.Equal(t, strPtr("Annual Survey"), sent.SurveyTitle)
		assert.Equal(t, boolPtr(false), sent.SendImmediately)
		assert.Equal(t, strPtr("2026-02-01T00:00:00Z"), sent.SurveySendDate)
		assert.Equal(t, strPtr("2026-03-01T00:00:00Z"), sent.SurveyCutoffDate)
		assert.Equal(t, intPtr(7), sent.SurveyReminderRateDays)
		assert.Equal(t, strPtr("Subject"), sent.EmailSubject)
		assert.Equal(t, strPtr("<p>Body</p>"), sent.EmailBody)
		assert.Equal(t, strPtr("Body"), sent.EmailBodyText)
		assert.Equal(t, boolPtr(true), sent.CommitteeVotingEnabled)
	})

	t.Run("response: every ITX field is carried over and committee ids are mapped v1 to v2", func(t *testing.T) {
		want := &survey.SurveyScheduleResult{
			UID:                    "survey-1",
			SurveyMonkeyID:         strPtr("sm-1"),
			IsProjectSurvey:        boolPtr(true),
			StageFilter:            strPtr("Active"),
			CreatorUsername:        strPtr("alice"),
			CreatorName:            strPtr("Alice Example"),
			CreatorID:              strPtr("alice"),
			CreatedAt:              strPtr("2026-01-01T00:00:00Z"),
			LastModifiedAt:         strPtr("2026-01-02T00:00:00Z"),
			LastModifiedBy:         strPtr("bob"),
			SurveyTitle:            strPtr("Annual Survey"),
			SurveyStatus:           "scheduled",
			ResponseStatus:         strPtr("open"),
			SurveySendDate:         strPtr("2026-02-01T00:00:00Z"),
			SurveyCutoffDate:       strPtr("2026-03-01T00:00:00Z"),
			SurveyReminderRateDays: intPtr(7),
			EmailSubject:           strPtr("Subject"),
			EmailBody:              strPtr("<p>Body</p>"),
			EmailBodyText:          strPtr("Body"),
			CommitteeCategory:      strPtr("Board"),
			Committees: []*survey.SurveyCommittee{{
				CommitteeName: strPtr("TSC"), CommitteeUID: strPtr("v2-committee-c1"), ProjectUID: strPtr("v2-project-p1"),
				ProjectName: strPtr("Project One"), SurveyURL: strPtr("https://example.com/s"),
				TotalRecipients: intPtr(10), TotalResponses: intPtr(4), NpsValue: f64Ptr(42.5),
			}},
			CommitteeVotingEnabled:        boolPtr(true),
			SurveyURL:                     strPtr("https://example.com/survey"),
			SendImmediately:               boolPtr(false),
			TotalRecipients:               intPtr(10),
			TotalResponses:                intPtr(4),
			IsNpsSurvey:                   boolPtr(true),
			NpsValue:                      f64Ptr(42.5),
			NumPromoters:                  intPtr(5),
			NumPassives:                   intPtr(3),
			NumDetractors:                 intPtr(2),
			TotalBouncedEmails:            intPtr(1),
			NumAutomatedRemindersToSend:   intPtr(3),
			NumAutomatedRemindersSent:     intPtr(1),
			NextAutomatedReminderAt:       strPtr("2026-02-08T00:00:00Z"),
			LatestAutomatedReminderSentAt: strPtr("2026-02-01T00:00:00Z"),
		}
		assert.Equal(t, want, got)
	})
}

func TestScheduleSurvey_Errors(t *testing.T) {
	t.Run("committee mapping failure is returned and ITX is not called", func(t *testing.T) {
		client := &mocks.MockSurveyClient{} // any call panics
		svc := newServiceWith(client, prefixMapper("bad-committee", ""))

		_, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), &survey.ScheduleSurveyPayload{CommitteeUID: "bad-committee"})

		assert.ErrorIs(t, err, errMapper)
	})

	t.Run("mapping failure is returned with context and not logged", func(t *testing.T) {
		// The service returns errors; the HTTP boundary logs them once.
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		svc := service.NewSurveyService(&mocks.MockSurveyClient{}, &mocks.MockExclusionClient{}, &mocks.MockSurveyResponseClient{}, prefixMapper("bad-committee", ""), logger)

		_, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), &survey.ScheduleSurveyPayload{CommitteeUID: "bad-committee"})

		assert.ErrorIs(t, err, errMapper)
		assert.ErrorContains(t, err, `map committee_uid "bad-committee" to v1`)
		assert.NotContains(t, logs.String(), `"level":"ERROR"`)
		assert.NotContains(t, logs.String(), `"level":"WARN"`)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewUnavailableError("itx down")
		client := &mocks.MockSurveyClient{
			ScheduleSurveyFunc: func(context.Context, *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
				return nil, itxErr
			},
		}
		svc := newServiceWith(client, prefixMapper("", ""))

		_, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), &survey.ScheduleSurveyPayload{CommitteeUID: "c"})

		assert.Same(t, itxErr, err)
	})
}

func TestScheduleSurvey_ResponseCommitteeMapping(t *testing.T) {
	respond := func(committees []itx.SurveyCommittee) *mocks.MockSurveyClient {
		return &mocks.MockSurveyClient{
			ScheduleSurveyFunc: func(context.Context, *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
				return &itx.SurveyScheduleResponse{ID: "s", Committees: committees}, nil
			},
		}
	}
	schedule := func(client *mocks.MockSurveyClient, mapper *mocks.MockIDMapper) (*survey.SurveyScheduleResult, error) {
		return newServiceWith(client, mapper).ScheduleSurvey(ctxWithPrincipal("alice"), &survey.ScheduleSurveyPayload{CommitteeUID: "c"})
	}

	t.Run("no committees stays nil", func(t *testing.T) {
		got, err := schedule(respond(nil), prefixMapper("", ""))
		require.NoError(t, err)
		assert.Nil(t, got.Committees)
	})

	t.Run("v1 id is kept when its mapping fails", func(t *testing.T) {
		got, err := schedule(respond([]itx.SurveyCommittee{{CommitteeID: strPtr("c-bad"), ProjectID: strPtr("p-bad")}}), prefixMapper("", "c-bad"))
		require.NoError(t, err)
		require.Len(t, got.Committees, 1)
		assert.Equal(t, "c-bad", *got.Committees[0].CommitteeUID, "committee falls back to the v1 id")
		assert.Equal(t, "v2-project-p-bad", *got.Committees[0].ProjectUID, "project still maps")
	})

	t.Run("project v1 id is kept when its mapping fails", func(t *testing.T) {
		got, err := schedule(respond([]itx.SurveyCommittee{{CommitteeID: strPtr("c1"), ProjectID: strPtr("p-bad")}}), prefixMapper("", "p-bad"))
		require.NoError(t, err)
		assert.Equal(t, "v2-committee-c1", *got.Committees[0].CommitteeUID)
		assert.Equal(t, "p-bad", *got.Committees[0].ProjectUID)
	})

	t.Run("missing or empty ids leave the uids nil", func(t *testing.T) {
		got, err := schedule(respond([]itx.SurveyCommittee{{CommitteeName: strPtr("No ids")}, {CommitteeID: strPtr(""), ProjectID: strPtr("")}}), prefixMapper("", ""))
		require.NoError(t, err)
		require.Len(t, got.Committees, 2)
		for _, c := range got.Committees {
			assert.Nil(t, c.CommitteeUID)
			assert.Nil(t, c.ProjectUID)
		}
	})

	t.Run("order is preserved", func(t *testing.T) {
		got, err := schedule(respond([]itx.SurveyCommittee{{CommitteeID: strPtr("a")}, {CommitteeID: strPtr("b")}, {CommitteeID: strPtr("c")}}), prefixMapper("", ""))
		require.NoError(t, err)
		require.Len(t, got.Committees, 3)
		assert.Equal(t, "v2-committee-a", *got.Committees[0].CommitteeUID)
		assert.Equal(t, "v2-committee-b", *got.Committees[1].CommitteeUID)
		assert.Equal(t, "v2-committee-c", *got.Committees[2].CommitteeUID)
	})
}

func TestGetSurvey(t *testing.T) {
	var gotID string
	var gotParams *itx.GetSurveyParams
	client := &mocks.MockSurveyClient{
		GetSurveyFunc: func(_ context.Context, id string, params *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
			gotID, gotParams = id, params
			return &itx.SurveyScheduleResponse{ID: id, SurveyStatus: "sent"}, nil
		},
	}
	get := func(p *survey.GetSurveyPayload) (*survey.SurveyScheduleResult, error) {
		gotID, gotParams = "", nil
		return newServiceWith(client, prefixMapper("p-bad", "")).GetSurvey(context.Background(), p)
	}

	t.Run("without filters no query params are sent", func(t *testing.T) {
		got, err := get(&survey.GetSurveyPayload{SurveyUID: "s-1"})
		require.NoError(t, err)
		assert.Equal(t, "s-1", gotID)
		assert.Nil(t, gotParams)
		assert.Equal(t, "s-1", got.UID)
		assert.Equal(t, "sent", got.SurveyStatus)
	})

	t.Run("project_uid is mapped v2 to v1", func(t *testing.T) {
		_, err := get(&survey.GetSurveyPayload{SurveyUID: "s-1", ProjectUID: strPtr("p1")})
		require.NoError(t, err)
		require.NotNil(t, gotParams)
		assert.Equal(t, "v1-project-p1", *gotParams.ProjectID)
		assert.Nil(t, gotParams.ProjectIDs)
	})

	t.Run("project_uids is split, trimmed, mapped and rejoined in order", func(t *testing.T) {
		_, err := get(&survey.GetSurveyPayload{SurveyUID: "s-1", ProjectUids: strPtr(" p1, p2 ,,p3 ")})
		require.NoError(t, err)
		require.NotNil(t, gotParams)
		assert.Equal(t, "v1-project-p1,v1-project-p2,v1-project-p3", *gotParams.ProjectIDs)
		assert.Nil(t, gotParams.ProjectID)
	})

	t.Run("empty filter values add no filter", func(t *testing.T) {
		_, err := get(&survey.GetSurveyPayload{SurveyUID: "s-1", ProjectUID: strPtr(""), ProjectUids: strPtr("")})
		require.NoError(t, err)
		if gotParams != nil {
			assert.Nil(t, gotParams.ProjectID)
			assert.Nil(t, gotParams.ProjectIDs)
		}
	})

	t.Run("both project filters are rejected before ITX is called", func(t *testing.T) {
		gotID = ""
		_, err := get(&survey.GetSurveyPayload{SurveyUID: "s-1", ProjectUID: strPtr("p1"), ProjectUids: strPtr("p2")})
		requireErrorType(t, err, domain.ErrorTypeValidation)
		assert.Empty(t, gotID, "ITX must not be called")
	})

	t.Run("project_uid mapping failure is returned", func(t *testing.T) {
		_, err := get(&survey.GetSurveyPayload{SurveyUID: "s-1", ProjectUID: strPtr("p-bad")})
		assert.ErrorIs(t, err, errMapper)
	})

	t.Run("one failing project_uids entry fails the request", func(t *testing.T) {
		_, err := get(&survey.GetSurveyPayload{SurveyUID: "s-1", ProjectUids: strPtr("p1,p-bad")})
		assert.ErrorIs(t, err, errMapper)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewNotFoundError("no such survey")
		svc := newServiceWith(&mocks.MockSurveyClient{
			GetSurveyFunc: func(context.Context, string, *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
				return nil, itxErr
			},
		}, prefixMapper("", ""))

		_, err := svc.GetSurvey(context.Background(), &survey.GetSurveyPayload{SurveyUID: "s-1"})

		assert.Same(t, itxErr, err)
	})
}

func TestUpdateSurvey_MapsRequestAndResponse(t *testing.T) {
	var gotID string
	var sent *itx.UpdateSurveyRequest
	client := &mocks.MockSurveyClient{
		UpdateSurveyFunc: func(_ context.Context, id string, req *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
			gotID, sent = id, req
			return &itx.SurveyScheduleResponse{ID: id, SurveyStatus: "scheduled", Committees: []itx.SurveyCommittee{{CommitteeID: strPtr("c1")}}}, nil
		},
	}
	update := func(p *survey.UpdateSurveyPayload) (*survey.SurveyScheduleResult, error) {
		gotID, sent = "", nil
		return newServiceWith(client, prefixMapper("c-bad", "")).UpdateSurvey(ctxWithPrincipal("alice"), p)
	}

	t.Run("committee is mapped and every field forwarded", func(t *testing.T) {
		got, err := update(&survey.UpdateSurveyPayload{
			SurveyUID:              "s-1",
			CommitteeUID:           strPtr("c-uid"),
			SurveyTitle:            strPtr("New title"),
			SurveySendDate:         strPtr("2026-02-01T00:00:00Z"),
			SurveyCutoffDate:       strPtr("2026-03-01T00:00:00Z"),
			SurveyReminderRateDays: intPtr(5),
			EmailSubject:           strPtr("Subject"),
			EmailBody:              strPtr("<p>Body</p>"),
			EmailBodyText:          strPtr("Body"),
			CommitteeVotingEnabled: boolPtr(true),
		})
		require.NoError(t, err)
		require.NotNil(t, sent)
		assert.Equal(t, "s-1", gotID)
		assert.Equal(t, []string{"v1-committee-c-uid"}, sent.Committees)
		assert.Equal(t, strPtr("New title"), sent.SurveyTitle)
		assert.Equal(t, strPtr("2026-02-01T00:00:00Z"), sent.SurveySendDate)
		assert.Equal(t, strPtr("2026-03-01T00:00:00Z"), sent.SurveyCutoffDate)
		assert.Equal(t, intPtr(5), sent.SurveyReminderRateDays)
		assert.Equal(t, strPtr("Subject"), sent.EmailSubject)
		assert.Equal(t, strPtr("<p>Body</p>"), sent.EmailBody)
		assert.Equal(t, strPtr("Body"), sent.EmailBodyText)
		assert.Equal(t, boolPtr(true), sent.CommitteeVotingEnabled)

		assert.Equal(t, "s-1", got.UID)
		require.Len(t, got.Committees, 1)
		assert.Equal(t, "v2-committee-c1", *got.Committees[0].CommitteeUID, "response committee ids are mapped v1 to v2")
	})

	t.Run("omitted or empty committee sends no committees", func(t *testing.T) {
		for name, uid := range map[string]*string{"nil": nil, "empty": strPtr("")} {
			_, err := update(&survey.UpdateSurveyPayload{SurveyUID: "s-1", CommitteeUID: uid})
			require.NoError(t, err, name)
			assert.Nil(t, sent.Committees, name)
		}
	})

	t.Run("committee mapping failure is returned and ITX is not called", func(t *testing.T) {
		_, err := update(&survey.UpdateSurveyPayload{SurveyUID: "s-1", CommitteeUID: strPtr("c-bad")})
		assert.ErrorIs(t, err, errMapper)
		assert.Nil(t, sent)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewConflictError("survey is not editable")
		svc := newServiceWith(&mocks.MockSurveyClient{
			UpdateSurveyFunc: func(context.Context, string, *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
				return nil, itxErr
			},
		}, prefixMapper("", ""))

		_, err := svc.UpdateSurvey(ctxWithPrincipal("alice"), &survey.UpdateSurveyPayload{SurveyUID: "s-1"})

		assert.Same(t, itxErr, err)
	})
}

func TestDeleteSurvey(t *testing.T) {
	t.Run("forwards the survey id", func(t *testing.T) {
		var gotID string
		svc := newServiceWith(&mocks.MockSurveyClient{
			DeleteSurveyFunc: func(_ context.Context, id string) error { gotID = id; return nil },
		}, prefixMapper("", ""))

		require.NoError(t, svc.DeleteSurvey(context.Background(), &survey.DeleteSurveyPayload{SurveyUID: "s-1"}))
		assert.Equal(t, "s-1", gotID)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewNotFoundError("gone")
		svc := newServiceWith(&mocks.MockSurveyClient{
			DeleteSurveyFunc: func(context.Context, string) error { return itxErr },
		}, prefixMapper("", ""))

		assert.Same(t, itxErr, svc.DeleteSurvey(context.Background(), &survey.DeleteSurveyPayload{SurveyUID: "s-1"}))
	})
}

// decodeLogRecords parses the JSON log lines written to buf.
func decodeLogRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	dec := json.NewDecoder(buf)
	for dec.More() {
		var rec map[string]any
		require.NoError(t, dec.Decode(&rec))
		records = append(records, rec)
	}
	return records
}

// TestDeleteSurvey_Logging pins the logging volume of a routine call: one Info
// line when it succeeds, with the request line kept at Debug.
func TestDeleteSurvey_Logging(t *testing.T) {
	deleteWith := func(level slog.Level) []map[string]any {
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: level}))
		svc := service.NewSurveyService(&mocks.MockSurveyClient{
			DeleteSurveyFunc: func(context.Context, string) error { return nil },
		}, &mocks.MockExclusionClient{}, &mocks.MockSurveyResponseClient{}, prefixMapper("", ""), logger)

		require.NoError(t, svc.DeleteSurvey(ctxWithPrincipal("alice"), &survey.DeleteSurveyPayload{SurveyUID: "s-1"}))
		return decodeLogRecords(t, &logs)
	}

	t.Run("one Info line when it succeeds", func(t *testing.T) {
		records := deleteWith(slog.LevelInfo)
		require.Len(t, records, 1)
		assert.Equal(t, "survey deleted successfully", records[0]["msg"])
		assert.Equal(t, "INFO", records[0]["level"])
	})

	t.Run("the request line is Debug", func(t *testing.T) {
		records := deleteWith(slog.LevelDebug)
		require.Len(t, records, 2)
		assert.Equal(t, "deleting survey", records[0]["msg"])
		assert.Equal(t, "DEBUG", records[0]["level"])
	})
}

func TestBulkResendSurvey(t *testing.T) {
	t.Run("forwards the survey id and recipient ids", func(t *testing.T) {
		var gotID string
		var gotReq *itx.BulkResendRequest
		svc := newServiceWith(&mocks.MockSurveyClient{
			BulkResendSurveyFunc: func(_ context.Context, id string, req *itx.BulkResendRequest) error {
				gotID, gotReq = id, req
				return nil
			},
		}, prefixMapper("", ""))

		require.NoError(t, svc.BulkResendSurvey(context.Background(), &survey.BulkResendSurveyPayload{SurveyUID: "s-1", RecipientIds: []string{"r1", "r2"}}))
		assert.Equal(t, "s-1", gotID)
		require.NotNil(t, gotReq)
		assert.Equal(t, []string{"r1", "r2"}, gotReq.RecipientIDs)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewUnavailableError("itx down")
		svc := newServiceWith(&mocks.MockSurveyClient{
			BulkResendSurveyFunc: func(context.Context, string, *itx.BulkResendRequest) error { return itxErr },
		}, prefixMapper("", ""))

		assert.Same(t, itxErr, svc.BulkResendSurvey(context.Background(), &survey.BulkResendSurveyPayload{SurveyUID: "s-1"}))
	})
}

func TestPreviewSendSurvey(t *testing.T) {
	var gotID string
	var gotCommittee *string
	response := &itx.PreviewSendResponse{
		AffectedProjects: []itx.LFXProject{
			{ID: "p1", Name: "Project One", Slug: "project-one", Status: "Active", LogoURL: strPtr("https://example.com/logo.png")},
			{ID: "p-bad", Name: "Unmapped", Slug: "unmapped", Status: "Active"},
			{ID: "", Name: "No id"},
		},
		AffectedCommittees: []itx.ExcludedCommittee{
			{ProjectID: "p1", ProjectName: "Project One", CommitteeID: "c1", CommitteeName: "TSC", CommitteeCategory: "Technical"},
			{ProjectID: "p-bad", CommitteeID: "c-bad", CommitteeName: "Unmapped"},
		},
		AffectedRecipients: []itx.ITXPreviewRecipient{
			{UserID: "u1", Name: strPtr("Alice Example"), FirstName: strPtr("Alice"), LastName: strPtr("Example"), Username: strPtr("alice"), Email: "alice@example.com", Role: strPtr("Chair")},
			{UserID: "u2", Email: "bob@example.com"},
		},
	}
	client := &mocks.MockSurveyClient{
		PreviewSendFunc: func(_ context.Context, id string, committee *string) (*itx.PreviewSendResponse, error) {
			gotID, gotCommittee = id, committee
			return response, nil
		},
	}
	t.Run("maps projects, committees and recipients", func(t *testing.T) {
		svc := newServiceWith(client, prefixMapper("c-bad", "p-bad"))
		got, err := svc.PreviewSendSurvey(context.Background(), &survey.PreviewSendSurveyPayload{SurveyUID: "s-1", CommitteeUID: strPtr("c-uid")})
		require.NoError(t, err)

		assert.Equal(t, "s-1", gotID)
		require.NotNil(t, gotCommittee)
		assert.Equal(t, "v1-committee-c-uid", *gotCommittee, "committee filter is mapped v2 to v1")

		require.Len(t, got.AffectedProjects, 3)
		assert.Equal(t, &survey.LFXProject{ID: "v2-project-p1", Name: "Project One", Slug: "project-one", Status: "Active", LogoURL: strPtr("https://example.com/logo.png")}, got.AffectedProjects[0])
		assert.Equal(t, "p-bad", got.AffectedProjects[1].ID, "falls back to the v1 id when mapping fails")
		assert.Equal(t, "", got.AffectedProjects[2].ID, "an empty id is not mapped")

		require.Len(t, got.AffectedCommittees, 2)
		assert.Equal(t, &survey.ExcludedCommittee{ProjectUID: "v2-project-p1", ProjectName: "Project One", CommitteeUID: "v2-committee-c1", CommitteeName: "TSC", CommitteeCategory: "Technical"}, got.AffectedCommittees[0])
		assert.Equal(t, "p-bad", got.AffectedCommittees[1].ProjectUID)
		assert.Equal(t, "v2-committee-c-bad", got.AffectedCommittees[1].CommitteeUID)

		require.Len(t, got.AffectedRecipients, 2)
		assert.Equal(t, &survey.ITXPreviewRecipient{UserID: "u1", Name: strPtr("Alice Example"), FirstName: strPtr("Alice"), LastName: strPtr("Example"), Username: strPtr("alice"), Email: "alice@example.com", Role: strPtr("Chair")}, got.AffectedRecipients[0])
		assert.Equal(t, &survey.ITXPreviewRecipient{UserID: "u2", Email: "bob@example.com"}, got.AffectedRecipients[1])
	})

	t.Run("committee v1 id is kept when its mapping fails", func(t *testing.T) {
		svc := newServiceWith(&mocks.MockSurveyClient{
			PreviewSendFunc: func(context.Context, string, *string) (*itx.PreviewSendResponse, error) {
				return &itx.PreviewSendResponse{
					AffectedCommittees: []itx.ExcludedCommittee{{ProjectID: "p1", CommitteeID: "c-bad", CommitteeName: "Unmapped"}},
				}, nil
			},
		}, prefixMapper("", "c-bad"))

		got, err := svc.PreviewSendSurvey(context.Background(), &survey.PreviewSendSurveyPayload{SurveyUID: "s-1"})
		require.NoError(t, err)
		require.Len(t, got.AffectedCommittees, 1)
		assert.Equal(t, "c-bad", got.AffectedCommittees[0].CommitteeUID, "committee falls back to the v1 id")
		assert.Equal(t, "v2-project-p1", got.AffectedCommittees[0].ProjectUID, "project still maps")
	})

	t.Run("no committee filter sends none", func(t *testing.T) {
		svc := newServiceWith(client, prefixMapper("", ""))
		_, err := svc.PreviewSendSurvey(context.Background(), &survey.PreviewSendSurveyPayload{SurveyUID: "s-1"})
		require.NoError(t, err)
		assert.Nil(t, gotCommittee)
	})

	t.Run("an empty ITX response yields empty non-nil lists", func(t *testing.T) {
		svc := newServiceWith(&mocks.MockSurveyClient{
			PreviewSendFunc: func(context.Context, string, *string) (*itx.PreviewSendResponse, error) {
				return &itx.PreviewSendResponse{}, nil
			},
		}, prefixMapper("", ""))

		got, err := svc.PreviewSendSurvey(context.Background(), &survey.PreviewSendSurveyPayload{SurveyUID: "s-1"})
		require.NoError(t, err)
		// The JSON contract needs [] rather than null.
		assert.NotNil(t, got.AffectedProjects)
		assert.Empty(t, got.AffectedProjects)
		assert.NotNil(t, got.AffectedCommittees)
		assert.Empty(t, got.AffectedCommittees)
		assert.NotNil(t, got.AffectedRecipients)
		assert.Empty(t, got.AffectedRecipients)
	})

	t.Run("committee mapping failure is returned and ITX is not called", func(t *testing.T) {
		svc := newServiceWith(&mocks.MockSurveyClient{}, prefixMapper("c-bad", ""))
		_, err := svc.PreviewSendSurvey(context.Background(), &survey.PreviewSendSurveyPayload{SurveyUID: "s-1", CommitteeUID: strPtr("c-bad")})
		assert.ErrorIs(t, err, errMapper)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewUnavailableError("itx down")
		svc := newServiceWith(&mocks.MockSurveyClient{
			PreviewSendFunc: func(context.Context, string, *string) (*itx.PreviewSendResponse, error) { return nil, itxErr },
		}, prefixMapper("", ""))

		_, err := svc.PreviewSendSurvey(context.Background(), &survey.PreviewSendSurveyPayload{SurveyUID: "s-1"})
		assert.Same(t, itxErr, err)
	})
}

// TestSurveyService_ResponseMappingFailure verifies that a failure while mapping
// the ITX response back to the Goa result is returned and no partial result leaks.
// The mappers fan out through BatchMap, which fails fast on a cancelled context.
func TestSurveyService_ResponseMappingFailure(t *testing.T) {
	cancelledCtx := func() context.Context {
		ctx, cancel := context.WithCancel(ctxWithPrincipal("alice"))
		cancel()
		return ctx
	}
	scheduleResponse := &itx.SurveyScheduleResponse{
		ID:         "survey-1",
		Committees: []itx.SurveyCommittee{{CommitteeID: strPtr("c1")}},
	}
	client := &mocks.MockSurveyClient{
		ScheduleSurveyFunc: func(context.Context, *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
			return scheduleResponse, nil
		},
		GetSurveyFunc: func(context.Context, string, *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
			return scheduleResponse, nil
		},
		UpdateSurveyFunc: func(context.Context, string, *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
			return scheduleResponse, nil
		},
	}
	svc := newServiceWith(client, prefixMapper("", ""))

	t.Run("ScheduleSurvey", func(t *testing.T) {
		got, err := svc.ScheduleSurvey(cancelledCtx(), &survey.ScheduleSurveyPayload{CommitteeUID: "c"})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, got)
	})

	t.Run("GetSurvey", func(t *testing.T) {
		got, err := svc.GetSurvey(cancelledCtx(), &survey.GetSurveyPayload{SurveyUID: "survey-1"})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, got)
	})

	t.Run("UpdateSurvey", func(t *testing.T) {
		got, err := svc.UpdateSurvey(cancelledCtx(), &survey.UpdateSurveyPayload{SurveyUID: "survey-1"})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, got)
	})

	previewWith := func(resp *itx.PreviewSendResponse) *service.SurveyService {
		return newServiceWith(&mocks.MockSurveyClient{
			PreviewSendFunc: func(context.Context, string, *string) (*itx.PreviewSendResponse, error) { return resp, nil },
		}, prefixMapper("", ""))
	}

	t.Run("PreviewSendSurvey projects", func(t *testing.T) {
		svc := previewWith(&itx.PreviewSendResponse{AffectedProjects: []itx.LFXProject{{ID: "p1"}}})
		got, err := svc.PreviewSendSurvey(cancelledCtx(), &survey.PreviewSendSurveyPayload{SurveyUID: "survey-1"})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, got)
	})

	t.Run("PreviewSendSurvey committees", func(t *testing.T) {
		// No projects, so the project fan-out is skipped and the committee one fails.
		svc := previewWith(&itx.PreviewSendResponse{AffectedCommittees: []itx.ExcludedCommittee{{CommitteeID: "c1"}}})
		got, err := svc.PreviewSendSurvey(cancelledCtx(), &survey.PreviewSendSurveyPayload{SurveyUID: "survey-1"})
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, got)
	})
}

func TestSendMissingRecipients(t *testing.T) {
	var gotID string
	var gotCommittee *string
	client := &mocks.MockSurveyClient{
		SendMissingRecipientsFunc: func(_ context.Context, id string, committee *string) error {
			gotID, gotCommittee = id, committee
			return nil
		},
	}
	svc := newServiceWith(client, prefixMapper("c-bad", ""))

	t.Run("maps the committee filter v2 to v1", func(t *testing.T) {
		require.NoError(t, svc.SendMissingRecipients(context.Background(), &survey.SendMissingRecipientsPayload{SurveyUID: "s-1", CommitteeUID: strPtr("c-uid")}))
		assert.Equal(t, "s-1", gotID)
		require.NotNil(t, gotCommittee)
		assert.Equal(t, "v1-committee-c-uid", *gotCommittee)
	})

	t.Run("no committee filter sends none", func(t *testing.T) {
		gotCommittee = strPtr("stale")
		require.NoError(t, svc.SendMissingRecipients(context.Background(), &survey.SendMissingRecipientsPayload{SurveyUID: "s-1"}))
		assert.Nil(t, gotCommittee)
	})

	t.Run("committee mapping failure is returned and ITX is not called", func(t *testing.T) {
		gotID = ""
		err := svc.SendMissingRecipients(context.Background(), &survey.SendMissingRecipientsPayload{SurveyUID: "s-1", CommitteeUID: strPtr("c-bad")})
		assert.ErrorIs(t, err, errMapper)
		assert.Empty(t, gotID)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewUnavailableError("itx down")
		failing := newServiceWith(&mocks.MockSurveyClient{
			SendMissingRecipientsFunc: func(context.Context, string, *string) error { return itxErr },
		}, prefixMapper("", ""))

		assert.Same(t, itxErr, failing.SendMissingRecipients(context.Background(), &survey.SendMissingRecipientsPayload{SurveyUID: "s-1"}))
	})
}

func TestValidateEmail(t *testing.T) {
	t.Run("forwards body and subject and returns the validated result", func(t *testing.T) {
		var sent *itx.ValidateEmailRequest
		svc := newServiceWith(&mocks.MockSurveyClient{
			ValidateEmailFunc: func(_ context.Context, req *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error) {
				sent = req
				return &itx.ValidateEmailResponse{Body: "<p>Validated</p>", Subject: "Validated subject"}, nil
			},
		}, prefixMapper("", ""))

		got, err := svc.ValidateEmail(context.Background(), &survey.ValidateEmailPayload{Body: strPtr("<p>Hi</p>"), Subject: strPtr("Hi")})

		require.NoError(t, err)
		require.NotNil(t, sent)
		assert.Equal(t, strPtr("<p>Hi</p>"), sent.Body)
		assert.Equal(t, strPtr("Hi"), sent.Subject)
		assert.Equal(t, "<p>Validated</p>", got.Body)
		assert.Equal(t, "Validated subject", got.Subject)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewValidationError("bad template")
		svc := newServiceWith(&mocks.MockSurveyClient{
			ValidateEmailFunc: func(context.Context, *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error) {
				return nil, itxErr
			},
		}, prefixMapper("", ""))

		got, err := svc.ValidateEmail(context.Background(), &survey.ValidateEmailPayload{})

		assert.Nil(t, got)
		assert.Same(t, itxErr, err)
	})
}
