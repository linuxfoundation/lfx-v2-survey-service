// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"net/http"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

func TestExclusionOperations(t *testing.T) {
	email := "alice@example.com"
	username := "alice"
	surveyID := "survey-1"
	committeeID := "committee-1"
	globalExclusion := "true"
	req := &itx.ExclusionRequest{
		Email:           &email,
		SurveyID:        &surveyID,
		CommitteeID:     &committeeID,
		GlobalExclusion: &globalExclusion,
	}
	reqJSON := `{"email":"alice@example.com","survey_id":"survey-1","committee_id":"committee-1","global_exclusion":"true"}`
	ctx := context.Background()

	runOperations(t, []operation{
		{
			name: "CreateExclusion",
			call: func(c *Client) (any, error) {
				return result(c.CreateExclusion(ctx, req))
			},
			method:     http.MethodPost,
			path:       "/v2/surveys/exclusion",
			wantBody:   reqJSON,
			respBody:   `{"id":"excl-1","email":"alice@example.com","survey_id":"survey-1","committee_id":"committee-1"}`,
			wantResult: &itx.Exclusion{ID: "excl-1", Email: &email, SurveyID: &surveyID, CommitteeID: &committeeID},
		},
		{
			name: "DeleteExclusion",
			call: func(c *Client) (any, error) {
				return noResult(c.DeleteExclusion(ctx, req))
			},
			method:   http.MethodDelete,
			path:     "/v2/surveys/exclusion",
			wantBody: reqJSON,
		},
		{
			name: "GetExclusion",
			call: func(c *Client) (any, error) {
				return result(c.GetExclusion(ctx, "excl-1"))
			},
			method:   http.MethodGet,
			path:     "/v2/surveys/exclusion/excl-1",
			respBody: `{"id":"excl-1","email":"alice@example.com","user":{"username":"alice"}}`,
			wantResult: func() any {
				got := &itx.ExtendedExclusion{}
				got.ID = "excl-1"
				got.Email = &email
				got.User = &itx.ExclusionUser{Username: &username}
				return got
			}(),
		},
		{
			name: "DeleteExclusionByID",
			call: func(c *Client) (any, error) {
				return noResult(c.DeleteExclusionByID(ctx, "excl-1"))
			},
			method: http.MethodDelete,
			path:   "/v2/surveys/exclusion/excl-1",
		},
	})
}
