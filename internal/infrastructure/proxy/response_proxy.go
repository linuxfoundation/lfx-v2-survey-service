// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// CreateResponse submits a survey response in ITX
func (c *Client) CreateResponse(ctx context.Context, req *itx.CreateSurveyResponseRequest) error {
	body, err := marshalJSON(req)
	if err != nil {
		return err
	}
	return doNoBody(ctx, c, http.MethodPost,
		fmt.Sprintf("%sv2/surveys/responses", c.config.BaseURL), body)
}

// GetResponse retrieves survey response details from ITX
func (c *Client) GetResponse(ctx context.Context, responseID string) (*itx.SurveyResponse, error) {
	return doJSON[itx.SurveyResponse](ctx, c, http.MethodGet,
		fmt.Sprintf("%sv2/surveys/responses/%s", c.config.BaseURL, responseID), nil)
}

// ListResponses retrieves a paginated list of individual survey responses from ITX
func (c *Client) ListResponses(ctx context.Context, surveyID string, params *itx.ListResponsesParams) (*itx.PaginatedSurveyResponses, error) {
	u, err := url.Parse(fmt.Sprintf("%sv2/surveys/%s/responses", c.config.BaseURL, surveyID))
	if err != nil {
		return nil, domain.NewInternalError("failed to parse URL", err)
	}
	if params != nil {
		q := u.Query()
		if params.PageToken != nil && *params.PageToken != "" {
			q.Add("page_token", *params.PageToken)
		}
		if params.PerPage != nil && *params.PerPage != "" {
			q.Add("per_page", *params.PerPage)
		}
		if params.ProjectID != nil && *params.ProjectID != "" {
			q.Add("project_id", *params.ProjectID)
		}
		if params.ProjectIDs != nil && *params.ProjectIDs != "" {
			q.Add("project_ids", *params.ProjectIDs)
		}
		u.RawQuery = q.Encode()
	}
	return doJSON[itx.PaginatedSurveyResponses](ctx, c, http.MethodGet, u.String(), nil)
}

// UpdateResponse updates a survey response in ITX
func (c *Client) UpdateResponse(ctx context.Context, responseID string, req *itx.UpdateSurveyResponseRequest) error {
	body, err := marshalJSON(req)
	if err != nil {
		return err
	}
	return doNoBody(ctx, c, http.MethodPut,
		fmt.Sprintf("%sv2/surveys/responses/%s", c.config.BaseURL, responseID), body)
}

// DeleteResponse removes a recipient from a survey and recalculates statistics in ITX
func (c *Client) DeleteResponse(ctx context.Context, surveyID string, responseID string) error {
	return doNoBody(ctx, c, http.MethodDelete,
		fmt.Sprintf("%sv2/surveys/%s/responses/%s", c.config.BaseURL, surveyID, responseID), nil)
}

// ResendResponse resends the survey email to a specific user in ITX
func (c *Client) ResendResponse(ctx context.Context, surveyID string, responseID string) error {
	return doNoBody(ctx, c, http.MethodPost,
		fmt.Sprintf("%sv2/surveys/%s/responses/%s/resend", c.config.BaseURL, surveyID, responseID), nil)
}
