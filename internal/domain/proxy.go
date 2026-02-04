// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// SurveyClient defines the interface for survey management operations in ITX
type SurveyClient interface {
	// ScheduleSurvey schedules a new survey in ITX
	ScheduleSurvey(ctx context.Context, req *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error)

	// GetSurvey retrieves survey details from ITX
	GetSurvey(ctx context.Context, surveyID string) (*itx.SurveyScheduleResponse, error)

	// UpdateSurvey updates a survey in ITX (only when status is "disabled")
	UpdateSurvey(ctx context.Context, surveyID string, req *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error)

	// DeleteSurvey deletes a survey in ITX (only when status is "disabled")
	DeleteSurvey(ctx context.Context, surveyID string) error

	// ExtendSurvey extends a survey's end time in ITX
	ExtendSurvey(ctx context.Context, surveyID string, req *itx.ExtendSurveyRequest) (*itx.SurveyScheduleResponse, error)

	// EnableSurvey enables a survey for responses in ITX
	EnableSurvey(ctx context.Context, surveyID string) error

	// BulkResendSurvey bulk resends survey emails to select recipients in ITX
	BulkResendSurvey(ctx context.Context, surveyID string, req *itx.BulkResendRequest) error

	// PreviewSend previews which recipients would be affected by a resend in ITX
	PreviewSend(ctx context.Context, surveyID string, committeeID *string) (*itx.PreviewSendResponse, error)

	// SendMissingRecipients sends survey emails to committee members who haven't received it in ITX
	SendMissingRecipients(ctx context.Context, surveyID string, committeeID *string) error

	// DeleteRecipientGroup removes a recipient group from survey and recalculates statistics in ITX
	DeleteRecipientGroup(ctx context.Context, surveyID string, committeeID *string, projectID *string, foundationID *string) error

	// CreateExclusion creates a survey or global exclusion in ITX
	CreateExclusion(ctx context.Context, req *itx.ExclusionRequest) (*itx.Exclusion, error)

	// DeleteExclusion deletes a survey or global exclusion in ITX
	DeleteExclusion(ctx context.Context, req *itx.ExclusionRequest) error

	// GetExclusion retrieves an exclusion by ID from ITX
	GetExclusion(ctx context.Context, exclusionID string) (*itx.ExtendedExclusion, error)

	// DeleteExclusionByID deletes an exclusion by its ID from ITX
	DeleteExclusionByID(ctx context.Context, exclusionID string) error

	// GetSurveyResults retrieves aggregated survey results from ITX
	GetSurveyResults(ctx context.Context, surveyID string) (*itx.SurveyResults, error)

	// ValidateEmail validates email template body and subject in ITX
	ValidateEmail(ctx context.Context, req *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error)
}

// SurveyResponseClient defines the interface for survey response operations in ITX
type SurveyResponseClient interface {
	// CreateResponse submits a survey response in ITX
	CreateResponse(ctx context.Context, req *itx.CreateResponseRequest) error

	// GetResponse retrieves survey response details from ITX
	GetResponse(ctx context.Context, responseID string) (*itx.ResponseResponse, error)

	// UpdateResponse updates a survey response in ITX
	UpdateResponse(ctx context.Context, responseID string, req *itx.UpdateResponseRequest) error

	// DeleteResponse removes a recipient from survey and recalculates statistics in ITX
	DeleteResponse(ctx context.Context, surveyID string, responseID string) error

	// ResendResponse resends the survey email to a specific user in ITX
	ResendResponse(ctx context.Context, surveyID string, responseID string) error
}

// ITXProxyClient combines both survey and survey response operations
type ITXProxyClient interface {
	SurveyClient
	SurveyResponseClient
}
