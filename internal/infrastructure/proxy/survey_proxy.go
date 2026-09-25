// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// ScheduleSurvey schedules a new survey in ITX
func (c *Client) ScheduleSurvey(ctx context.Context, req *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/schedule", c.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request (OAuth2 transport will add Authorization header automatically)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response directly into domain model
	var result itx.SurveyScheduleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// GetSurvey retrieves survey details from ITX
func (c *Client) GetSurvey(ctx context.Context, surveyID string, params *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
	// Build URL with query parameters
	baseURL := fmt.Sprintf("%sv2/surveys/%s/schedule", c.config.BaseURL, surveyID)
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, domain.NewInternalError("failed to parse URL", err)
	}

	// Add query parameters if provided
	if params != nil {
		query := parsedURL.Query()
		if params.ProjectID != nil && *params.ProjectID != "" {
			query.Add("project_id", *params.ProjectID)
		}
		if params.ProjectIDs != nil && *params.ProjectIDs != "" {
			query.Add("project_ids", *params.ProjectIDs)
		}
		parsedURL.RawQuery = query.Encode()
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.SurveyScheduleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// UpdateSurvey updates a survey in ITX (only when status is "disabled")
func (c *Client) UpdateSurvey(ctx context.Context, surveyID string, req *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/%s/schedule", c.config.BaseURL, surveyID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes (ITX returns 400 if status is not "disabled")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.SurveyScheduleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// DeleteSurvey deletes a survey in ITX (only when status is "disabled")
func (c *Client) DeleteSurvey(ctx context.Context, surveyID string) error {
	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/%s/schedule", c.config.BaseURL, surveyID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes (ITX returns 400 if status is not "disabled")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// ExtendSurvey extends a survey's schedule time in ITX
func (c *Client) ExtendSurvey(ctx context.Context, surveyID string, req *itx.ExtendSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/%s/extend", c.config.BaseURL, surveyID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.SurveyScheduleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// EnableSurvey enables a survey for responses in ITX
func (c *Client) EnableSurvey(ctx context.Context, surveyID string) error {
	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/%s/enable", c.config.BaseURL, surveyID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// BulkResendSurvey bulk resends survey emails to select recipients in ITX
func (c *Client) BulkResendSurvey(ctx context.Context, surveyID string, req *itx.BulkResendRequest) error {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/%s/bulk_resend", c.config.BaseURL, surveyID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// PreviewSend previews which recipients would be affected by a resend
func (c *Client) PreviewSend(ctx context.Context, surveyID string, committeeID *string) (*itx.PreviewSendResponse, error) {
	// Create HTTP request with optional committee_id query parameter
	url := fmt.Sprintf("%sv2/surveys/%s/preview_send", c.config.BaseURL, surveyID)
	if committeeID != nil && *committeeID != "" {
		url = fmt.Sprintf("%s?committee_id=%s", url, *committeeID)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Unmarshal response
	var result itx.PreviewSendResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &result, nil
}

// SendMissingRecipients sends survey emails to committee members who haven't received it
func (c *Client) SendMissingRecipients(ctx context.Context, surveyID string, committeeID *string) error {
	// Create HTTP request with optional committee_id query parameter
	url := fmt.Sprintf("%sv2/surveys/%s/send_missing_recipients", c.config.BaseURL, surveyID)
	if committeeID != nil && *committeeID != "" {
		url = fmt.Sprintf("%s?committee_id=%s", url, *committeeID)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// DeleteRecipientGroup removes a recipient group from survey and recalculates statistics in ITX
func (c *Client) DeleteRecipientGroup(ctx context.Context, surveyID string, committeeID *string, projectID *string, foundationID *string) error {
	// Create base URL
	baseURL := fmt.Sprintf("%sv2/surveys/%s/recipient_group", c.config.BaseURL, surveyID)

	// Parse URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return domain.NewInternalError("failed to parse URL", err)
	}

	// Build query parameters
	query := u.Query()
	if committeeID != nil && *committeeID != "" {
		query.Set("committee_id", *committeeID)
	}
	if projectID != nil && *projectID != "" {
		query.Set("project_id", *projectID)
	}
	if foundationID != nil && *foundationID != "" {
		query.Set("foundation_id", *foundationID)
	}
	u.RawQuery = query.Encode()

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes (ITX returns 204 on success)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// GetSurveyResults retrieves aggregated survey results from ITX
func (c *Client) GetSurveyResults(ctx context.Context, surveyID string) (*itx.SurveyResults, error) {
	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/%s/results", c.config.BaseURL, surveyID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.SurveyResults
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// ValidateEmail validates email template body and subject in ITX
func (c *Client) ValidateEmail(ctx context.Context, req *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/validate_email", c.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization header is automatically set by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:surveys")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Unmarshal response
	var result itx.ValidateEmailResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &result, nil
}
