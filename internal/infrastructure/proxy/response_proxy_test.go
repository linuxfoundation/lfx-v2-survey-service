// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseOperations(t *testing.T) {
	ctx := context.Background()
	answer := "Great community"
	rating := 9
	answers := []itx.SurveyAnswer{
		{QuestionID: "q1", AnswerText: &answer},
		{QuestionID: "q2", RatingValue: &rating},
	}
	answersJSON := `[{"question_id":"q1","answer_text":"Great community"},{"question_id":"q2","rating_value":9}]`

	runOperations(t, []operation{
		{
			name: "CreateResponse",
			call: func(c *Client) (any, error) {
				return noResult(c.CreateResponse(ctx, &itx.CreateSurveyResponseRequest{
					SurveyResponseUID: "resp-1",
					SurveyUID:         "survey-1",
					Answers:           answers,
				}))
			},
			method:   http.MethodPost,
			path:     "/v2/surveys/responses",
			wantBody: `{"survey_response_uid":"resp-1","survey_uid":"survey-1","answers":` + answersJSON + `}`,
		},
		{
			name: "GetResponse",
			call: func(c *Client) (any, error) {
				return result(c.GetResponse(ctx, "resp-1"))
			},
			method:   http.MethodGet,
			path:     "/v2/surveys/responses/resp-1",
			respBody: `{"survey_response_uid":"resp-1","survey_uid":"survey-1","project_uid":"proj-1","response_status":"submitted","answers":[{"question_id":"q1","answer_text":"Great community"}]}`,
			check: func(t *testing.T, got any) {
				resp := got.(*itx.SurveyResponse)
				assert.Equal(t, "resp-1", resp.SurveyResponseUID)
				assert.Equal(t, "survey-1", resp.SurveyUID)
				assert.Equal(t, "proj-1", resp.ProjectUID)
				assert.Equal(t, "submitted", resp.ResponseStatus)
				require.Len(t, resp.Answers, 1)
				assert.Equal(t, "q1", resp.Answers[0].QuestionID)
			},
		},
		{
			name: "UpdateResponse",
			call: func(c *Client) (any, error) {
				return noResult(c.UpdateResponse(ctx, "resp-1", &itx.UpdateSurveyResponseRequest{Answers: answers}))
			},
			method:   http.MethodPut,
			path:     "/v2/surveys/responses/resp-1",
			wantBody: `{"answers":` + answersJSON + `}`,
		},
		{
			name: "DeleteResponse",
			call: func(c *Client) (any, error) {
				return noResult(c.DeleteResponse(ctx, "survey-1", "resp-1"))
			},
			method: http.MethodDelete,
			path:   "/v2/surveys/survey-1/responses/resp-1",
		},
		{
			name: "ResendResponse",
			call: func(c *Client) (any, error) {
				return noResult(c.ResendResponse(ctx, "survey-1", "resp-1"))
			},
			method: http.MethodPost,
			path:   "/v2/surveys/survey-1/responses/resp-1/resend",
		},
	})
}

func TestListResponses(t *testing.T) {
	ctx := context.Background()
	str := func(s string) *string { return &s }
	page := `{"data":[{"id":"r1","survey_id":"survey-1","email":"alice@example.com","response_status":"Responded"}],` +
		`"meta":{"page_token":"next","total_pages":3,"total_results":25,"per_page":10}}`

	checkPage := func(t *testing.T, got any) {
		p := got.(*itx.PaginatedSurveyResponses)
		require.Len(t, p.Data, 1)
		assert.Equal(t, "r1", p.Data[0].ID)
		assert.Equal(t, "survey-1", p.Data[0].SurveyID)
		require.NotNil(t, p.Data[0].Email)
		assert.Equal(t, "alice@example.com", *p.Data[0].Email)
		assert.Equal(t, itx.PageMetadata{PageToken: "next", TotalPages: 3, TotalResults: 25, PerPage: 10}, p.Meta)
	}

	call := func(params *itx.ListResponsesParams) func(c *Client) (any, error) {
		return func(c *Client) (any, error) { return result(c.ListResponses(ctx, "survey-1", params)) }
	}

	runOperations(t, []operation{
		{
			name:     "ListResponses/nil params",
			call:     call(nil),
			method:   http.MethodGet,
			path:     "/v2/surveys/survey-1/responses",
			respBody: page,
			check:    checkPage,
		},
		{
			name: "ListResponses/all params",
			call: call(&itx.ListResponsesParams{
				PageToken:  str("tok"),
				PerPage:    str("10"),
				ProjectID:  str("proj-1"),
				ProjectIDs: str("proj-1,proj-2"),
			}),
			method: http.MethodGet,
			path:   "/v2/surveys/survey-1/responses",
			query: url.Values{
				"page_token":  {"tok"},
				"per_page":    {"10"},
				"project_id":  {"proj-1"},
				"project_ids": {"proj-1,proj-2"},
			},
			respBody: page,
			check:    checkPage,
		},
		{
			name:     "ListResponses/empty-string params are omitted",
			call:     call(&itx.ListResponsesParams{PageToken: str(""), PerPage: str(""), ProjectID: str(""), ProjectIDs: str("")}),
			method:   http.MethodGet,
			path:     "/v2/surveys/survey-1/responses",
			respBody: page,
			check:    checkPage,
		},
		{
			name:     "ListResponses/only set params are sent",
			call:     call(&itx.ListResponsesParams{PerPage: str("5")}),
			method:   http.MethodGet,
			path:     "/v2/surveys/survey-1/responses",
			query:    url.Values{"per_page": {"5"}},
			respBody: page,
			check:    checkPage,
		},
	})
}
