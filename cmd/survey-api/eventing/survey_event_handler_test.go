// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// surveyNumericFields are the survey-level counters DynamoDB delivers as strings.
var surveyNumericFields = []string{
	"survey_reminder_rate_days", "nps_value", "num_promoters", "num_passives", "num_detractors",
	"total_recipients", "total_recipients_sent", "total_responses", "total_recipients_opened",
	"total_recipients_clicked", "total_delivery_errors",
}

// committeeNumericFields are the per-committee counters.
var committeeNumericFields = []string{
	"nps_value", "num_promoters", "num_passives", "num_detractors", "total_recipients",
	"total_recipients_sent", "total_responses", "total_recipients_opened",
	"total_recipients_clicked", "total_delivery_errors",
}

// v1IDMapper maps v1 → v2 with distinct prefixes so project/committee mix-ups
// show up in assertions; ids listed in failIDs return an error.
func v1IDMapper(failIDs ...string) *mocks.MockIDMapper {
	fails := map[string]bool{}
	for _, id := range failIDs {
		fails[id] = true
	}
	mapper := func(prefix string) func(context.Context, string) (string, error) {
		return func(_ context.Context, id string) (string, error) {
			if fails[id] {
				return "", errors.New("mapper unavailable")
			}
			return prefix + id, nil
		}
	}
	return &mocks.MockIDMapper{
		MapCommitteeV1ToV2Func: mapper("v2-committee-"),
		MapProjectV1ToV2Func:   mapper("v2-project-"),
	}
}

// surveyV1Data is a v1 itx-surveys record with numeric fields as strings
// (the DynamoDB shape) and two committees.
func surveyV1Data() map[string]any {
	return map[string]any{
		"id":                        "survey-1",
		"survey_monkey_id":          "412349483",
		"is_project_survey":         true,
		"stage_filter":              "Active",
		"creator_username":          "alice",
		"creator_name":              "Alice Example",
		"creator_id":                "creator-1",
		"created_at":                "2030-01-01T00:00:00Z",
		"last_modified_at":          "2030-01-02T00:00:00Z",
		"last_modified_by":          "bob",
		"survey_title":              "Q4 NPS",
		"survey_send_date":          "2030-01-03T00:00:00Z",
		"survey_cutoff_date":        "2030-01-31T00:00:00Z",
		"survey_reminder_rate_days": "7",
		"send_immediately":          true,
		"email_subject":             "Subject",
		"email_body":                "<p>Body</p>",
		"email_body_text":           "Body",
		"committee_category":        "Board",
		"committee_voting_enabled":  true,
		"survey_status":             "sent",
		"nps_value":                 "42",
		"num_promoters":             "10",
		"num_passives":              "5",
		"num_detractors":            "2",
		"total_recipients":          "20",
		"total_recipients_sent":     "19",
		"total_responses":           "17",
		"total_recipients_opened":   "15",
		"total_recipients_clicked":  "12",
		"total_delivery_errors":     "1",
		"is_nps_survey":             true,
		"collector_url":             "https://example.com/collect",
		"committees": []any{
			map[string]any{
				"committee_id": "c1", "committee_name": "Board", "project_id": "p1", "project_name": "Project One",
				"nps_value": "30", "num_promoters": "6", "num_passives": "3", "num_detractors": "1",
				"total_recipients": "10", "total_recipients_sent": "9", "total_responses": "8",
				"total_recipients_opened": "7", "total_recipients_clicked": "6", "total_delivery_errors": "1",
			},
			map[string]any{"committee_id": "c2", "committee_name": "TSC", "project_id": "p2", "project_name": "Project Two"},
		},
	}
}

// ─── SurveyDBRaw.UnmarshalJSON ───────────────────────────────────────────────

func TestSurveyDBRaw_UnmarshalJSON(t *testing.T) {
	t.Run("maps every field and converts numeric strings", func(t *testing.T) {
		raw, err := json.Marshal(surveyV1Data())
		require.NoError(t, err)

		var got SurveyDBRaw
		require.NoError(t, json.Unmarshal(raw, &got))

		assert.Equal(t, SurveyDBRaw{
			ID: "survey-1", SurveyMonkeyID: "412349483", IsProjectSurvey: true, StageFilter: "Active",
			CreatorUsername: "alice", CreatorName: "Alice Example", CreatorID: "creator-1",
			CreatedAt: "2030-01-01T00:00:00Z", LastModifiedAt: "2030-01-02T00:00:00Z", LastModifiedBy: "bob",
			SurveyTitle: "Q4 NPS", SurveySendDate: "2030-01-03T00:00:00Z", SurveyCutoffDate: "2030-01-31T00:00:00Z",
			SurveyReminderRateDays: 7, SendImmediately: true, EmailSubject: "Subject", EmailBody: "<p>Body</p>",
			EmailBodyText: "Body", CommitteeCategory: "Board", CommitteeVotingEnabled: true, SurveyStatus: "sent",
			NPSValue: 42, NumPromoters: 10, NumPassives: 5, NumDetractors: 2, TotalRecipients: 20,
			TotalSentRecipients: 19, TotalResponses: 17, TotalRecipientsOpened: 15, TotalRecipientsClicked: 12,
			TotalDeliveryErrors: 1, IsNPSSurvey: true, CollectorURL: "https://example.com/collect",
			Committees: []SurveyCommitteeDBRaw{
				{CommitteeID: "c1", CommitteeName: "Board", ProjectID: "p1", ProjectName: "Project One",
					NPSValue: 30, NumPromoters: 6, NumPassives: 3, NumDetractors: 1, TotalRecipients: 10,
					TotalSentRecipients: 9, TotalResponses: 8, TotalRecipientsOpened: 7, TotalRecipientsClicked: 6,
					TotalDeliveryErrors: 1},
				{CommitteeID: "c2", CommitteeName: "TSC", ProjectID: "p2", ProjectName: "Project Two"},
			},
		}, got)
	})

	t.Run("accepts JSON numbers, empty strings and null for counters", func(t *testing.T) {
		var got SurveyDBRaw
		require.NoError(t, json.Unmarshal([]byte(
			`{"nps_value":42,"num_promoters":"","num_passives":null,"total_responses":7.9}`), &got))

		assert.Equal(t, 42, got.NPSValue)
		assert.Equal(t, 0, got.NumPromoters, "empty string is zero")
		assert.Equal(t, 0, got.NumPassives, "null is zero")
		assert.Equal(t, 0, got.NumDetractors, "missing is zero")
		assert.Equal(t, 7, got.TotalResponses, "float is truncated")
	})

	for _, field := range surveyNumericFields {
		t.Run("rejects non-numeric "+field, func(t *testing.T) {
			var got SurveyDBRaw
			err := json.Unmarshal([]byte(fmt.Sprintf(`{%q:"abc"}`, field)), &got)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to convert "+field)
		})
	}

	t.Run("rejects an unsupported counter type", func(t *testing.T) {
		var got SurveyDBRaw
		err := json.Unmarshal([]byte(`{"nps_value":true}`), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type for numeric field")
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		var got SurveyDBRaw
		require.Error(t, got.UnmarshalJSON([]byte(`{`)))
	})

	t.Run("propagates a bad committee", func(t *testing.T) {
		var got SurveyDBRaw
		err := json.Unmarshal([]byte(`{"committees":[{"nps_value":"abc"}]}`), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to convert nps_value")
	})
}

// ─── SurveyCommitteeDBRaw.UnmarshalJSON ──────────────────────────────────────

func TestSurveyCommitteeDBRaw_UnmarshalJSON(t *testing.T) {
	t.Run("maps strings and converts every counter", func(t *testing.T) {
		var got SurveyCommitteeDBRaw
		require.NoError(t, json.Unmarshal([]byte(`{
			"committee_id":"c1","committee_name":"Board","project_id":"p1","project_name":"Project One",
			"nps_value":"1","num_promoters":2,"num_passives":"3","num_detractors":4,"total_recipients":"5",
			"total_recipients_sent":6,"total_responses":"7","total_recipients_opened":8,
			"total_recipients_clicked":"9","total_delivery_errors":10}`), &got))

		assert.Equal(t, SurveyCommitteeDBRaw{
			CommitteeID: "c1", CommitteeName: "Board", ProjectID: "p1", ProjectName: "Project One",
			NPSValue: 1, NumPromoters: 2, NumPassives: 3, NumDetractors: 4, TotalRecipients: 5,
			TotalSentRecipients: 6, TotalResponses: 7, TotalRecipientsOpened: 8, TotalRecipientsClicked: 9,
			TotalDeliveryErrors: 10,
		}, got)
	})

	t.Run("empty string and null are zero", func(t *testing.T) {
		var got SurveyCommitteeDBRaw
		require.NoError(t, json.Unmarshal([]byte(`{"nps_value":"","num_promoters":null}`), &got))
		assert.Zero(t, got.NPSValue)
		assert.Zero(t, got.NumPromoters)
	})

	for _, field := range committeeNumericFields {
		t.Run("rejects non-numeric "+field, func(t *testing.T) {
			var got SurveyCommitteeDBRaw
			err := json.Unmarshal([]byte(fmt.Sprintf(`{%q:"abc"}`, field)), &got)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to convert "+field)
		})
	}

	t.Run("rejects an unsupported counter type", func(t *testing.T) {
		var got SurveyCommitteeDBRaw
		err := json.Unmarshal([]byte(`{"nps_value":[1]}`), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type for numeric field")
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		var got SurveyCommitteeDBRaw
		require.Error(t, got.UnmarshalJSON([]byte(`{`)))
	})
}

// ─── convertMapToSurveyData ──────────────────────────────────────────────────

func TestConvertMapToSurveyData(t *testing.T) {
	ctx := context.Background()

	t.Run("maps every field and resolves committee and project UIDs", func(t *testing.T) {
		got, err := convertMapToSurveyData(ctx, surveyV1Data(), v1IDMapper(), discardLogger())
		require.NoError(t, err)

		assert.Equal(t, &domain.SurveyData{
			UID: "survey-1", ID: "survey-1", SurveyMonkeyID: "412349483", IsProjectSurvey: true, StageFilter: "Active",
			CreatorUsername: "alice", CreatorName: "Alice Example", CreatorID: "creator-1",
			CreatedAt: "2030-01-01T00:00:00Z", LastModifiedAt: "2030-01-02T00:00:00Z", LastModifiedBy: "bob",
			SurveyTitle: "Q4 NPS", SurveySendDate: "2030-01-03T00:00:00Z", SurveyCutoffDate: "2030-01-31T00:00:00Z",
			SurveyReminderRateDays: 7, SendImmediately: true, EmailSubject: "Subject", EmailBody: "<p>Body</p>",
			EmailBodyText: "Body", CommitteeCategory: "Board", CommitteeVotingEnabled: true, SurveyStatus: "sent",
			NPSValue: 42, NumPromoters: 10, NumPassives: 5, NumDetractors: 2, TotalRecipients: 20,
			TotalSentRecipients: 19, TotalResponses: 17, TotalRecipientsOpened: 15, TotalRecipientsClicked: 12,
			TotalDeliveryErrors: 1, IsNPSSurvey: true, CollectorURL: "https://example.com/collect",
			Committees: []domain.SurveyCommitteeData{
				{CommitteeUID: "v2-committee-c1", CommitteeID: "c1", CommitteeName: "Board",
					ProjectUID: "v2-project-p1", ProjectID: "p1", ProjectName: "Project One",
					NPSValue: 30, NumPromoters: 6, NumPassives: 3, NumDetractors: 1, TotalRecipients: 10,
					TotalSentRecipients: 9, TotalResponses: 8, TotalRecipientsOpened: 7, TotalRecipientsClicked: 6,
					TotalDeliveryErrors: 1},
				{CommitteeUID: "v2-committee-c2", CommitteeID: "c2", CommitteeName: "TSC",
					ProjectUID: "v2-project-p2", ProjectID: "p2", ProjectName: "Project Two"},
			},
		}, got)
	})

	t.Run("a failed committee mapping leaves only the committee UID empty", func(t *testing.T) {
		got, err := convertMapToSurveyData(ctx, surveyV1Data(), v1IDMapper("c1"), discardLogger())
		require.NoError(t, err)

		require.Len(t, got.Committees, 2)
		assert.Empty(t, got.Committees[0].CommitteeUID)
		assert.Equal(t, "c1", got.Committees[0].CommitteeID, "v1 id is kept")
		assert.Equal(t, "v2-project-p1", got.Committees[0].ProjectUID)
		assert.Equal(t, "v2-committee-c2", got.Committees[1].CommitteeUID)
	})

	t.Run("a failed project mapping leaves only the project UID empty", func(t *testing.T) {
		got, err := convertMapToSurveyData(ctx, surveyV1Data(), v1IDMapper("p1"), discardLogger())
		require.NoError(t, err)

		require.Len(t, got.Committees, 2)
		assert.Empty(t, got.Committees[0].ProjectUID)
		assert.Equal(t, "p1", got.Committees[0].ProjectID, "v1 id is kept")
		assert.Equal(t, "v2-committee-c1", got.Committees[0].CommitteeUID)
		assert.Equal(t, "v2-project-p2", got.Committees[1].ProjectUID)
	})

	t.Run("empty v1 ids are not sent to the mapper", func(t *testing.T) {
		data := surveyV1Data()
		data["committees"] = []any{map[string]any{"committee_name": "No ids"}}

		// An unconfigured MockIDMapper panics if the mapper is called.
		got, err := convertMapToSurveyData(ctx, data, &mocks.MockIDMapper{}, discardLogger())
		require.NoError(t, err)

		require.Len(t, got.Committees, 1)
		assert.Empty(t, got.Committees[0].CommitteeUID)
		assert.Empty(t, got.Committees[0].ProjectUID)
	})

	t.Run("a survey without committees yields none", func(t *testing.T) {
		data := surveyV1Data()
		delete(data, "committees")

		got, err := convertMapToSurveyData(ctx, data, &mocks.MockIDMapper{}, discardLogger())
		require.NoError(t, err)
		assert.Empty(t, got.Committees)
	})

	t.Run("an invalid numeric field is an error", func(t *testing.T) {
		data := surveyV1Data()
		data["total_responses"] = "many"

		_, err := convertMapToSurveyData(ctx, data, v1IDMapper(), discardLogger())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal JSON into SurveyDBRaw")
	})

	t.Run("an unmarshalable value is an error", func(t *testing.T) {
		_, err := convertMapToSurveyData(ctx, map[string]any{"id": func() {}}, v1IDMapper(), discardLogger())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal v1Data")
	})
}

// ─── handleSurveyUpdate ──────────────────────────────────────────────────────

func TestHandleSurveyUpdate(t *testing.T) {
	ctx := context.Background()
	const key = "itx-surveys.survey-1"

	t.Run("publishes created when no mapping exists, then stores the mapping", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)

		retry := handleSurveyUpdate(ctx, key, surveyV1Data(), pub, v1IDMapper(), kv, discardLogger())

		assert.False(t, retry)
		require.Len(t, *events, 1)
		assert.Equal(t, "created", (*events)[0].Action)
		assert.Equal(t, "survey-1", (*events)[0].Survey.UID)
		assert.Equal(t, "v2-committee-c1", (*events)[0].Survey.Committees[0].CommitteeUID)
		assert.Equal(t, "1", kv.value("survey.survey-1"))
	})

	t.Run("publishes updated when a mapping already exists", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey.survey-1": "1"})

		retry := handleSurveyUpdate(ctx, key, surveyV1Data(), pub, v1IDMapper(), kv, discardLogger())

		assert.False(t, retry)
		require.Len(t, *events, 1)
		assert.Equal(t, "updated", (*events)[0].Action)
	})

	t.Run("a tombstoned survey that is written again is published as updated", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey.survey-1": tombstoneMarker})

		handleSurveyUpdate(ctx, key, surveyV1Data(), pub, v1IDMapper(), kv, discardLogger())

		require.Len(t, *events, 1)
		assert.Equal(t, "updated", (*events)[0].Action)
		assert.Equal(t, "1", kv.value("survey.survey-1"), "tombstone is replaced by the live marker")
	})

	t.Run("a conversion failure is acknowledged without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		data := surveyV1Data()
		data["nps_value"] = "bad"

		assert.False(t, handleSurveyUpdate(ctx, key, data, pub, v1IDMapper(), newFakeKV(nil), discardLogger()))
		assert.Empty(t, *events)
	})

	t.Run("a missing uid is acknowledged without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		data := surveyV1Data()
		delete(data, "id")

		assert.False(t, handleSurveyUpdate(ctx, key, data, pub, v1IDMapper(), newFakeKV(nil), discardLogger()))
		assert.Empty(t, *events)
	})

	t.Run("a survey with no resolvable parent is skipped", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)

		// Both committee and project mappings fail, so no committee has a UID.
		retry := handleSurveyUpdate(ctx, key, surveyV1Data(), pub, v1IDMapper("c1", "p1", "c2", "p2"), kv, discardLogger())

		assert.False(t, retry)
		assert.Empty(t, *events)
		assert.Empty(t, kv.data, "no mapping is stored for a skipped survey")
	})

	t.Run("a survey with only a project UID is still published", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)

		handleSurveyUpdate(ctx, key, surveyV1Data(), pub, v1IDMapper("c1", "c2"), newFakeKV(nil), discardLogger())

		require.Len(t, *events, 1, "project UID alone is a valid parent")
	})

	t.Run("a survey with no committees is skipped", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		data := surveyV1Data()
		delete(data, "committees")

		assert.False(t, handleSurveyUpdate(ctx, key, data, pub, &mocks.MockIDMapper{}, newFakeKV(nil), discardLogger()))
		assert.Empty(t, *events)
	})

	t.Run("a transient publish failure is retried and stores no mapping", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nats.ErrTimeout)
		kv := newFakeKV(nil)

		assert.True(t, handleSurveyUpdate(ctx, key, surveyV1Data(), pub, v1IDMapper(), kv, discardLogger()))
		assert.Empty(t, kv.data)
	})

	t.Run("a permanent publish failure is acknowledged and stores no mapping", func(t *testing.T) {
		pub, _ := newRecordingPublisher(errors.New("invalid payload"))
		kv := newFakeKV(nil)

		assert.False(t, handleSurveyUpdate(ctx, key, surveyV1Data(), pub, v1IDMapper(), kv, discardLogger()))
		assert.Empty(t, kv.data)
	})

	t.Run("a mapping store failure after publish is still acknowledged", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)
		kv.putErr = errors.New("kv down")

		assert.False(t, handleSurveyUpdate(ctx, key, surveyV1Data(), pub, v1IDMapper(), kv, discardLogger()))
		assert.Len(t, *events, 1)
	})
}

// ─── handleSurveyDelete ──────────────────────────────────────────────────────

func TestHandleSurveyDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("publishes deleted with the uid and tombstones the mapping", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey.survey-1": "1"})

		assert.False(t, handleSurveyDelete(ctx, "survey-1", pub, kv, discardLogger()))

		require.Len(t, *events, 1)
		assert.Equal(t, "deleted", (*events)[0].Action)
		assert.Equal(t, &domain.SurveyData{UID: "survey-1", ID: "survey-1"}, (*events)[0].Survey)
		assert.Equal(t, tombstoneMarker, kv.value("survey.survey-1"))
	})

	t.Run("an already tombstoned survey is not published again", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey.survey-1": tombstoneMarker})

		assert.False(t, handleSurveyDelete(ctx, "survey-1", pub, kv, discardLogger()))
		assert.Empty(t, *events)
	})

	t.Run("a live (non-tombstone) mapping does not suppress the delete", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey.survey-1": "1"})

		handleSurveyDelete(ctx, "survey-1", pub, kv, discardLogger())
		assert.Len(t, *events, 1)
	})

	t.Run("a never-seen survey is still published as deleted", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)

		handleSurveyDelete(ctx, "survey-9", pub, kv, discardLogger())
		assert.Len(t, *events, 1)
		assert.Equal(t, tombstoneMarker, kv.value("survey.survey-9"))
	})

	t.Run("a transient publish failure is retried and not tombstoned", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nats.ErrNoResponders)
		kv := newFakeKV(map[string]string{"survey.survey-1": "1"})

		assert.True(t, handleSurveyDelete(ctx, "survey-1", pub, kv, discardLogger()))
		assert.Equal(t, "1", kv.value("survey.survey-1"))
	})

	t.Run("a permanent publish failure is acknowledged and not tombstoned", func(t *testing.T) {
		pub, _ := newRecordingPublisher(errors.New("invalid payload"))
		kv := newFakeKV(map[string]string{"survey.survey-1": "1"})

		assert.False(t, handleSurveyDelete(ctx, "survey-1", pub, kv, discardLogger()))
		assert.Equal(t, "1", kv.value("survey.survey-1"))
	})

	t.Run("a tombstone store failure is still acknowledged", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)
		kv.putErr = errors.New("kv down")

		assert.False(t, handleSurveyDelete(ctx, "survey-1", pub, kv, discardLogger()))
		assert.Len(t, *events, 1)
	})
}

// ─── isTransientError ────────────────────────────────────────────────────────

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"nats no responders", nats.ErrNoResponders, true},
		{"nats timeout", nats.ErrTimeout, true},
		{"nats connection closed", nats.ErrConnectionClosed, true},
		{"nats connection draining", nats.ErrConnectionDraining, true},
		{"wrapped nats sentinel", fmt.Errorf("publish: %w", nats.ErrNoResponders), true},
		{"message mentions timeout", errors.New("request timeout"), true},
		{"message mentions connection", errors.New("lost connection to server"), true},
		{"message mentions unavailable", errors.New("service unavailable"), true},
		{"message mentions deadline", errors.New("context deadline exceeded"), true},
		{"unrelated nats error", nats.ErrBadSubscription, false},
		{"permanent error", errors.New("invalid payload"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTransientError(tt.err))
		})
	}
}
