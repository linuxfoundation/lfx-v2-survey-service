// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"goa.design/goa/v3/security"
)

// SurveyAPI implements the survey.Service and survey.Auther interfaces
type SurveyAPI struct {
	surveyService *service.SurveyService
}

// NewSurveyAPI creates a new SurveyAPI instance
func NewSurveyAPI(surveyService *service.SurveyService) *SurveyAPI {
	return &SurveyAPI{
		surveyService: surveyService,
	}
}

// ScheduleSurvey implements survey.Service.ScheduleSurvey
func (api *SurveyAPI) ScheduleSurvey(ctx context.Context, p *survey.ScheduleSurveyPayload) (*survey.SurveyScheduleResult, error) {
	return api.surveyService.ScheduleSurvey(ctx, p)
}

// GetSurvey implements survey.Service.GetSurvey
func (api *SurveyAPI) GetSurvey(ctx context.Context, p *survey.GetSurveyPayload) (*survey.SurveyScheduleResult, error) {
	return api.surveyService.GetSurvey(ctx, p)
}

// UpdateSurvey implements survey.Service.UpdateSurvey
func (api *SurveyAPI) UpdateSurvey(ctx context.Context, p *survey.UpdateSurveyPayload) (*survey.SurveyScheduleResult, error) {
	return api.surveyService.UpdateSurvey(ctx, p)
}

// DeleteSurvey implements survey.Service.DeleteSurvey
func (api *SurveyAPI) DeleteSurvey(ctx context.Context, p *survey.DeleteSurveyPayload) error {
	return api.surveyService.DeleteSurvey(ctx, p)
}

// BulkResendSurvey implements survey.Service.BulkResendSurvey
func (api *SurveyAPI) BulkResendSurvey(ctx context.Context, p *survey.BulkResendSurveyPayload) error {
	return api.surveyService.BulkResendSurvey(ctx, p)
}

// PreviewSendSurvey implements survey.Service.PreviewSendSurvey
func (api *SurveyAPI) PreviewSendSurvey(ctx context.Context, p *survey.PreviewSendSurveyPayload) (*survey.PreviewSendResult, error) {
	return api.surveyService.PreviewSendSurvey(ctx, p)
}

// SendMissingRecipients implements survey.Service.SendMissingRecipients
func (api *SurveyAPI) SendMissingRecipients(ctx context.Context, p *survey.SendMissingRecipientsPayload) error {
	return api.surveyService.SendMissingRecipients(ctx, p)
}

// DeleteSurveyResponse implements survey.Service.DeleteSurveyResponse
func (api *SurveyAPI) DeleteSurveyResponse(ctx context.Context, p *survey.DeleteSurveyResponsePayload) error {
	return api.surveyService.DeleteSurveyResponse(ctx, p)
}

// ResendSurveyResponse implements survey.Service.ResendSurveyResponse
func (api *SurveyAPI) ResendSurveyResponse(ctx context.Context, p *survey.ResendSurveyResponsePayload) error {
	return api.surveyService.ResendSurveyResponse(ctx, p)
}

// DeleteRecipientGroup implements survey.Service.DeleteRecipientGroup
func (api *SurveyAPI) DeleteRecipientGroup(ctx context.Context, p *survey.DeleteRecipientGroupPayload) error {
	return api.surveyService.DeleteRecipientGroup(ctx, p)
}

// CreateExclusion implements survey.Service.CreateExclusion
func (api *SurveyAPI) CreateExclusion(ctx context.Context, p *survey.CreateExclusionPayload) (*survey.ExclusionResult, error) {
	return api.surveyService.CreateExclusion(ctx, p)
}

// DeleteExclusion implements survey.Service.DeleteExclusion
func (api *SurveyAPI) DeleteExclusion(ctx context.Context, p *survey.DeleteExclusionPayload) error {
	return api.surveyService.DeleteExclusion(ctx, p)
}

// GetExclusion implements survey.Service.GetExclusion
func (api *SurveyAPI) GetExclusion(ctx context.Context, p *survey.GetExclusionPayload) (*survey.ExtendedExclusionResult, error) {
	return api.surveyService.GetExclusion(ctx, p)
}

// DeleteExclusionByID implements survey.Service.DeleteExclusionByID
func (api *SurveyAPI) DeleteExclusionByID(ctx context.Context, p *survey.DeleteExclusionByIDPayload) error {
	return api.surveyService.DeleteExclusionByID(ctx, p)
}

// ValidateEmail implements survey.Service.ValidateEmail
func (api *SurveyAPI) ValidateEmail(ctx context.Context, p *survey.ValidateEmailPayload) (*survey.ValidateEmailResult, error) {
	return api.surveyService.ValidateEmail(ctx, p)
}

// JWTAuth implements survey.Auther.JWTAuth
// This is called by goa to validate JWT tokens before calling service methods
func (api *SurveyAPI) JWTAuth(ctx context.Context, token string, scheme *security.JWTScheme) (context.Context, error) {
	// The actual JWT validation is performed in the service layer
	// Here we just pass the context through since goa needs this method to exist
	return ctx, nil
}
