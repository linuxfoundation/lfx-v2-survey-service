// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// CreateExclusion creates a survey or global exclusion in ITX
func (c *Client) CreateExclusion(ctx context.Context, req *itx.ExclusionRequest) (*itx.Exclusion, error) {
	body, err := marshalJSON(req)
	if err != nil {
		return nil, err
	}
	return doJSON[itx.Exclusion](ctx, c, http.MethodPost,
		fmt.Sprintf("%sv2/surveys/exclusion", c.config.BaseURL), body)
}

// DeleteExclusion deletes a survey or global exclusion in ITX
func (c *Client) DeleteExclusion(ctx context.Context, req *itx.ExclusionRequest) error {
	body, err := marshalJSON(req)
	if err != nil {
		return err
	}
	return doNoBody(ctx, c, http.MethodDelete,
		fmt.Sprintf("%sv2/surveys/exclusion", c.config.BaseURL), body)
}

// GetExclusion retrieves an exclusion by ID from ITX
func (c *Client) GetExclusion(ctx context.Context, exclusionID string) (*itx.ExtendedExclusion, error) {
	return doJSON[itx.ExtendedExclusion](ctx, c, http.MethodGet,
		fmt.Sprintf("%sv2/surveys/exclusion/%s", c.config.BaseURL, exclusionID), nil)
}

// DeleteExclusionByID deletes an exclusion by its ID from ITX
func (c *Client) DeleteExclusionByID(ctx context.Context, exclusionID string) error {
	return doNoBody(ctx, c, http.MethodDelete,
		fmt.Sprintf("%sv2/surveys/exclusion/%s", c.config.BaseURL, exclusionID), nil)
}
