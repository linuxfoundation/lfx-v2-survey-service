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

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// CreateExclusion creates a survey or global exclusion in ITX
func (c *Client) CreateExclusion(ctx context.Context, req *itx.ExclusionRequest) (*itx.Exclusion, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/exclusion", c.config.BaseURL)
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

	// Handle non-2xx status codes (ITX returns 201 on success)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Unmarshal response
	var result itx.Exclusion
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &result, nil
}

// DeleteExclusion deletes a survey or global exclusion in ITX
func (c *Client) DeleteExclusion(ctx context.Context, req *itx.ExclusionRequest) error {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/exclusion", c.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, bytes.NewReader(body))
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

	// Handle non-2xx status codes (ITX returns 204 on success)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// GetExclusion retrieves an exclusion by ID from ITX
func (c *Client) GetExclusion(ctx context.Context, exclusionID string) (*itx.ExtendedExclusion, error) {
	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/exclusion/%s", c.config.BaseURL, exclusionID)
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
	var result itx.ExtendedExclusion
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &result, nil
}

// DeleteExclusionByID deletes an exclusion by its ID from ITX
func (c *Client) DeleteExclusionByID(ctx context.Context, exclusionID string) error {
	// Create HTTP request
	url := fmt.Sprintf("%sv2/surveys/exclusion/%s", c.config.BaseURL, exclusionID)
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

	// Handle non-2xx status codes (ITX returns 204 on success)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}
