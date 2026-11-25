// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── ListSurveyTemplates ─────────────────────────────────────────────────────

func TestListSurveyTemplates_GetsTemplatesPath(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"id":"412349483","title":"Board Member NPS","nickname":"Board NPS","href":"https://api.surveymonkey.com/v3/surveys/412349483"},
			{"id":"512349484","title":"Contributor NPS","nickname":"","href":"https://api.surveymonkey.com/v3/surveys/512349484"}
		]`))
	}))
	defer srv.Close()

	templates, err := testClient(srv).ListSurveyTemplates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("expected method GET, got %q", gotMethod)
	}
	if gotPath != "/v2/surveys/templates" {
		t.Errorf("expected path /v2/surveys/templates, got %q", gotPath)
	}
	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(templates))
	}
	first := templates[0]
	if first.ID != "412349483" || first.Title != "Board Member NPS" || first.Nickname != "Board NPS" ||
		first.Href != "https://api.surveymonkey.com/v3/surveys/412349483" {
		t.Errorf("unexpected first template: %+v", first)
	}
	if templates[1].ID != "512349484" || templates[1].Nickname != "" {
		t.Errorf("unexpected second template: %+v", templates[1])
	}
}

func TestListSurveyTemplates_NullBodyReturnsEmptySlice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`null`))
	}))
	defer srv.Close()

	templates, err := testClient(srv).ListSurveyTemplates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if templates == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(templates) != 0 {
		t.Errorf("expected empty slice, got %d items", len(templates))
	}
}

func TestListSurveyTemplates_Non2xxMapsToDomainError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"403", "message":"not authorized for this method"}`))
	}))
	defer srv.Close()

	templates, err := testClient(srv).ListSurveyTemplates(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if templates != nil {
		t.Errorf("expected nil templates on error, got %+v", templates)
	}
	var domErr *domain.DomainError
	if !errors.As(err, &domErr) {
		t.Fatalf("expected *domain.DomainError, got %T: %v", err, err)
	}
}

// ─── survey operations ───────────────────────────────────────────────────────

func TestSurveyOperations(t *testing.T) {
	ctx := context.Background()
	str := func(v string) *string { return &v }
	committee := "committee-1"
	emptyStr := ""

	scheduled := `{"id":"survey-1","survey_title":"Q4 NPS","survey_status":"scheduled","response_status":"open","total_recipients":12,"committees":[{"committee_id":"committee-1"}]}`
	checkScheduled := func(t *testing.T, got any) {
		resp := got.(*itx.SurveyScheduleResponse)
		assert.Equal(t, "survey-1", resp.ID)
		require.NotNil(t, resp.SurveyTitle)
		assert.Equal(t, "Q4 NPS", *resp.SurveyTitle)
		assert.Equal(t, "scheduled", resp.SurveyStatus)
		require.NotNil(t, resp.TotalRecipients)
		assert.Equal(t, 12, *resp.TotalRecipients)
		require.Len(t, resp.Committees, 1)
	}

	getSurvey := func(params *itx.GetSurveyParams) func(c *Client) (any, error) {
		return func(c *Client) (any, error) { return result(c.GetSurvey(ctx, "survey-1", params)) }
	}
	previewSend := func(id *string) func(c *Client) (any, error) {
		return func(c *Client) (any, error) { return result(c.PreviewSend(ctx, "survey-1", id)) }
	}
	sendMissing := func(id *string) func(c *Client) (any, error) {
		return func(c *Client) (any, error) { return noResult(c.SendMissingRecipients(ctx, "survey-1", id)) }
	}
	deleteGroup := func(committeeID, projectID, foundationID *string) func(c *Client) (any, error) {
		return func(c *Client) (any, error) {
			return noResult(c.DeleteRecipientGroup(ctx, "survey-1", committeeID, projectID, foundationID))
		}
	}
	checkPreview := func(t *testing.T, got any) {
		resp := got.(*itx.PreviewSendResponse)
		require.Len(t, resp.AffectedRecipients, 1)
		assert.Equal(t, "user-1", resp.AffectedRecipients[0].UserID)
		assert.Equal(t, "alice@example.com", resp.AffectedRecipients[0].Email)
		require.Len(t, resp.AffectedCommittees, 1)
		assert.Equal(t, "committee-1", resp.AffectedCommittees[0].CommitteeID)
		require.Len(t, resp.AffectedProjects, 1)
		assert.Equal(t, "proj-1", resp.AffectedProjects[0].ID)
	}
	previewBody := `{"affected_projects":[{"id":"proj-1","name":"P","slug":"p","status":"active"}],` +
		`"affected_committees":[{"project_id":"proj-1","project_name":"P","committee_id":"committee-1","committee_name":"C","committee_category":"Board"}],` +
		`"affected_recipients":[{"user_id":"user-1","email":"alice@example.com"}]}`

	runOperations(t, []operation{
		{
			name: "ScheduleSurvey",
			call: func(c *Client) (any, error) {
				return result(c.ScheduleSurvey(ctx, &itx.ScheduleSurveyRequest{
					SurveyTitle:     str("Q4 NPS"),
					SurveyMonkeyID:  str("412349483"),
					SendImmediately: func() *bool { v := true; return &v }(),
					Committees:      []string{"committee-1"},
				}))
			},
			method:   http.MethodPost,
			path:     "/v2/surveys/schedule",
			wantBody: `{"survey_title":"Q4 NPS","survey_monkey_id":"412349483","send_immediately":true,"committees":["committee-1"]}`,
			respBody: scheduled,
			check:    checkScheduled,
		},
		{
			name:     "GetSurvey/nil params",
			call:     getSurvey(nil),
			method:   http.MethodGet,
			path:     "/v2/surveys/survey-1/schedule",
			respBody: scheduled,
			check:    checkScheduled,
		},
		{
			name:     "GetSurvey/both params",
			call:     getSurvey(&itx.GetSurveyParams{ProjectID: str("proj-1"), ProjectIDs: str("proj-1,proj-2")}),
			method:   http.MethodGet,
			path:     "/v2/surveys/survey-1/schedule",
			query:    url.Values{"project_id": {"proj-1"}, "project_ids": {"proj-1,proj-2"}},
			respBody: scheduled,
			check:    checkScheduled,
		},
		{
			name:     "GetSurvey/empty params omitted",
			call:     getSurvey(&itx.GetSurveyParams{ProjectID: &emptyStr, ProjectIDs: &emptyStr}),
			method:   http.MethodGet,
			path:     "/v2/surveys/survey-1/schedule",
			respBody: scheduled,
			check:    checkScheduled,
		},
		{
			name: "UpdateSurvey",
			call: func(c *Client) (any, error) {
				return result(c.UpdateSurvey(ctx, "survey-1", &itx.UpdateSurveyRequest{
					SurveyTitle: str("Renamed"),
					Committees:  []string{"committee-2"},
				}))
			},
			method:   http.MethodPut,
			path:     "/v2/surveys/survey-1/schedule",
			wantBody: `{"survey_title":"Renamed","committees":["committee-2"]}`,
			respBody: scheduled,
			check:    checkScheduled,
		},
		{
			name: "DeleteSurvey",
			call: func(c *Client) (any, error) {
				return noResult(c.DeleteSurvey(ctx, "survey-1"))
			},
			method: http.MethodDelete,
			path:   "/v2/surveys/survey-1/schedule",
		},
		{
			name: "ExtendSurvey",
			call: func(c *Client) (any, error) {
				return result(c.ExtendSurvey(ctx, "survey-1", &itx.ExtendSurveyRequest{SurveyCutoffDate: "2030-01-31T00:00:00Z"}))
			},
			method:   http.MethodPost,
			path:     "/v2/surveys/survey-1/extend",
			wantBody: `{"survey_cutoff_date":"2030-01-31T00:00:00Z"}`,
			respBody: scheduled,
			check:    checkScheduled,
		},
		{
			name: "EnableSurvey",
			call: func(c *Client) (any, error) {
				return noResult(c.EnableSurvey(ctx, "survey-1"))
			},
			method: http.MethodPut,
			path:   "/v2/surveys/survey-1/enable",
		},
		{
			name: "BulkResendSurvey",
			call: func(c *Client) (any, error) {
				return noResult(c.BulkResendSurvey(ctx, "survey-1", &itx.BulkResendRequest{RecipientIDs: []string{"r1", "r2"}}))
			},
			method:   http.MethodPost,
			path:     "/v2/surveys/survey-1/bulk_resend",
			wantBody: `{"recipient_ids":["r1","r2"]}`,
		},
		{
			name:     "PreviewSend/no committee",
			call:     previewSend(nil),
			method:   http.MethodGet,
			path:     "/v2/surveys/survey-1/preview_send",
			respBody: previewBody,
			check:    checkPreview,
		},
		{
			name:     "PreviewSend/empty committee omitted",
			call:     previewSend(&emptyStr),
			method:   http.MethodGet,
			path:     "/v2/surveys/survey-1/preview_send",
			respBody: previewBody,
			check:    checkPreview,
		},
		{
			name:     "PreviewSend/with committee",
			call:     previewSend(&committee),
			method:   http.MethodGet,
			path:     "/v2/surveys/survey-1/preview_send",
			query:    url.Values{"committee_id": {"committee-1"}},
			respBody: previewBody,
			check:    checkPreview,
		},
		{
			name:   "SendMissingRecipients/no committee",
			call:   sendMissing(nil),
			method: http.MethodPost,
			path:   "/v2/surveys/survey-1/send_missing_recipients",
		},
		{
			name:   "SendMissingRecipients/empty committee omitted",
			call:   sendMissing(&emptyStr),
			method: http.MethodPost,
			path:   "/v2/surveys/survey-1/send_missing_recipients",
		},
		{
			name:   "SendMissingRecipients/with committee",
			call:   sendMissing(&committee),
			method: http.MethodPost,
			path:   "/v2/surveys/survey-1/send_missing_recipients",
			query:  url.Values{"committee_id": {"committee-1"}},
		},
		{
			name:   "DeleteRecipientGroup/no selectors",
			call:   deleteGroup(nil, nil, nil),
			method: http.MethodDelete,
			path:   "/v2/surveys/survey-1/recipient_group",
		},
		{
			name:   "DeleteRecipientGroup/empty selectors omitted",
			call:   deleteGroup(&emptyStr, &emptyStr, &emptyStr),
			method: http.MethodDelete,
			path:   "/v2/surveys/survey-1/recipient_group",
		},
		{
			name:   "DeleteRecipientGroup/committee only",
			call:   deleteGroup(&committee, nil, nil),
			method: http.MethodDelete,
			path:   "/v2/surveys/survey-1/recipient_group",
			query:  url.Values{"committee_id": {"committee-1"}},
		},
		{
			name:   "DeleteRecipientGroup/project and foundation",
			call:   deleteGroup(nil, str("proj-1"), str("found-1")),
			method: http.MethodDelete,
			path:   "/v2/surveys/survey-1/recipient_group",
			query:  url.Values{"project_id": {"proj-1"}, "foundation_id": {"found-1"}},
		},
		{
			name: "GetSurveyResults",
			call: func(c *Client) (any, error) {
				return result(c.GetSurveyResults(ctx, "survey-1"))
			},
			method: http.MethodGet,
			path:   "/v2/surveys/survey-1/results",
			respBody: `{"survey_results":[{"question_id":"q1","question_text":"NPS?","question_type":"rating","responses":[{"answer":"9","count":4,"percentage":50}]}],` +
				`"num_recipients":8,"num_responses":4}`,
			check: func(t *testing.T, got any) {
				res := got.(*itx.SurveyResults)
				assert.Equal(t, 8, res.NumRecipients)
				assert.Equal(t, 4, res.NumResponses)
				require.Len(t, res.SurveyResults, 1)
				assert.Equal(t, "q1", res.SurveyResults[0].QuestionID)
				assert.Equal(t, []itx.QuestionResponse{{Answer: "9", Count: 4, Percentage: 50}}, res.SurveyResults[0].Responses)
			},
		},
		{
			name: "ValidateEmail",
			call: func(c *Client) (any, error) {
				return result(c.ValidateEmail(ctx, &itx.ValidateEmailRequest{Body: str("<p>Hi {{name}}</p>"), Subject: str("Survey")}))
			},
			method:     http.MethodPost,
			path:       "/v2/surveys/validate_email",
			wantBody:   `{"body":"<p>Hi {{name}}</p>","subject":"Survey"}`,
			respBody:   `{"body":"<p>Hi Alice</p>","subject":"Survey"}`,
			wantResult: &itx.ValidateEmailResponse{Body: "<p>Hi Alice</p>", Subject: "Survey"},
		},
	})
}
