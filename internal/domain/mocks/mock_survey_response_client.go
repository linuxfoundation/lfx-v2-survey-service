// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// MockSurveyResponseClient is a configurable test double for domain.SurveyResponseClient.
//
// ListResponses is the most commonly tested method and has dedicated capture fields
// (CapturedSurveyID, CapturedParams) that are populated on every call, making it easy
// to assert forwarding behaviour without a custom Func closure.
// All other methods use a Func field; when nil they panic.
type MockSurveyResponseClient struct {
	// ListResponses fixed return values (used when ListResponsesFunc is nil)
	ListResponsesResult *itx.PaginatedSurveyResponses
	ListResponsesErr    error
	// CapturedSurveyID and CapturedParams are set on every ListResponses call.
	CapturedSurveyID string
	CapturedParams   *itx.ListResponsesParams
	// ListResponsesFunc overrides the fixed values when non-nil.
	ListResponsesFunc func(ctx context.Context, surveyID string, params *itx.ListResponsesParams) (*itx.PaginatedSurveyResponses, error)

	CreateResponseFunc func(ctx context.Context, req *itx.CreateResponseRequest) error
	GetResponseFunc    func(ctx context.Context, responseID string) (*itx.ResponseResponse, error)
	UpdateResponseFunc func(ctx context.Context, responseID string, req *itx.UpdateResponseRequest) error
	DeleteResponseFunc func(ctx context.Context, surveyID string, responseID string) error
	ResendResponseFunc func(ctx context.Context, surveyID string, responseID string) error
}

// Compile-time assertion.
var _ domain.SurveyResponseClient = (*MockSurveyResponseClient)(nil)

func (m *MockSurveyResponseClient) ListResponses(ctx context.Context, surveyID string, params *itx.ListResponsesParams) (*itx.PaginatedSurveyResponses, error) {
	m.CapturedSurveyID = surveyID
	m.CapturedParams = params
	if m.ListResponsesFunc != nil {
		return m.ListResponsesFunc(ctx, surveyID, params)
	}
	return m.ListResponsesResult, m.ListResponsesErr
}

func (m *MockSurveyResponseClient) CreateResponse(ctx context.Context, req *itx.CreateResponseRequest) error {
	if m.CreateResponseFunc != nil {
		return m.CreateResponseFunc(ctx, req)
	}
	panic("MockSurveyResponseClient.CreateResponse: not configured")
}

func (m *MockSurveyResponseClient) GetResponse(ctx context.Context, responseID string) (*itx.ResponseResponse, error) {
	if m.GetResponseFunc != nil {
		return m.GetResponseFunc(ctx, responseID)
	}
	panic("MockSurveyResponseClient.GetResponse: not configured")
}

func (m *MockSurveyResponseClient) UpdateResponse(ctx context.Context, responseID string, req *itx.UpdateResponseRequest) error {
	if m.UpdateResponseFunc != nil {
		return m.UpdateResponseFunc(ctx, responseID, req)
	}
	panic("MockSurveyResponseClient.UpdateResponse: not configured")
}

func (m *MockSurveyResponseClient) DeleteResponse(ctx context.Context, surveyID string, responseID string) error {
	if m.DeleteResponseFunc != nil {
		return m.DeleteResponseFunc(ctx, surveyID, responseID)
	}
	panic("MockSurveyResponseClient.DeleteResponse: not configured")
}

func (m *MockSurveyResponseClient) ResendResponse(ctx context.Context, surveyID string, responseID string) error {
	if m.ResendResponseFunc != nil {
		return m.ResendResponseFunc(ctx, surveyID, responseID)
	}
	panic("MockSurveyResponseClient.ResendResponse: not configured")
}
