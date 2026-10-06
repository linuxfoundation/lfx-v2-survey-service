// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"goa.design/goa/v3/security"
)

// SurveyAPI implements the survey.Service and survey.Auther interfaces
type SurveyAPI struct {
	surveyService *service.SurveyService
	auth          domain.Authenticator
}

// NewSurveyAPI creates a new SurveyAPI instance.
// auth is wired here rather than into SurveyService so that validation is
// concentrated at the single Goa JWTAuth seam instead of being sprayed across
// every service method.
func NewSurveyAPI(surveyService *service.SurveyService, auth domain.Authenticator) *SurveyAPI {
	return &SurveyAPI{
		surveyService: surveyService,
		auth:          auth,
	}
}

// mapDomainError translates a domain error into the appropriate Goa transport
// error type. It lives here — at the outermost adapter layer — because Goa
// error types are a transport concern; the service layer must not import them.
func mapDomainError(err error) error {
	var domainErr *domain.DomainError
	if !errors.As(err, &domainErr) {
		return &survey.InternalServerError{
			Code:    "500",
			Message: "Internal server error",
		}
	}
	switch domainErr.Type {
	case domain.ErrorTypeValidation:
		return &survey.BadRequestError{Code: "400", Message: domainErr.Message}
	case domain.ErrorTypeNotFound:
		return &survey.NotFoundError{Code: "404", Message: domainErr.Message}
	case domain.ErrorTypeConflict:
		return &survey.ConflictError{Code: "409", Message: domainErr.Message}
	case domain.ErrorTypeUnavailable:
		return &survey.ServiceUnavailableError{Code: "503", Message: domainErr.Message}
	default:
		return &survey.InternalServerError{Code: "500", Message: domainErr.Message}
	}
}

// ScheduleSurvey implements survey.Service.ScheduleSurvey
func (api *SurveyAPI) ScheduleSurvey(ctx context.Context, p *survey.ScheduleSurveyPayload) (*survey.SurveyScheduleResult, error) {
	result, err := api.surveyService.ScheduleSurvey(ctx, p)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return result, nil
}

// GetSurvey implements survey.Service.GetSurvey
func (api *SurveyAPI) GetSurvey(ctx context.Context, p *survey.GetSurveyPayload) (*survey.SurveyScheduleResult, error) {
	result, err := api.surveyService.GetSurvey(ctx, p)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return result, nil
}

// UpdateSurvey implements survey.Service.UpdateSurvey
func (api *SurveyAPI) UpdateSurvey(ctx context.Context, p *survey.UpdateSurveyPayload) (*survey.SurveyScheduleResult, error) {
	result, err := api.surveyService.UpdateSurvey(ctx, p)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return result, nil
}

// DeleteSurvey implements survey.Service.DeleteSurvey
func (api *SurveyAPI) DeleteSurvey(ctx context.Context, p *survey.DeleteSurveyPayload) error {
	if err := api.surveyService.DeleteSurvey(ctx, p); err != nil {
		return mapDomainError(err)
	}
	return nil
}

// BulkResendSurvey implements survey.Service.BulkResendSurvey
func (api *SurveyAPI) BulkResendSurvey(ctx context.Context, p *survey.BulkResendSurveyPayload) error {
	if err := api.surveyService.BulkResendSurvey(ctx, p); err != nil {
		return mapDomainError(err)
	}
	return nil
}

// PreviewSendSurvey implements survey.Service.PreviewSendSurvey
func (api *SurveyAPI) PreviewSendSurvey(ctx context.Context, p *survey.PreviewSendSurveyPayload) (*survey.PreviewSendResult, error) {
	result, err := api.surveyService.PreviewSendSurvey(ctx, p)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return result, nil
}

// SendMissingRecipients implements survey.Service.SendMissingRecipients
func (api *SurveyAPI) SendMissingRecipients(ctx context.Context, p *survey.SendMissingRecipientsPayload) error {
	if err := api.surveyService.SendMissingRecipients(ctx, p); err != nil {
		return mapDomainError(err)
	}
	return nil
}

// DeleteSurveyResponse implements survey.Service.DeleteSurveyResponse
func (api *SurveyAPI) DeleteSurveyResponse(ctx context.Context, p *survey.DeleteSurveyResponsePayload) error {
	if err := api.surveyService.DeleteSurveyResponse(ctx, p); err != nil {
		return mapDomainError(err)
	}
	return nil
}

// ResendSurveyResponse implements survey.Service.ResendSurveyResponse
func (api *SurveyAPI) ResendSurveyResponse(ctx context.Context, p *survey.ResendSurveyResponsePayload) error {
	if err := api.surveyService.ResendSurveyResponse(ctx, p); err != nil {
		return mapDomainError(err)
	}
	return nil
}

// DeleteRecipientGroup implements survey.Service.DeleteRecipientGroup
func (api *SurveyAPI) DeleteRecipientGroup(ctx context.Context, p *survey.DeleteRecipientGroupPayload) error {
	if err := api.surveyService.DeleteRecipientGroup(ctx, p); err != nil {
		return mapDomainError(err)
	}
	return nil
}

// CreateExclusion implements survey.Service.CreateExclusion
func (api *SurveyAPI) CreateExclusion(ctx context.Context, p *survey.CreateExclusionPayload) (*survey.ExclusionResult, error) {
	result, err := api.surveyService.CreateExclusion(ctx, p)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return result, nil
}

// DeleteExclusion implements survey.Service.DeleteExclusion
func (api *SurveyAPI) DeleteExclusion(ctx context.Context, p *survey.DeleteExclusionPayload) error {
	if err := api.surveyService.DeleteExclusion(ctx, p); err != nil {
		return mapDomainError(err)
	}
	return nil
}

// GetExclusion implements survey.Service.GetExclusion
func (api *SurveyAPI) GetExclusion(ctx context.Context, p *survey.GetExclusionPayload) (*survey.ExtendedExclusionResult, error) {
	result, err := api.surveyService.GetExclusion(ctx, p)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return result, nil
}

// DeleteExclusionByID implements survey.Service.DeleteExclusionByID
func (api *SurveyAPI) DeleteExclusionByID(ctx context.Context, p *survey.DeleteExclusionByIDPayload) error {
	if err := api.surveyService.DeleteExclusionByID(ctx, p); err != nil {
		return mapDomainError(err)
	}
	return nil
}

// ListSurveyResponses implements survey.Service.ListSurveyResponses
func (api *SurveyAPI) ListSurveyResponses(ctx context.Context, p *survey.ListSurveyResponsesPayload) (*survey.SurveyResponsesPage, error) {
	result, err := api.surveyService.ListSurveyResponses(ctx, p)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return result, nil
}

// ValidateEmail implements survey.Service.ValidateEmail
func (api *SurveyAPI) ValidateEmail(ctx context.Context, p *survey.ValidateEmailPayload) (*survey.ValidateEmailResult, error) {
	result, err := api.surveyService.ValidateEmail(ctx, p)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return result, nil
}

// JWTAuth implements survey.Auther.JWTAuth.
// It is the single authentication seam: validates the Heimdall-issued JWT,
// extracts the principal, and stores it in context so service methods can read
// it via ctx.Value(constants.PrincipalContextID) without needing an Authenticator
// dependency of their own.
func (api *SurveyAPI) JWTAuth(ctx context.Context, token string, _ *security.JWTScheme) (context.Context, error) {
	principal, err := api.auth.ParsePrincipal(ctx, token)
	if err != nil {
		return ctx, &survey.UnauthorizedError{
			Code:    "401",
			Message: "Unauthorized: " + err.Error(),
		}
	}
	return context.WithValue(ctx, constants.PrincipalContextID, principal), nil
}
