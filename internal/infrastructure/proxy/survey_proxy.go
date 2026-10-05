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

// ScheduleSurvey schedules a new survey in ITX
func (c *Client) ScheduleSurvey(ctx context.Context, req *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	body, err := marshalJSON(req)
	if err != nil {
		return nil, err
	}
	return doJSON[itx.SurveyScheduleResponse](ctx, c, http.MethodPost,
		fmt.Sprintf("%sv2/surveys/schedule", c.config.BaseURL), body)
}

// GetSurvey retrieves survey details from ITX
func (c *Client) GetSurvey(ctx context.Context, surveyID string, params *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
	u, err := url.Parse(fmt.Sprintf("%sv2/surveys/%s/schedule", c.config.BaseURL, surveyID))
	if err != nil {
		return nil, domain.NewInternalError("failed to parse URL", err)
	}
	if params != nil {
		q := u.Query()
		if params.ProjectID != nil && *params.ProjectID != "" {
			q.Add("project_id", *params.ProjectID)
		}
		if params.ProjectIDs != nil && *params.ProjectIDs != "" {
			q.Add("project_ids", *params.ProjectIDs)
		}
		u.RawQuery = q.Encode()
	}
	return doJSON[itx.SurveyScheduleResponse](ctx, c, http.MethodGet, u.String(), nil)
}

// UpdateSurvey updates a survey in ITX (only when status is "disabled")
func (c *Client) UpdateSurvey(ctx context.Context, surveyID string, req *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	body, err := marshalJSON(req)
	if err != nil {
		return nil, err
	}
	return doJSON[itx.SurveyScheduleResponse](ctx, c, http.MethodPut,
		fmt.Sprintf("%sv2/surveys/%s/schedule", c.config.BaseURL, surveyID), body)
}

// DeleteSurvey deletes a survey in ITX (only when status is "disabled")
func (c *Client) DeleteSurvey(ctx context.Context, surveyID string) error {
	return doNoBody(ctx, c, http.MethodDelete,
		fmt.Sprintf("%sv2/surveys/%s/schedule", c.config.BaseURL, surveyID), nil)
}

// ExtendSurvey extends a survey's schedule time in ITX
func (c *Client) ExtendSurvey(ctx context.Context, surveyID string, req *itx.ExtendSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	body, err := marshalJSON(req)
	if err != nil {
		return nil, err
	}
	return doJSON[itx.SurveyScheduleResponse](ctx, c, http.MethodPost,
		fmt.Sprintf("%sv2/surveys/%s/extend", c.config.BaseURL, surveyID), body)
}

// EnableSurvey enables a survey for responses in ITX
func (c *Client) EnableSurvey(ctx context.Context, surveyID string) error {
	return doNoBody(ctx, c, http.MethodPut,
		fmt.Sprintf("%sv2/surveys/%s/enable", c.config.BaseURL, surveyID), nil)
}

// BulkResendSurvey bulk resends survey emails to select recipients in ITX
func (c *Client) BulkResendSurvey(ctx context.Context, surveyID string, req *itx.BulkResendRequest) error {
	body, err := marshalJSON(req)
	if err != nil {
		return err
	}
	return doNoBody(ctx, c, http.MethodPost,
		fmt.Sprintf("%sv2/surveys/%s/bulk_resend", c.config.BaseURL, surveyID), body)
}

// PreviewSend previews which recipients would be affected by a resend
func (c *Client) PreviewSend(ctx context.Context, surveyID string, committeeID *string) (*itx.PreviewSendResponse, error) {
	u := fmt.Sprintf("%sv2/surveys/%s/preview_send", c.config.BaseURL, surveyID)
	if committeeID != nil && *committeeID != "" {
		u = fmt.Sprintf("%s?committee_id=%s", u, *committeeID)
	}
	return doJSON[itx.PreviewSendResponse](ctx, c, http.MethodGet, u, nil)
}

// SendMissingRecipients sends survey emails to committee members who haven't received it
func (c *Client) SendMissingRecipients(ctx context.Context, surveyID string, committeeID *string) error {
	u := fmt.Sprintf("%sv2/surveys/%s/send_missing_recipients", c.config.BaseURL, surveyID)
	if committeeID != nil && *committeeID != "" {
		u = fmt.Sprintf("%s?committee_id=%s", u, *committeeID)
	}
	return doNoBody(ctx, c, http.MethodPost, u, nil)
}

// DeleteRecipientGroup removes a recipient group from a survey and recalculates statistics in ITX
func (c *Client) DeleteRecipientGroup(ctx context.Context, surveyID string, committeeID *string, projectID *string, foundationID *string) error {
	u, err := url.Parse(fmt.Sprintf("%sv2/surveys/%s/recipient_group", c.config.BaseURL, surveyID))
	if err != nil {
		return domain.NewInternalError("failed to parse URL", err)
	}
	q := u.Query()
	if committeeID != nil && *committeeID != "" {
		q.Set("committee_id", *committeeID)
	}
	if projectID != nil && *projectID != "" {
		q.Set("project_id", *projectID)
	}
	if foundationID != nil && *foundationID != "" {
		q.Set("foundation_id", *foundationID)
	}
	u.RawQuery = q.Encode()
	return doNoBody(ctx, c, http.MethodDelete, u.String(), nil)
}

// GetSurveyResults retrieves aggregated survey results from ITX
func (c *Client) GetSurveyResults(ctx context.Context, surveyID string) (*itx.SurveyResults, error) {
	return doJSON[itx.SurveyResults](ctx, c, http.MethodGet,
		fmt.Sprintf("%sv2/surveys/%s/results", c.config.BaseURL, surveyID), nil)
}

// ValidateEmail validates email template body and subject in ITX
func (c *Client) ValidateEmail(ctx context.Context, req *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error) {
	body, err := marshalJSON(req)
	if err != nil {
		return nil, err
	}
	return doJSON[itx.ValidateEmailResponse](ctx, c, http.MethodPost,
		fmt.Sprintf("%sv2/surveys/validate_email", c.config.BaseURL), body)
}
