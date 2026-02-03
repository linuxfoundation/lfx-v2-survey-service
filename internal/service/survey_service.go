// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

type SurveyService struct {
	auth     domain.Authenticator
	proxy    domain.ITXProxyClient
	idMapper domain.IDMapper
	logger   *slog.Logger
}

func NewSurveyService(
	auth domain.Authenticator,
	proxy domain.ITXProxyClient,
	idMapper domain.IDMapper,
	logger *slog.Logger,
) *SurveyService {
	return &SurveyService{
		auth:     auth,
		proxy:    proxy,
		idMapper: idMapper,
		logger:   logger,
	}
}

// ScheduleSurvey implements survey.Service.ScheduleSurvey
func (s *SurveyService) ScheduleSurvey(ctx context.Context, p *survey.ScheduleSurveyPayload) (*survey.SurveyScheduleResult, error) {
	// Parse JWT token to get principal (or use mock principal if configured)
	token := ""
	if p.Token != nil {
		token = *p.Token
	}
	principal, err := s.auth.ParsePrincipal(ctx, token, s.logger)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse JWT", "error", err)
		return nil, &survey.UnauthorizedError{
			Code:    "401",
			Message: "Unauthorized: " + err.Error(),
		}
	}

	s.logger.InfoContext(ctx, "scheduling survey",
		"principal", principal,
		"survey_title", p.SurveyTitle,
		"survey_monkey_id", p.SurveyMonkeyID,
		"send_immediately", p.SendImmediately,
	)

	// Build ITX request - map goa payload directly to ITX structure
	itxRequest := &itx.ScheduleSurveyRequest{
		IsProjectSurvey:        p.IsProjectSurvey,
		StageFilter:            p.StageFilter,
		CreatorUsername:        p.CreatorUsername,
		CreatorName:            p.CreatorName,
		CreatorID:              p.CreatorID,
		SurveyMonkeyID:         p.SurveyMonkeyID,
		SurveyTitle:            p.SurveyTitle,
		SendImmediately:        p.SendImmediately,
		SurveySendDate:         p.SurveySendDate,
		SurveyCutoffDate:       p.SurveyCutoffDate,
		SurveyReminderRateDays: p.SurveyReminderRateDays,
		EmailSubject:           p.EmailSubject,
		EmailBody:              p.EmailBody,
		EmailBodyText:          p.EmailBodyText,
		Committees:             p.Committees,
		CommitteeVotingEnabled: p.CommitteeVotingEnabled,
	}

	// Call ITX API
	itxResponse, err := s.proxy.ScheduleSurvey(ctx, itxRequest)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := mapITXResponseToResult(itxResponse)

	s.logger.InfoContext(ctx, "survey scheduled successfully",
		"survey_id", result.ID,
		"survey_status", result.SurveyStatus,
	)

	return result, nil
}

// GetSurvey implements survey.Service.GetSurvey
func (s *SurveyService) GetSurvey(ctx context.Context, p *survey.GetSurveyPayload) (*survey.SurveyScheduleResult, error) {
	// Parse JWT token to get principal
	token := ""
	if p.Token != nil {
		token = *p.Token
	}
	principal, err := s.auth.ParsePrincipal(ctx, token, s.logger)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse JWT", "error", err)
		return nil, &survey.UnauthorizedError{
			Code:    "401",
			Message: "Unauthorized: " + err.Error(),
		}
	}

	s.logger.InfoContext(ctx, "getting survey",
		"principal", principal,
		"survey_id", p.SurveyID,
	)

	// Call ITX API
	itxResponse, err := s.proxy.GetSurvey(ctx, p.SurveyID)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := mapITXResponseToResult(itxResponse)

	s.logger.InfoContext(ctx, "survey retrieved successfully",
		"survey_id", result.ID,
	)

	return result, nil
}

// UpdateSurvey implements survey.Service.UpdateSurvey
func (s *SurveyService) UpdateSurvey(ctx context.Context, p *survey.UpdateSurveyPayload) (*survey.SurveyScheduleResult, error) {
	// Parse JWT token to get principal
	token := ""
	if p.Token != nil {
		token = *p.Token
	}
	principal, err := s.auth.ParsePrincipal(ctx, token, s.logger)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse JWT", "error", err)
		return nil, &survey.UnauthorizedError{
			Code:    "401",
			Message: "Unauthorized: " + err.Error(),
		}
	}

	s.logger.InfoContext(ctx, "updating survey",
		"principal", principal,
		"survey_id", p.SurveyID,
		"survey_title", p.SurveyTitle,
	)

	// Build ITX request
	itxRequest := &itx.UpdateSurveyRequest{
		CreatorID:              p.CreatorID,
		SurveyTitle:            p.SurveyTitle,
		SurveySendDate:         p.SurveySendDate,
		SurveyCutoffDate:       p.SurveyCutoffDate,
		SurveyReminderRateDays: p.SurveyReminderRateDays,
		EmailSubject:           p.EmailSubject,
		EmailBody:              p.EmailBody,
		EmailBodyText:          p.EmailBodyText,
		Committees:             p.Committees,
		CommitteeVotingEnabled: p.CommitteeVotingEnabled,
	}

	// Call ITX API
	itxResponse, err := s.proxy.UpdateSurvey(ctx, p.SurveyID, itxRequest)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := mapITXResponseToResult(itxResponse)

	s.logger.InfoContext(ctx, "survey updated successfully",
		"survey_id", result.ID,
	)

	return result, nil
}

// DeleteSurvey implements survey.Service.DeleteSurvey
func (s *SurveyService) DeleteSurvey(ctx context.Context, p *survey.DeleteSurveyPayload) error {
	// Parse JWT token to get principal
	token := ""
	if p.Token != nil {
		token = *p.Token
	}
	principal, err := s.auth.ParsePrincipal(ctx, token, s.logger)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse JWT", "error", err)
		return &survey.UnauthorizedError{
			Code:    "401",
			Message: "Unauthorized: " + err.Error(),
		}
	}

	s.logger.InfoContext(ctx, "deleting survey",
		"principal", principal,
		"survey_id", p.SurveyID,
	)

	// Call ITX API
	err = s.proxy.DeleteSurvey(ctx, p.SurveyID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey deleted successfully",
		"survey_id", p.SurveyID,
	)

	return nil
}

// BulkResendSurvey implements survey.Service.BulkResendSurvey
func (s *SurveyService) BulkResendSurvey(ctx context.Context, p *survey.BulkResendSurveyPayload) error {
	// Parse JWT token to get principal
	token := ""
	if p.Token != nil {
		token = *p.Token
	}
	principal, err := s.auth.ParsePrincipal(ctx, token, s.logger)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse JWT", "error", err)
		return &survey.UnauthorizedError{
			Code:    "401",
			Message: "Unauthorized: " + err.Error(),
		}
	}

	s.logger.InfoContext(ctx, "bulk resending survey",
		"principal", principal,
		"survey_id", p.SurveyID,
		"recipient_count", len(p.RecipientIds),
	)

	// Build ITX request
	itxRequest := &itx.BulkResendRequest{
		RecipientIDs: p.RecipientIds,
	}

	// Call ITX API
	err = s.proxy.BulkResendSurvey(ctx, p.SurveyID, itxRequest)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey bulk resend dispatched successfully",
		"survey_id", p.SurveyID,
	)

	return nil
}

// PreviewSendSurvey implements survey.Service.PreviewSendSurvey
func (s *SurveyService) PreviewSendSurvey(ctx context.Context, p *survey.PreviewSendSurveyPayload) (*survey.PreviewSendResult, error) {
	// Parse JWT token to get principal
	token := ""
	if p.Token != nil {
		token = *p.Token
	}
	principal, err := s.auth.ParsePrincipal(ctx, token, s.logger)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse JWT", "error", err)
		return nil, &survey.UnauthorizedError{
			Code:    "401",
			Message: "Unauthorized: " + err.Error(),
		}
	}

	s.logger.InfoContext(ctx, "previewing survey send",
		"principal", principal,
		"survey_id", p.SurveyID,
		"committee_id", p.CommitteeID,
	)

	// Call ITX API
	itxResponse, err := s.proxy.PreviewSend(ctx, p.SurveyID, p.CommitteeID)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := mapPreviewSendResponseToResult(itxResponse)

	s.logger.InfoContext(ctx, "survey preview send retrieved successfully",
		"survey_id", p.SurveyID,
		"affected_recipients", len(result.AffectedRecipients),
	)

	return result, nil
}

// SendMissingRecipients implements survey.Service.SendMissingRecipients
func (s *SurveyService) SendMissingRecipients(ctx context.Context, p *survey.SendMissingRecipientsPayload) error {
	// Parse JWT token to get principal
	token := ""
	if p.Token != nil {
		token = *p.Token
	}
	principal, err := s.auth.ParsePrincipal(ctx, token, s.logger)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse JWT", "error", err)
		return &survey.UnauthorizedError{
			Code:    "401",
			Message: "Unauthorized: " + err.Error(),
		}
	}

	s.logger.InfoContext(ctx, "sending survey to missing recipients",
		"principal", principal,
		"survey_id", p.SurveyID,
		"committee_id", p.CommitteeID,
	)

	// Call ITX API
	err = s.proxy.SendMissingRecipients(ctx, p.SurveyID, p.CommitteeID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey send to missing recipients dispatched successfully",
		"survey_id", p.SurveyID,
	)

	return nil
}

// Helper functions

// mapITXResponseToResult maps ITX response to Goa result (extracted to avoid duplication)
func mapITXResponseToResult(itxResponse *itx.SurveyScheduleResponse) *survey.SurveyScheduleResult {
	return &survey.SurveyScheduleResult{
		ID:                            itxResponse.ID,
		SurveyMonkeyID:                itxResponse.SurveyMonkeyID,
		IsProjectSurvey:               itxResponse.IsProjectSurvey,
		StageFilter:                   itxResponse.StageFilter,
		CreatorUsername:               itxResponse.CreatorUsername,
		CreatorName:                   itxResponse.CreatorName,
		CreatorID:                     itxResponse.CreatorID,
		CreatedAt:                     itxResponse.CreatedAt,
		LastModifiedAt:                itxResponse.LastModifiedAt,
		LastModifiedBy:                itxResponse.LastModifiedBy,
		SurveyTitle:                   itxResponse.SurveyTitle,
		SurveyStatus:                  itxResponse.SurveyStatus,
		ResponseStatus:                itxResponse.ResponseStatus,
		SurveySendDate:                itxResponse.SurveySendDate,
		SurveyCutoffDate:              itxResponse.SurveyCutoffDate,
		SurveyReminderRateDays:        itxResponse.SurveyReminderRateDays,
		EmailSubject:                  itxResponse.EmailSubject,
		EmailBody:                     itxResponse.EmailBody,
		EmailBodyText:                 itxResponse.EmailBodyText,
		CommitteeCategory:             itxResponse.CommitteeCategory,
		Committees:                    mapSurveyCommitteesToResult(itxResponse.Committees),
		CommitteeVotingEnabled:        itxResponse.CommitteeVotingEnabled,
		SurveyURL:                     itxResponse.SurveyURL,
		SendImmediately:               itxResponse.SendImmediately,
		TotalRecipients:               itxResponse.TotalRecipients,
		TotalResponses:                itxResponse.TotalResponses,
		IsNpsSurvey:                   itxResponse.IsNPSSurvey,
		NpsValue:                      itxResponse.NPSValue,
		NumPromoters:                  itxResponse.NumPromoters,
		NumPassives:                   itxResponse.NumPassives,
		NumDetractors:                 itxResponse.NumDetractors,
		TotalBouncedEmails:            itxResponse.TotalBouncedEmails,
		NumAutomatedRemindersToSend:   itxResponse.NumAutomatedRemindersToSend,
		NumAutomatedRemindersSent:     itxResponse.NumAutomatedRemindersSent,
		NextAutomatedReminderAt:       itxResponse.NextAutomatedReminderAt,
		LatestAutomatedReminderSentAt: itxResponse.LatestAutomatedReminderSentAt,
	}
}

func mapSurveyCommitteesToResult(committees []itx.SurveyCommittee) []*survey.SurveyCommittee {
	if committees == nil {
		return nil
	}

	result := make([]*survey.SurveyCommittee, len(committees))
	for i, c := range committees {
		result[i] = &survey.SurveyCommittee{
			CommitteeName:   c.CommitteeName,
			CommitteeID:     c.CommitteeID,
			ProjectID:       c.ProjectID,
			ProjectName:     c.ProjectName,
			SurveyURL:       c.SurveyURL,
			TotalRecipients: c.TotalRecipients,
			TotalResponses:  c.TotalResponses,
			NpsValue:        c.NPSValue,
		}
	}
	return result
}

func mapPreviewSendResponseToResult(itxResponse *itx.PreviewSendResponse) *survey.PreviewSendResult {
	return &survey.PreviewSendResult{
		AffectedProjects:    mapLFXProjectsToResult(itxResponse.AffectedProjects),
		AffectedCommittees:  mapExcludedCommitteesToResult(itxResponse.AffectedCommittees),
		AffectedRecipients:  mapITXPreviewRecipientsToResult(itxResponse.AffectedRecipients),
	}
}

func mapLFXProjectsToResult(projects []itx.LFXProject) []*survey.LFXProject {
	// Always return an empty slice instead of nil to ensure JSON marshals as []
	result := make([]*survey.LFXProject, 0, len(projects))
	for _, p := range projects {
		result = append(result, &survey.LFXProject{
			ID:      p.ID,
			Name:    p.Name,
			Slug:    p.Slug,
			Status:  p.Status,
			LogoURL: p.LogoURL,
		})
	}
	return result
}

func mapExcludedCommitteesToResult(committees []itx.ExcludedCommittee) []*survey.ExcludedCommittee {
	// Always return an empty slice instead of nil to ensure JSON marshals as []
	result := make([]*survey.ExcludedCommittee, 0, len(committees))
	for _, c := range committees {
		result = append(result, &survey.ExcludedCommittee{
			ProjectID:         c.ProjectID,
			ProjectName:       c.ProjectName,
			CommitteeID:       c.CommitteeID,
			CommitteeName:     c.CommitteeName,
			CommitteeCategory: c.CommitteeCategory,
		})
	}
	return result
}

func mapITXPreviewRecipientsToResult(recipients []itx.ITXPreviewRecipient) []*survey.ITXPreviewRecipient {
	// Always return an empty slice instead of nil to ensure JSON marshals as []
	result := make([]*survey.ITXPreviewRecipient, 0, len(recipients))
	for _, r := range recipients {
		result = append(result, &survey.ITXPreviewRecipient{
			UserID:    r.UserID,
			Name:      r.Name,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			Username:  r.Username,
			Email:     r.Email,
			Role:      r.Role,
		})
	}
	return result
}

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
		return &survey.BadRequestError{
			Code:    "400",
			Message: domainErr.Message,
		}
	case domain.ErrorTypeNotFound:
		return &survey.NotFoundError{
			Code:    "404",
			Message: domainErr.Message,
		}
	case domain.ErrorTypeConflict:
		return &survey.ConflictError{
			Code:    "409",
			Message: domainErr.Message,
		}
	case domain.ErrorTypeUnavailable:
		return &survey.ServiceUnavailableError{
			Code:    "503",
			Message: domainErr.Message,
		}
	default:
		return &survey.InternalServerError{
			Code:    "500",
			Message: domainErr.Message,
		}
	}
}
