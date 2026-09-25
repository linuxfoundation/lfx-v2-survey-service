// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// MockSurveyClient is a configurable test double for domain.SurveyClient.
// Each method has a corresponding Func field; when nil the method panics,
// making accidental calls immediately visible in test output.
type MockSurveyClient struct {
	ScheduleSurveyFunc        func(ctx context.Context, req *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error)
	GetSurveyFunc             func(ctx context.Context, surveyID string, queryParams *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error)
	UpdateSurveyFunc          func(ctx context.Context, surveyID string, req *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error)
	DeleteSurveyFunc          func(ctx context.Context, surveyID string) error
	ExtendSurveyFunc          func(ctx context.Context, surveyID string, req *itx.ExtendSurveyRequest) (*itx.SurveyScheduleResponse, error)
	EnableSurveyFunc          func(ctx context.Context, surveyID string) error
	BulkResendSurveyFunc      func(ctx context.Context, surveyID string, req *itx.BulkResendRequest) error
	PreviewSendFunc           func(ctx context.Context, surveyID string, committeeID *string) (*itx.PreviewSendResponse, error)
	SendMissingRecipientsFunc func(ctx context.Context, surveyID string, committeeID *string) error
	DeleteRecipientGroupFunc  func(ctx context.Context, surveyID string, committeeID *string, projectID *string, foundationID *string) error
	GetSurveyResultsFunc      func(ctx context.Context, surveyID string) (*itx.SurveyResults, error)
	ValidateEmailFunc         func(ctx context.Context, req *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error)
}

// Compile-time assertion.
var _ domain.SurveyClient = (*MockSurveyClient)(nil)

func (m *MockSurveyClient) ScheduleSurvey(ctx context.Context, req *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	if m.ScheduleSurveyFunc != nil {
		return m.ScheduleSurveyFunc(ctx, req)
	}
	panic("MockSurveyClient.ScheduleSurvey: not configured")
}

func (m *MockSurveyClient) GetSurvey(ctx context.Context, surveyID string, queryParams *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
	if m.GetSurveyFunc != nil {
		return m.GetSurveyFunc(ctx, surveyID, queryParams)
	}
	panic("MockSurveyClient.GetSurvey: not configured")
}

func (m *MockSurveyClient) UpdateSurvey(ctx context.Context, surveyID string, req *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	if m.UpdateSurveyFunc != nil {
		return m.UpdateSurveyFunc(ctx, surveyID, req)
	}
	panic("MockSurveyClient.UpdateSurvey: not configured")
}

func (m *MockSurveyClient) DeleteSurvey(ctx context.Context, surveyID string) error {
	if m.DeleteSurveyFunc != nil {
		return m.DeleteSurveyFunc(ctx, surveyID)
	}
	panic("MockSurveyClient.DeleteSurvey: not configured")
}

func (m *MockSurveyClient) ExtendSurvey(ctx context.Context, surveyID string, req *itx.ExtendSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	if m.ExtendSurveyFunc != nil {
		return m.ExtendSurveyFunc(ctx, surveyID, req)
	}
	panic("MockSurveyClient.ExtendSurvey: not configured")
}

func (m *MockSurveyClient) EnableSurvey(ctx context.Context, surveyID string) error {
	if m.EnableSurveyFunc != nil {
		return m.EnableSurveyFunc(ctx, surveyID)
	}
	panic("MockSurveyClient.EnableSurvey: not configured")
}

func (m *MockSurveyClient) BulkResendSurvey(ctx context.Context, surveyID string, req *itx.BulkResendRequest) error {
	if m.BulkResendSurveyFunc != nil {
		return m.BulkResendSurveyFunc(ctx, surveyID, req)
	}
	panic("MockSurveyClient.BulkResendSurvey: not configured")
}

func (m *MockSurveyClient) PreviewSend(ctx context.Context, surveyID string, committeeID *string) (*itx.PreviewSendResponse, error) {
	if m.PreviewSendFunc != nil {
		return m.PreviewSendFunc(ctx, surveyID, committeeID)
	}
	panic("MockSurveyClient.PreviewSend: not configured")
}

func (m *MockSurveyClient) SendMissingRecipients(ctx context.Context, surveyID string, committeeID *string) error {
	if m.SendMissingRecipientsFunc != nil {
		return m.SendMissingRecipientsFunc(ctx, surveyID, committeeID)
	}
	panic("MockSurveyClient.SendMissingRecipients: not configured")
}

func (m *MockSurveyClient) DeleteRecipientGroup(ctx context.Context, surveyID string, committeeID *string, projectID *string, foundationID *string) error {
	if m.DeleteRecipientGroupFunc != nil {
		return m.DeleteRecipientGroupFunc(ctx, surveyID, committeeID, projectID, foundationID)
	}
	panic("MockSurveyClient.DeleteRecipientGroup: not configured")
}

func (m *MockSurveyClient) GetSurveyResults(ctx context.Context, surveyID string) (*itx.SurveyResults, error) {
	if m.GetSurveyResultsFunc != nil {
		return m.GetSurveyResultsFunc(ctx, surveyID)
	}
	panic("MockSurveyClient.GetSurveyResults: not configured")
}

func (m *MockSurveyClient) ValidateEmail(ctx context.Context, req *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error) {
	if m.ValidateEmailFunc != nil {
		return m.ValidateEmailFunc(ctx, req)
	}
	panic("MockSurveyClient.ValidateEmail: not configured")
}
