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
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return nil, err
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
		"survey_uid", result.UID,
		"survey_status", result.SurveyStatus,
	)

	return result, nil
}

// GetSurvey implements survey.Service.GetSurvey
func (s *SurveyService) GetSurvey(ctx context.Context, p *survey.GetSurveyPayload) (*survey.SurveyScheduleResult, error) {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "getting survey",
		"principal", principal,
		"survey_uid", p.SurveyUID,
	)

	// Call ITX API
	itxResponse, err := s.proxy.GetSurvey(ctx, p.SurveyUID)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := mapITXResponseToResult(itxResponse)

	s.logger.InfoContext(ctx, "survey retrieved successfully",
		"survey_uid", result.UID,
	)

	return result, nil
}

// UpdateSurvey implements survey.Service.UpdateSurvey
func (s *SurveyService) UpdateSurvey(ctx context.Context, p *survey.UpdateSurveyPayload) (*survey.SurveyScheduleResult, error) {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "updating survey",
		"principal", principal,
		"survey_uid", p.SurveyUID,
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
	itxResponse, err := s.proxy.UpdateSurvey(ctx, p.SurveyUID, itxRequest)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := mapITXResponseToResult(itxResponse)

	s.logger.InfoContext(ctx, "survey updated successfully",
		"survey_uid", result.UID,
	)

	return result, nil
}

// DeleteSurvey implements survey.Service.DeleteSurvey
func (s *SurveyService) DeleteSurvey(ctx context.Context, p *survey.DeleteSurveyPayload) error {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "deleting survey",
		"principal", principal,
		"survey_uid", p.SurveyUID,
	)

	// Call ITX API
	err = s.proxy.DeleteSurvey(ctx, p.SurveyUID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey deleted successfully",
		"survey_uid", p.SurveyUID,
	)

	return nil
}

// BulkResendSurvey implements survey.Service.BulkResendSurvey
func (s *SurveyService) BulkResendSurvey(ctx context.Context, p *survey.BulkResendSurveyPayload) error {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "bulk resending survey",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"recipient_count", len(p.RecipientIds),
	)

	// Build ITX request
	itxRequest := &itx.BulkResendRequest{
		RecipientIDs: p.RecipientIds,
	}

	// Call ITX API
	err = s.proxy.BulkResendSurvey(ctx, p.SurveyUID, itxRequest)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey bulk resend dispatched successfully",
		"survey_uid", p.SurveyUID,
	)

	return nil
}

// PreviewSendSurvey implements survey.Service.PreviewSendSurvey
func (s *SurveyService) PreviewSendSurvey(ctx context.Context, p *survey.PreviewSendSurveyPayload) (*survey.PreviewSendResult, error) {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "previewing survey send",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"committee_uid", p.CommitteeUID,
	)

	// Call ITX API
	itxResponse, err := s.proxy.PreviewSend(ctx, p.SurveyUID, p.CommitteeUID)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := mapPreviewSendResponseToResult(itxResponse)

	s.logger.InfoContext(ctx, "survey preview send retrieved successfully",
		"survey_uid", p.SurveyUID,
		"affected_recipients", len(result.AffectedRecipients),
	)

	return result, nil
}

// SendMissingRecipients implements survey.Service.SendMissingRecipients
func (s *SurveyService) SendMissingRecipients(ctx context.Context, p *survey.SendMissingRecipientsPayload) error {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "sending survey to missing recipients",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"committee_uid", p.CommitteeUID,
	)

	// Call ITX API
	err = s.proxy.SendMissingRecipients(ctx, p.SurveyUID, p.CommitteeUID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey send to missing recipients dispatched successfully",
		"survey_uid", p.SurveyUID,
	)

	return nil
}

// DeleteSurveyResponse removes a recipient from survey and recalculates statistics
func (s *SurveyService) DeleteSurveyResponse(ctx context.Context, p *survey.DeleteSurveyResponsePayload) error {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "deleting survey response",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"response_id", p.ResponseID,
	)

	// Call ITX API
	err = s.proxy.DeleteResponse(ctx, p.SurveyUID, p.ResponseID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey response deleted successfully",
		"survey_uid", p.SurveyUID,
		"response_id", p.ResponseID,
	)

	return nil
}

// ResendSurveyResponse resends the survey email to a specific user
func (s *SurveyService) ResendSurveyResponse(ctx context.Context, p *survey.ResendSurveyResponsePayload) error {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "resending survey response",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"response_id", p.ResponseID,
	)

	// Call ITX API
	err = s.proxy.ResendResponse(ctx, p.SurveyUID, p.ResponseID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey response resent successfully",
		"survey_uid", p.SurveyUID,
		"response_id", p.ResponseID,
	)

	return nil
}

// DeleteRecipientGroup removes a recipient group from survey and recalculates statistics
func (s *SurveyService) DeleteRecipientGroup(ctx context.Context, p *survey.DeleteRecipientGroupPayload) error {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "deleting recipient group from survey",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"committee_uid", p.CommitteeUID,
		"project_uid", p.ProjectUID,
		"foundation_id", p.FoundationID,
	)

	// Call ITX API
	err = s.proxy.DeleteRecipientGroup(ctx, p.SurveyUID, p.CommitteeUID, p.ProjectUID, p.FoundationID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "recipient group deleted successfully",
		"survey_uid", p.SurveyUID,
	)

	return nil
}

// CreateExclusion creates a survey or global exclusion
func (s *SurveyService) CreateExclusion(ctx context.Context, p *survey.CreateExclusionPayload) (*survey.ExclusionResult, error) {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "creating exclusion",
		"principal", principal,
		"email", p.Email,
		"user_id", p.UserID,
	)

	// Build ITX request
	itxRequest := &itx.ExclusionRequest{
		Email:           p.Email,
		UserID:          p.UserID,
		SurveyID:        p.SurveyUID,
		CommitteeID:     p.CommitteeUID,
		GlobalExclusion: p.GlobalExclusion,
	}

	// Call ITX API
	itxResponse, err := s.proxy.CreateExclusion(ctx, itxRequest)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := mapExclusionToResult(itxResponse)

	s.logger.InfoContext(ctx, "exclusion created successfully",
		"exclusion_uid", result.UID,
	)

	return result, nil
}

// DeleteExclusion deletes a survey or global exclusion
func (s *SurveyService) DeleteExclusion(ctx context.Context, p *survey.DeleteExclusionPayload) error {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "deleting exclusion",
		"principal", principal,
		"email", p.Email,
		"user_id", p.UserID,
	)

	// Build ITX request
	itxRequest := &itx.ExclusionRequest{
		Email:           p.Email,
		UserID:          p.UserID,
		SurveyID:        p.SurveyUID,
		CommitteeID:     p.CommitteeUID,
		GlobalExclusion: p.GlobalExclusion,
	}

	// Call ITX API
	err = s.proxy.DeleteExclusion(ctx, itxRequest)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "exclusion deleted successfully")

	return nil
}

// GetExclusion retrieves an exclusion by ID
func (s *SurveyService) GetExclusion(ctx context.Context, p *survey.GetExclusionPayload) (*survey.ExtendedExclusionResult, error) {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "getting exclusion",
		"principal", principal,
		"exclusion_id", p.ExclusionID,
	)

	// Call ITX API
	itxResponse, err := s.proxy.GetExclusion(ctx, p.ExclusionID)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := mapExtendedExclusionToResult(itxResponse)

	s.logger.InfoContext(ctx, "exclusion retrieved successfully",
		"exclusion_uid", result.UID,
	)

	return result, nil
}

// DeleteExclusionByID deletes an exclusion by its ID
func (s *SurveyService) DeleteExclusionByID(ctx context.Context, p *survey.DeleteExclusionByIDPayload) error {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "deleting exclusion by ID",
		"principal", principal,
		"exclusion_id", p.ExclusionID,
	)

	// Call ITX API
	err = s.proxy.DeleteExclusionByID(ctx, p.ExclusionID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "exclusion deleted successfully",
		"exclusion_id", p.ExclusionID,
	)

	return nil
}

// ValidateEmail validates email template body and subject
func (s *SurveyService) ValidateEmail(ctx context.Context, p *survey.ValidateEmailPayload) (*survey.ValidateEmailResult, error) {
	// Parse JWT token to get principal
	principal, err := s.parsePrincipal(ctx, p.Token)
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "validating email template",
		"principal", principal,
	)

	// Build ITX request
	itxRequest := &itx.ValidateEmailRequest{
		Body:    p.Body,
		Subject: p.Subject,
	}

	// Call ITX API
	itxResponse, err := s.proxy.ValidateEmail(ctx, itxRequest)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result
	result := &survey.ValidateEmailResult{
		Body:    itxResponse.Body,
		Subject: itxResponse.Subject,
	}

	s.logger.InfoContext(ctx, "email template validated successfully")

	return result, nil
}

// Helper functions

// parsePrincipal extracts and validates the JWT token, returning the principal
func (s *SurveyService) parsePrincipal(ctx context.Context, token *string) (string, error) {
	t := ""
	if token != nil {
		t = *token
	}
	principal, err := s.auth.ParsePrincipal(ctx, t, s.logger)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse JWT", "error", err)
		return "", &survey.UnauthorizedError{
			Code:    "401",
			Message: "Unauthorized: " + err.Error(),
		}
	}
	return principal, nil
}

// mapITXResponseToResult maps ITX response to Goa result (extracted to avoid duplication)
func mapITXResponseToResult(itxResponse *itx.SurveyScheduleResponse) *survey.SurveyScheduleResult {
	return &survey.SurveyScheduleResult{
		UID:                           itxResponse.ID,
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
			CommitteeUID:    c.CommitteeID,
			ProjectUID:      c.ProjectID,
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
			ProjectUID:        c.ProjectID,
			ProjectName:       c.ProjectName,
			CommitteeUID:      c.CommitteeID,
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

func mapExclusionToResult(itxExclusion *itx.Exclusion) *survey.ExclusionResult {
	return &survey.ExclusionResult{
		UID:             itxExclusion.ID,
		Email:           itxExclusion.Email,
		SurveyUID:       itxExclusion.SurveyID,
		CommitteeUID:    itxExclusion.CommitteeID,
		GlobalExclusion: itxExclusion.GlobalExclusion,
		UserID:          itxExclusion.UserID,
	}
}

func mapExtendedExclusionToResult(itxExclusion *itx.ExtendedExclusion) *survey.ExtendedExclusionResult {
	result := &survey.ExtendedExclusionResult{
		UID:             itxExclusion.ID,
		Email:           itxExclusion.Email,
		SurveyUID:       itxExclusion.SurveyID,
		CommitteeUID:    itxExclusion.CommitteeID,
		GlobalExclusion: itxExclusion.GlobalExclusion,
		UserID:          itxExclusion.UserID,
	}

	if itxExclusion.User != nil {
		emails := make([]*survey.UserEmail, 0, len(itxExclusion.User.Emails))
		for _, e := range itxExclusion.User.Emails {
			emails = append(emails, &survey.UserEmail{
				ID:           e.ID,
				EmailAddress: e.EmailAddress,
				IsPrimary:    e.IsPrimary,
			})
		}

		result.User = &survey.ExclusionUser{
			ID:       itxExclusion.User.ID,
			Username: itxExclusion.User.Username,
			Emails:   emails,
		}
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
