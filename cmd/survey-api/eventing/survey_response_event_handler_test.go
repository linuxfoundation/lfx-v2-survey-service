// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

// responseV1Data is a v1 itx-survey-responses record with numeric fields as
// strings (the DynamoDB shape). Every string and counter is distinct so a
// swapped pair of fields cannot pass; booleans are checked one by one in the
// tests because they cannot all be distinct.
func responseV1Data() map[string]any {
	return map[string]any{
		"id":                               "resp-1",
		"survey_id":                        "survey-1",
		"survey_monkey_respondent_id":      "respondent-1",
		"email":                            "alice@example.com",
		"committee_member_id":              "member-1",
		"first_name":                       "Alice",
		"last_name":                        "Example",
		"created_at":                       "2030-02-01T00:00:00Z",
		"response_datetime":                "2030-02-02T00:00:00Z",
		"last_received_time":               "2030-02-03T00:00:00Z",
		"num_automated_reminders_received": "3",
		"username":                         "alice",
		"voting_status":                    "Voting Rep",
		"role":                             "Primary",
		"job_title":                        "Engineer",
		"membership_tier":                  "Gold",
		"organization":                     map[string]any{"id": "org-1", "name": "Example Org"},
		"project":                          map[string]any{"id": "p1", "name": "Project One"},
		"committee_id":                     "c1",
		"committee_voting_enabled":         false,
		"survey_link":                      "https://example.com/s/1",
		"nps_value":                        "9",
		"survey_monkey_question_answers": []any{
			map[string]any{
				"question_id": "q1", "question_text": "How likely?", "question_family": "matrix",
				"question_subtype": "rating",
				"answers":          []any{map[string]any{"choice_id": "ch1", "text": "Very"}},
			},
		},
		"ses_message_id":             "ses-1",
		"ses_bounce_type":            "Permanent",
		"ses_bounce_subtype":         "General",
		"ses_bounce_diagnostic_code": "550",
		"ses_complaint_exists":       false,
		"ses_complaint_type":         "abuse",
		"ses_complaint_date":         "2030-02-04T00:00:00Z",
		"ses_delivery_successful":    false,
		"email_opened_first_time":    "2030-02-05T00:00:00Z",
		"email_opened_last_time":     "2030-02-06T00:00:00Z",
		"link_clicked_first_time":    "2030-02-07T00:00:00Z",
		"link_clicked_last_time":     "2030-02-08T00:00:00Z",
		"excluded":                   false,
	}
}

// parentSurveyKV returns the v1-objects bucket holding the parent survey.
func parentSurveyKV(t *testing.T) *fakeKV {
	t.Helper()
	return newFakeKV(map[string]string{"itx-surveys.survey-1": string(mustJSON(t, surveyV1Data()))})
}

// parentMappingKV returns a mappings bucket where the parent survey is known.
func parentMappingKV() *fakeKV {
	return newFakeKV(map[string]string{"survey.survey-1": "1"})
}

// ─── SurveyResponseDBRaw.UnmarshalJSON ───────────────────────────────────────

func TestSurveyResponseDBRaw_UnmarshalJSON(t *testing.T) {
	t.Run("maps every field and converts numeric strings", func(t *testing.T) {
		var got SurveyResponseDBRaw
		require.NoError(t, json.Unmarshal(mustJSON(t, responseV1Data()), &got))

		assert.Equal(t, SurveyResponseDBRaw{
			ID: "resp-1", SurveyID: "survey-1", SurveyMonkeyRespondent: "respondent-1",
			Email: "alice@example.com", CommitteeMemberID: "member-1", FirstName: "Alice", LastName: "Example",
			CreatedAt: "2030-02-01T00:00:00Z", ResponseDatetime: "2030-02-02T00:00:00Z",
			LastReceivedTime: "2030-02-03T00:00:00Z", NumAutomatedRemindersReceived: 3, Username: "alice",
			VotingStatus: "Voting Rep", Role: "Primary", JobTitle: "Engineer", MembershipTier: "Gold",
			Organization: domain.SurveyResponseOrgData{ID: "org-1", Name: "Example Org"},
			Project:      SurveyResponseProjectDBRaw{ID: "p1", Name: "Project One"},
			CommitteeID:  "c1", SurveyLink: "https://example.com/s/1", NPSValue: 9,
			SurveyMonkeyQuestionAnswers: []domain.SurveyMonkeyQuestionAnswers{{
				QuestionID: "q1", QuestionText: "How likely?", QuestionFamily: "matrix", QuestionSubtype: "rating",
				Answers: []domain.SurveyMonkeyAnswer{{ChoiceID: "ch1", Text: "Very"}},
			}},
			SESMessageID: "ses-1", SESBounceType: "Permanent", SESBounceSubtype: "General",
			SESBounceDiagnosticCode: "550", SESComplaintType: "abuse", SESComplaintDate: "2030-02-04T00:00:00Z",
			EmailOpenedFirstTime: "2030-02-05T00:00:00Z", EmailOpenedLastTime: "2030-02-06T00:00:00Z",
			LinkClickedFirstTime: "2030-02-07T00:00:00Z", LinkClickedLastTime: "2030-02-08T00:00:00Z",
		}, got)
	})

	t.Run("each boolean maps to its own field", func(t *testing.T) {
		bools := map[string]func(SurveyResponseDBRaw) bool{
			"committee_voting_enabled": func(r SurveyResponseDBRaw) bool { return r.CommitteeVotingEnabled },
			"ses_complaint_exists":     func(r SurveyResponseDBRaw) bool { return r.SESComplaintExists },
			"ses_delivery_successful":  func(r SurveyResponseDBRaw) bool { return r.SESDeliverySuccessful },
			"excluded":                 func(r SurveyResponseDBRaw) bool { return r.Excluded },
		}
		for key := range bools {
			var got SurveyResponseDBRaw
			require.NoError(t, json.Unmarshal([]byte(`{"`+key+`":true}`), &got))
			for other, field := range bools {
				assert.Equal(t, other == key, field(got), "setting %s must affect only that field (checked %s)", key, other)
			}
		}
	})

	t.Run("accepts JSON numbers, empty strings and null for counters", func(t *testing.T) {
		var got SurveyResponseDBRaw
		require.NoError(t, json.Unmarshal([]byte(`{"num_automated_reminders_received":4.9,"nps_value":""}`), &got))
		assert.Equal(t, 4, got.NumAutomatedRemindersReceived)
		assert.Equal(t, 0, got.NPSValue)

		got = SurveyResponseDBRaw{}
		require.NoError(t, json.Unmarshal([]byte(`{"num_automated_reminders_received":null,"nps_value":10}`), &got))
		assert.Equal(t, 0, got.NumAutomatedRemindersReceived)
		assert.Equal(t, 10, got.NPSValue)
	})

	t.Run("a non-numeric counter names the offending field", func(t *testing.T) {
		var got SurveyResponseDBRaw
		err := json.Unmarshal([]byte(`{"num_automated_reminders_received":"many"}`), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "num_automated_reminders_received")

		err = json.Unmarshal([]byte(`{"nps_value":"high"}`), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nps_value")
	})

	t.Run("a counter of the wrong JSON type is rejected", func(t *testing.T) {
		var got SurveyResponseDBRaw
		err := json.Unmarshal([]byte(`{"nps_value":true}`), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type")
	})

	t.Run("malformed JSON is rejected", func(t *testing.T) {
		var got SurveyResponseDBRaw
		require.Error(t, got.UnmarshalJSON([]byte(`{`)))
	})
}

// ─── convertMapToSurveyResponseData ──────────────────────────────────────────

func TestConvertMapToSurveyResponseData(t *testing.T) {
	ctx := context.Background()

	t.Run("maps every field and resolves project and committee UIDs", func(t *testing.T) {
		got, err := convertMapToSurveyResponseData(ctx, responseV1Data(), v1IDMapper(), discardLogger())
		require.NoError(t, err)

		assert.Equal(t, &domain.SurveyResponseData{
			UID: "resp-1", ID: "resp-1", SurveyID: "survey-1", SurveyUID: "survey-1",
			SurveyMonkeyRespondent: "respondent-1", Email: "alice@example.com", CommitteeMemberID: "member-1",
			FirstName: "Alice", LastName: "Example", CreatedAt: "2030-02-01T00:00:00Z",
			ResponseDatetime: "2030-02-02T00:00:00Z", LastReceivedTime: "2030-02-03T00:00:00Z",
			NumAutomatedRemindersReceived: 3, Username: "alice", VotingStatus: "Voting Rep", Role: "Primary",
			JobTitle: "Engineer", MembershipTier: "Gold",
			Organization: domain.SurveyResponseOrgData{ID: "org-1", Name: "Example Org"},
			Project:      domain.SurveyResponseProjectData{ProjectUID: "v2-project-p1", ID: "p1", Name: "Project One"},
			CommitteeUID: "v2-committee-c1", CommitteeID: "c1", SurveyLink: "https://example.com/s/1", NPSValue: 9,
			SurveyMonkeyQuestionAnswers: []domain.SurveyMonkeyQuestionAnswers{{
				QuestionID: "q1", QuestionText: "How likely?", QuestionFamily: "matrix", QuestionSubtype: "rating",
				Answers: []domain.SurveyMonkeyAnswer{{ChoiceID: "ch1", Text: "Very"}},
			}},
			SESMessageID: "ses-1", SESBounceType: "Permanent", SESBounceSubtype: "General",
			SESBounceDiagnosticCode: "550", SESComplaintType: "abuse", SESComplaintDate: "2030-02-04T00:00:00Z",
			EmailOpenedFirstTime: "2030-02-05T00:00:00Z", EmailOpenedLastTime: "2030-02-06T00:00:00Z",
			LinkClickedFirstTime: "2030-02-07T00:00:00Z", LinkClickedLastTime: "2030-02-08T00:00:00Z",
		}, got)
	})

	t.Run("each boolean maps to its own field", func(t *testing.T) {
		bools := map[string]func(*domain.SurveyResponseData) bool{
			"committee_voting_enabled": func(r *domain.SurveyResponseData) bool { return r.CommitteeVotingEnabled },
			"ses_complaint_exists":     func(r *domain.SurveyResponseData) bool { return r.SESComplaintExists },
			"ses_delivery_successful":  func(r *domain.SurveyResponseData) bool { return r.SESDeliverySuccessful },
			"excluded":                 func(r *domain.SurveyResponseData) bool { return r.Excluded },
		}
		for key := range bools {
			data := responseV1Data()
			data[key] = true
			got, err := convertMapToSurveyResponseData(ctx, data, v1IDMapper(), discardLogger())
			require.NoError(t, err)
			for other, field := range bools {
				assert.Equal(t, other == key, field(got), "setting %s must affect only that field (checked %s)", key, other)
			}
		}
	})

	t.Run("a project mapping failure leaves the project UID empty", func(t *testing.T) {
		got, err := convertMapToSurveyResponseData(ctx, responseV1Data(), v1IDMapper("p1"), discardLogger())
		require.NoError(t, err)
		assert.Empty(t, got.Project.ProjectUID)
		assert.Equal(t, "p1", got.Project.ID, "the v1 id is kept")
		assert.Equal(t, "Project One", got.Project.Name)
		assert.Equal(t, "v2-committee-c1", got.CommitteeUID, "committee mapping is independent")
	})

	t.Run("a committee mapping failure leaves the committee UID empty", func(t *testing.T) {
		got, err := convertMapToSurveyResponseData(ctx, responseV1Data(), v1IDMapper("c1"), discardLogger())
		require.NoError(t, err)
		assert.Empty(t, got.CommitteeUID)
		assert.Equal(t, "c1", got.CommitteeID)
		assert.Equal(t, "v2-project-p1", got.Project.ProjectUID, "project mapping is independent")
	})

	t.Run("absent project and committee ids are not sent to the mapper", func(t *testing.T) {
		data := responseV1Data()
		delete(data, "project")
		delete(data, "committee_id")

		// An unconfigured mock panics if called.
		got, err := convertMapToSurveyResponseData(ctx, data, &mocks.MockIDMapper{}, discardLogger())
		require.NoError(t, err)
		assert.Empty(t, got.Project.ProjectUID)
		assert.Empty(t, got.CommitteeUID)
	})

	t.Run("a non-numeric counter is an error", func(t *testing.T) {
		data := responseV1Data()
		data["nps_value"] = "bad"

		_, err := convertMapToSurveyResponseData(ctx, data, v1IDMapper(), discardLogger())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal JSON into SurveyResponseDBRaw")
	})

	t.Run("an unmarshalable value is an error", func(t *testing.T) {
		_, err := convertMapToSurveyResponseData(ctx, map[string]any{"id": func() {}}, v1IDMapper(), discardLogger())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal v1Data")
	})
}

// ─── extractSurveyDenormFields / applyParentSurveyDenormalization ────────────

func TestExtractSurveyDenormFields(t *testing.T) {
	t.Run("keeps only the display fields and committee names", func(t *testing.T) {
		data := surveyV1Data()
		data["is_nps_survey"] = false // distinguish it from is_project_survey

		got, err := extractSurveyDenormFields(data)
		require.NoError(t, err)

		assert.Equal(t, &domain.SurveyData{
			SurveyTitle: "Q4 NPS", SurveyStatus: "sent", SurveySendDate: "2030-01-03T00:00:00Z",
			SurveyCutoffDate: "2030-01-31T00:00:00Z", IsNPSSurvey: false, IsProjectSurvey: true,
			CommitteeCategory: "Board", CreatorName: "Alice Example", CreatedAt: "2030-01-01T00:00:00Z",
			LastModifiedAt: "2030-01-02T00:00:00Z", TotalResponses: 17, TotalRecipients: 20,
			Committees: []domain.SurveyCommitteeData{
				{CommitteeID: "c1", CommitteeName: "Board"},
				{CommitteeID: "c2", CommitteeName: "TSC"},
			},
		}, got)
	})

	t.Run("a survey without committees has none", func(t *testing.T) {
		data := surveyV1Data()
		delete(data, "committees")

		got, err := extractSurveyDenormFields(data)
		require.NoError(t, err)
		assert.Empty(t, got.Committees)
	})

	t.Run("a non-numeric counter is an error", func(t *testing.T) {
		data := surveyV1Data()
		data["total_responses"] = "many"

		_, err := extractSurveyDenormFields(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal into SurveyDBRaw")
	})

	t.Run("an unmarshalable value is an error", func(t *testing.T) {
		_, err := extractSurveyDenormFields(map[string]any{"id": func() {}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal survey data")
	})
}

func TestApplyParentSurveyDenormalization(t *testing.T) {
	survey := func() *domain.SurveyData {
		return &domain.SurveyData{
			SurveyTitle: "Title", SurveyStatus: "Status", SurveySendDate: "Send", SurveyCutoffDate: "Cutoff",
			IsNPSSurvey: true, IsProjectSurvey: false, CommitteeCategory: "Category", CreatorName: "Creator",
			CreatedAt: "Created", LastModifiedAt: "Modified", TotalResponses: 7, TotalRecipients: 8,
			Committees: []domain.SurveyCommitteeData{
				{CommitteeID: "c1", CommitteeName: "First"},
				{CommitteeID: "c2", CommitteeName: "Second"},
			},
		}
	}

	t.Run("copies each survey field onto its response counterpart", func(t *testing.T) {
		r := &domain.SurveyResponseData{CommitteeID: "c2"}
		applyParentSurveyDenormalization(r, survey())

		assert.Equal(t, &domain.SurveyResponseData{
			CommitteeID: "c2", CommitteeName: "Second",
			SurveyTitle: "Title", SurveyStatus: "Status", SurveySendDate: "Send", SurveyCutoffDate: "Cutoff",
			IsNPSSurvey: true, IsProjectSurvey: false, CommitteeCategory: "Category", CreatorName: "Creator",
			SurveyCreatedAt: "Created", SurveyLastModifiedAt: "Modified", TotalResponses: 7, TotalRecipients: 8,
		}, r)
	})

	t.Run("booleans are not swapped", func(t *testing.T) {
		s := survey()
		s.IsNPSSurvey, s.IsProjectSurvey = false, true
		r := &domain.SurveyResponseData{}
		applyParentSurveyDenormalization(r, s)

		assert.False(t, r.IsNPSSurvey)
		assert.True(t, r.IsProjectSurvey)
	})

	t.Run("an unmatched committee id falls back to the first committee", func(t *testing.T) {
		r := &domain.SurveyResponseData{CommitteeID: "other"}
		applyParentSurveyDenormalization(r, survey())
		assert.Equal(t, "First", r.CommitteeName)
	})

	t.Run("a response with no committee id uses the first committee", func(t *testing.T) {
		r := &domain.SurveyResponseData{}
		applyParentSurveyDenormalization(r, survey())
		assert.Equal(t, "First", r.CommitteeName)
	})

	t.Run("a response with no committee id does not match a committee with a blank id", func(t *testing.T) {
		s := survey()
		s.Committees = []domain.SurveyCommitteeData{
			{CommitteeID: "c1", CommitteeName: "First"},
			{CommitteeID: "", CommitteeName: "Blank"},
		}
		r := &domain.SurveyResponseData{}
		applyParentSurveyDenormalization(r, s)
		assert.Equal(t, "First", r.CommitteeName)
	})

	t.Run("a survey with no committees leaves the committee name empty", func(t *testing.T) {
		s := survey()
		s.Committees = nil
		r := &domain.SurveyResponseData{CommitteeID: "c1"}
		applyParentSurveyDenormalization(r, s)
		assert.Empty(t, r.CommitteeName)
	})
}

// ─── handleSurveyResponseUpdate ──────────────────────────────────────────────

func TestHandleSurveyResponseUpdate(t *testing.T) {
	ctx := context.Background()
	const key = "itx-survey-responses.resp-1"

	update := func(t *testing.T, data map[string]any, pub *mocks.MockEventPublisher, mappings, objects *fakeKV) bool {
		t.Helper()
		return handleSurveyResponseUpdate(ctx, key, data, pub, v1IDMapper(), mappings, objects, nil, discardLogger())
	}

	t.Run("publishes created with the parent survey denormalized, then stores the mapping", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		mappings := parentMappingKV()

		retry := update(t, responseV1Data(), pub, mappings, parentSurveyKV(t))

		assert.False(t, retry)
		require.Len(t, *events, 1)
		assert.Equal(t, "created", (*events)[0].Action)
		got := (*events)[0].Response
		assert.Equal(t, "resp-1", got.UID)
		assert.Equal(t, "v2-project-p1", got.Project.ProjectUID)
		assert.Equal(t, "v2-committee-c1", got.CommitteeUID)
		assert.Equal(t, "Q4 NPS", got.SurveyTitle)
		assert.Equal(t, "sent", got.SurveyStatus)
		assert.Equal(t, "Board", got.CommitteeName, "matched by the response's committee id")
		assert.Equal(t, 17, got.TotalResponses)
		assert.Equal(t, 20, got.TotalRecipients)
		assert.Equal(t, "1", mappings.value("survey_response.resp-1"))
	})

	t.Run("denormalizes a msgpack-encoded parent survey", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		objects := newFakeKV(nil)
		packed, err := msgpack.Marshal(surveyV1Data())
		require.NoError(t, err)
		objects.data["itx-surveys.survey-1"] = packed

		update(t, responseV1Data(), pub, parentMappingKV(), objects)

		require.Len(t, *events, 1)
		assert.Equal(t, "Q4 NPS", (*events)[0].Response.SurveyTitle)
	})

	t.Run("publishes updated when a mapping already exists", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		mappings := parentMappingKV()
		mappings.data["survey_response.resp-1"] = []byte("1")

		assert.False(t, update(t, responseV1Data(), pub, mappings, parentSurveyKV(t)))
		require.Len(t, *events, 1)
		assert.Equal(t, "updated", (*events)[0].Action)
	})

	t.Run("a conversion failure is acknowledged without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		data := responseV1Data()
		data["nps_value"] = "bad"

		assert.False(t, update(t, data, pub, parentMappingKV(), parentSurveyKV(t)))
		assert.Empty(t, *events)
	})

	t.Run("a missing uid is acknowledged without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		data := responseV1Data()
		delete(data, "id")

		assert.False(t, update(t, data, pub, parentMappingKV(), parentSurveyKV(t)))
		assert.Empty(t, *events)
	})

	t.Run("a missing parent survey id is acknowledged without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		data := responseV1Data()
		delete(data, "survey_id")

		assert.False(t, update(t, data, pub, parentMappingKV(), parentSurveyKV(t)))
		assert.Empty(t, *events)
	})

	t.Run("an unknown parent survey is retried", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		mappings := newFakeKV(nil)

		assert.True(t, update(t, responseV1Data(), pub, mappings, parentSurveyKV(t)))
		assert.Empty(t, *events)
		assert.Empty(t, mappings.data)
	})

	t.Run("a tombstoned parent survey is retried", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		mappings := newFakeKV(map[string]string{"survey.survey-1": tombstoneMarker})

		assert.True(t, update(t, responseV1Data(), pub, mappings, parentSurveyKV(t)))
		assert.Empty(t, *events)
	})

	t.Run("a response with no resolvable project is skipped", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		mappings := parentMappingKV()

		retry := handleSurveyResponseUpdate(ctx, key, responseV1Data(), pub, v1IDMapper("p1"),
			mappings, parentSurveyKV(t), nil, discardLogger())

		assert.False(t, retry)
		assert.Empty(t, *events)
		assert.NotContains(t, mappings.data, "survey_response.resp-1")
	})

	t.Run("denormalization degrades gracefully", func(t *testing.T) {
		assertNotDenormalized := func(t *testing.T, events *[]publishedEvent) {
			t.Helper()
			require.Len(t, *events, 1)
			got := (*events)[0].Response
			assert.Equal(t, "resp-1", got.UID, "the response is still published")
			assert.Empty(t, got.SurveyTitle)
			assert.Empty(t, got.CommitteeName)
			assert.Zero(t, got.TotalResponses)
		}

		t.Run("parent survey not in v1-objects", func(t *testing.T) {
			pub, events := newRecordingPublisher(nil)
			assert.False(t, update(t, responseV1Data(), pub, parentMappingKV(), newFakeKV(nil)))
			assertNotDenormalized(t, events)
		})

		t.Run("parent survey is not valid JSON or msgpack", func(t *testing.T) {
			pub, events := newRecordingPublisher(nil)
			objects := newFakeKV(map[string]string{"itx-surveys.survey-1": "\xc1"})
			assert.False(t, update(t, responseV1Data(), pub, parentMappingKV(), objects))
			assertNotDenormalized(t, events)
		})

		t.Run("parent survey has an unreadable counter", func(t *testing.T) {
			pub, events := newRecordingPublisher(nil)
			data := surveyV1Data()
			data["total_responses"] = "many"
			objects := newFakeKV(map[string]string{"itx-surveys.survey-1": string(mustJSON(t, data))})
			assert.False(t, update(t, responseV1Data(), pub, parentMappingKV(), objects))
			assertNotDenormalized(t, events)
		})

		t.Run("a permanent fetch error", func(t *testing.T) {
			pub, events := newRecordingPublisher(nil)
			objects := newFakeKV(nil)
			objects.getErr = map[string]error{"itx-surveys.survey-1": errors.New("bucket corrupted")}
			assert.False(t, update(t, responseV1Data(), pub, parentMappingKV(), objects))
			assertNotDenormalized(t, events)
		})
	})

	t.Run("a transient parent survey fetch error is retried without publishing", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		objects := newFakeKV(nil)
		objects.getErr = map[string]error{"itx-surveys.survey-1": nats.ErrTimeout}

		assert.True(t, update(t, responseV1Data(), pub, parentMappingKV(), objects))
		assert.Empty(t, *events)
	})

	t.Run("a transient publish failure is retried and stores no mapping", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nats.ErrTimeout)
		mappings := parentMappingKV()

		assert.True(t, update(t, responseV1Data(), pub, mappings, parentSurveyKV(t)))
		assert.NotContains(t, mappings.data, "survey_response.resp-1")
	})

	t.Run("a permanent publish failure is acknowledged and stores no mapping", func(t *testing.T) {
		pub, _ := newRecordingPublisher(errors.New("invalid payload"))
		mappings := parentMappingKV()

		assert.False(t, update(t, responseV1Data(), pub, mappings, parentSurveyKV(t)))
		assert.NotContains(t, mappings.data, "survey_response.resp-1")
	})

	t.Run("a mapping store failure after publish is still acknowledged", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		mappings := parentMappingKV()
		mappings.putErr = errors.New("kv down")

		assert.False(t, update(t, responseV1Data(), pub, mappings, parentSurveyKV(t)))
		assert.Len(t, *events, 1)
	})
}

// ─── handleSurveyResponseUpdate: LFID invite ────────────────────────────────

func TestHandleSurveyResponseUpdate_Invite(t *testing.T) {
	ctx := context.Background()
	const key = "itx-survey-responses.resp-1"

	// newHandler wires an invite handler whose survey lookup succeeds.
	newHandler := func(t *testing.T, mappings *fakeKV) (*SurveyResponseInviteHandler, *mocks.MockInviteSender, *fakeKV) {
		t.Helper()
		survey := surveyV1Data()
		survey["name"] = "Q4 NPS Survey"
		objects := newFakeKV(map[string]string{"itx-surveys.survey-1": string(mustJSON(t, survey))})
		sender := &mocks.MockInviteSender{Result: &domain.InviteResult{InviteUID: "invite-1"}}
		return &SurveyResponseInviteHandler{
			inviteSender:     sender,
			userReader:       &mocks.MockUserReader{Err: domain.ErrUserNotFound},
			v1ObjectsKV:      objects,
			v1MappingsKV:     mappings,
			selfServeBaseURL: "https://app.example.com",
		}, sender, objects
	}

	t.Run("a new response without a username sends an invite", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nil)
		mappings := parentMappingKV()
		invites, sender, objects := newHandler(t, mappings)
		data := responseV1Data()
		data["username"] = ""

		retry := handleSurveyResponseUpdate(ctx, key, data, pub, v1IDMapper(), mappings, objects, invites, discardLogger())

		assert.False(t, retry)
		require.True(t, sender.Called)
		assert.Equal(t, "alice@example.com", sender.LastRequest.Recipient.Email)
		assert.Equal(t, "Alice Example", sender.LastRequest.Recipient.Name)
		assert.Equal(t, "survey-1", sender.LastRequest.Resource.UID)
		assert.Equal(t, "Q4 NPS Survey", sender.LastRequest.Resource.Name)
		assert.Equal(t, "https://app.example.com/surveys/survey-1", sender.LastRequest.ReturnURL)
		assert.Equal(t, "invite-1", mappings.value("v1_survey_response_lfid_invite_sent.resp-1"))
	})

	t.Run("a new response that already has a username sends no invite", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nil)
		mappings := parentMappingKV()
		invites, sender, objects := newHandler(t, mappings)

		handleSurveyResponseUpdate(ctx, key, responseV1Data(), pub, v1IDMapper(), mappings, objects, invites, discardLogger())

		assert.False(t, sender.Called)
	})

	t.Run("an updated response without a username sends no invite", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nil)
		mappings := parentMappingKV()
		mappings.data["survey_response.resp-1"] = []byte("1")
		invites, sender, objects := newHandler(t, mappings)
		data := responseV1Data()
		data["username"] = ""

		handleSurveyResponseUpdate(ctx, key, data, pub, v1IDMapper(), mappings, objects, invites, discardLogger())

		assert.False(t, sender.Called)
	})

	t.Run("a failed publish sends no invite", func(t *testing.T) {
		pub, _ := newRecordingPublisher(errors.New("invalid payload"))
		mappings := parentMappingKV()
		invites, sender, objects := newHandler(t, mappings)
		data := responseV1Data()
		data["username"] = ""

		handleSurveyResponseUpdate(ctx, key, data, pub, v1IDMapper(), mappings, objects, invites, discardLogger())

		assert.False(t, sender.Called)
	})
}

// ─── handleSurveyResponseDelete ──────────────────────────────────────────────

func TestHandleSurveyResponseDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("publishes deleted with only the ids, then tombstones the mapping", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey_response.resp-1": "1"})

		retry := handleSurveyResponseDelete(ctx, "resp-1", pub, kv, discardLogger())

		assert.False(t, retry)
		require.Len(t, *events, 1)
		assert.Equal(t, "deleted", (*events)[0].Action)
		assert.Equal(t, &domain.SurveyResponseData{UID: "resp-1", ID: "resp-1"}, (*events)[0].Response)
		assert.Equal(t, tombstoneMarker, kv.value("survey_response.resp-1"))
	})

	t.Run("deleting a response with no mapping still publishes", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)

		assert.False(t, handleSurveyResponseDelete(ctx, "resp-1", pub, kv, discardLogger()))
		assert.Len(t, *events, 1)
		assert.Equal(t, tombstoneMarker, kv.value("survey_response.resp-1"))
	})

	t.Run("an already tombstoned response is not published again", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(map[string]string{"survey_response.resp-1": tombstoneMarker})

		assert.False(t, handleSurveyResponseDelete(ctx, "resp-1", pub, kv, discardLogger()))
		assert.Empty(t, *events)
	})

	t.Run("a transient publish failure is retried and does not tombstone", func(t *testing.T) {
		pub, _ := newRecordingPublisher(nats.ErrNoResponders)
		kv := newFakeKV(map[string]string{"survey_response.resp-1": "1"})

		assert.True(t, handleSurveyResponseDelete(ctx, "resp-1", pub, kv, discardLogger()))
		assert.Equal(t, "1", kv.value("survey_response.resp-1"))
	})

	t.Run("a permanent publish failure is acknowledged and does not tombstone", func(t *testing.T) {
		pub, _ := newRecordingPublisher(errors.New("invalid payload"))
		kv := newFakeKV(map[string]string{"survey_response.resp-1": "1"})

		assert.False(t, handleSurveyResponseDelete(ctx, "resp-1", pub, kv, discardLogger()))
		assert.Equal(t, "1", kv.value("survey_response.resp-1"))
	})

	t.Run("a tombstone store failure is still acknowledged", func(t *testing.T) {
		pub, events := newRecordingPublisher(nil)
		kv := newFakeKV(nil)
		kv.putErr = errors.New("kv down")

		assert.False(t, handleSurveyResponseDelete(ctx, "resp-1", pub, kv, discardLogger()))
		assert.Len(t, *events, 1)
	})
}
