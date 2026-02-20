// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/concurrent"
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
		"committee_uid", p.CommitteeUID,
	)

	// Map committee UID from V2 to V1 (ITX expects V1 SFID)
	committeeV1, err := s.idMapper.MapCommitteeV2ToV1(ctx, p.CommitteeUID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map committee UID to V1",
			"committee_uid", p.CommitteeUID,
			"error", err,
		)
		return nil, mapDomainError(err)
	}

	s.logger.DebugContext(ctx, "mapped committee ID",
		"committee_v2_uid", p.CommitteeUID,
		"committee_v1_sfid", committeeV1,
	)

	// Build ITX request - convert single committee_uid to array for ITX
	committees := []string{committeeV1}

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
		Committees:             committees,
		CommitteeVotingEnabled: p.CommitteeVotingEnabled,
	}

	// Call ITX API
	itxResponse, err := s.proxy.ScheduleSurvey(ctx, itxRequest)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result (including V1 to V2 ID mapping)
	result, err := s.mapITXResponseToResult(ctx, itxResponse)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map ITX response",
			"error", err,
		)
		return nil, mapDomainError(err)
	}

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

	// Map response back to goa result (including V1 to V2 ID mapping)
	result, err := s.mapITXResponseToResult(ctx, itxResponse)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map ITX response",
			"error", err,
		)
		return nil, mapDomainError(err)
	}

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
		"committee_uid", p.CommitteeUID,
	)

	// Build ITX request - convert single committee_uid to array for ITX
	// Map committee UID from V2 to V1 if provided
	var committees []string
	if p.CommitteeUID != nil && *p.CommitteeUID != "" {
		committeeV1, err := s.mapOptionalCommitteeV2ToV1(ctx, p.CommitteeUID)
		if err != nil {
			return nil, mapDomainError(err)
		}
		if committeeV1 != nil {
			committees = []string{*committeeV1}
		}
	}

	itxRequest := &itx.UpdateSurveyRequest{
		CreatorID:              p.CreatorID,
		SurveyTitle:            p.SurveyTitle,
		SurveySendDate:         p.SurveySendDate,
		SurveyCutoffDate:       p.SurveyCutoffDate,
		SurveyReminderRateDays: p.SurveyReminderRateDays,
		EmailSubject:           p.EmailSubject,
		EmailBody:              p.EmailBody,
		EmailBodyText:          p.EmailBodyText,
		Committees:             committees,
		CommitteeVotingEnabled: p.CommitteeVotingEnabled,
	}

	// Call ITX API
	itxResponse, err := s.proxy.UpdateSurvey(ctx, p.SurveyUID, itxRequest)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result (including V1 to V2 ID mapping)
	result, err := s.mapITXResponseToResult(ctx, itxResponse)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map ITX response",
			"error", err,
		)
		return nil, mapDomainError(err)
	}

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

	// Map committee UID from V2 to V1 if provided (ITX expects V1 SFID)
	committeeV1, err := s.mapOptionalCommitteeV2ToV1(ctx, p.CommitteeUID)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Call ITX API
	itxResponse, err := s.proxy.PreviewSend(ctx, p.SurveyUID, committeeV1)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result (including V1 to V2 ID mapping)
	result, err := s.mapPreviewSendResponseToResult(ctx, itxResponse)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map preview send response",
			"error", err,
		)
		return nil, mapDomainError(err)
	}

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

	// Map committee UID from V2 to V1 if provided (ITX expects V1 SFID)
	committeeV1, err := s.mapOptionalCommitteeV2ToV1(ctx, p.CommitteeUID)
	if err != nil {
		return mapDomainError(err)
	}

	// Call ITX API
	err = s.proxy.SendMissingRecipients(ctx, p.SurveyUID, committeeV1)
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

	// Map committee UID from V2 to V1 if provided (ITX expects V1 SFID)
	committeeV1, err := s.mapOptionalCommitteeV2ToV1(ctx, p.CommitteeUID)
	if err != nil {
		return mapDomainError(err)
	}

	// Map project UID from V2 to V1 if provided (ITX expects V1 SFID)
	projectV1, err := s.mapOptionalProjectV2ToV1(ctx, p.ProjectUID)
	if err != nil {
		return mapDomainError(err)
	}

	// Call ITX API
	err = s.proxy.DeleteRecipientGroup(ctx, p.SurveyUID, committeeV1, projectV1, p.FoundationID)
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

	// Map committee UID from V2 to V1 if provided (ITX expects V1 SFID)
	committeeV1, err := s.mapOptionalCommitteeV2ToV1(ctx, p.CommitteeUID)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Build ITX request
	itxRequest := &itx.ExclusionRequest{
		Email:           p.Email,
		UserID:          p.UserID,
		SurveyID:        p.SurveyUID,
		CommitteeID:     committeeV1,
		GlobalExclusion: p.GlobalExclusion,
	}

	// Call ITX API
	itxResponse, err := s.proxy.CreateExclusion(ctx, itxRequest)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to goa result (including V1 to V2 ID mapping)
	result, err := s.mapExclusionToResult(ctx, itxResponse)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map exclusion response",
			"error", err,
		)
		return nil, mapDomainError(err)
	}

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

	// Map committee UID from V2 to V1 if provided (ITX expects V1 SFID)
	committeeV1, err := s.mapOptionalCommitteeV2ToV1(ctx, p.CommitteeUID)
	if err != nil {
		return mapDomainError(err)
	}

	// Build ITX request
	itxRequest := &itx.ExclusionRequest{
		Email:           p.Email,
		UserID:          p.UserID,
		SurveyID:        p.SurveyUID,
		CommitteeID:     committeeV1,
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

	// Map response back to goa result (including V1 to V2 ID mapping)
	result, err := s.mapExtendedExclusionToResult(ctx, itxResponse)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map extended exclusion response",
			"error", err,
		)
		return nil, mapDomainError(err)
	}

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

// mapOptionalCommitteeV2ToV1 maps an optional committee UID from V2 to V1 with logging
func (s *SurveyService) mapOptionalCommitteeV2ToV1(ctx context.Context, committeeUID *string) (*string, error) {
	if committeeUID == nil || *committeeUID == "" {
		return nil, nil
	}

	mapped, err := s.idMapper.MapCommitteeV2ToV1(ctx, *committeeUID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map committee UID to V1",
			"committee_uid", *committeeUID,
			"error", err,
		)
		return nil, err
	}

	s.logger.DebugContext(ctx, "mapped committee ID",
		"committee_v2_uid", *committeeUID,
		"committee_v1_sfid", mapped,
	)

	return &mapped, nil
}

// mapOptionalProjectV2ToV1 maps an optional project UID from V2 to V1 with logging
func (s *SurveyService) mapOptionalProjectV2ToV1(ctx context.Context, projectUID *string) (*string, error) {
	if projectUID == nil || *projectUID == "" {
		return nil, nil
	}

	mapped, err := s.idMapper.MapProjectV2ToV1(ctx, *projectUID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map project UID to V1",
			"project_uid", *projectUID,
			"error", err,
		)
		return nil, err
	}

	s.logger.DebugContext(ctx, "mapped project ID",
		"project_v2_uid", *projectUID,
		"project_v1_sfid", mapped,
	)

	return &mapped, nil
}

// mapITXResponseToResult maps ITX response to Goa result with V1→V2 ID mapping
func (s *SurveyService) mapITXResponseToResult(ctx context.Context, itxResponse *itx.SurveyScheduleResponse) (*survey.SurveyScheduleResult, error) {
	// Map committees from V1 to V2
	committees, err := s.mapSurveyCommitteesToResult(ctx, itxResponse.Committees)
	if err != nil {
		return nil, err
	}

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
		Committees:                    committees,
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
	}, nil
}

func (s *SurveyService) mapSurveyCommitteesToResult(ctx context.Context, committees []itx.SurveyCommittee) ([]*survey.SurveyCommittee, error) {
	if committees == nil {
		return nil, nil
	}

	result := make([]*survey.SurveyCommittee, len(committees))

	// Create worker pool with 5 workers
	pool := concurrent.NewWorkerPool(5)

	// Build mapping functions for each committee
	mappingFunctions := make([]func() error, len(committees))
	for i, c := range committees {
		mappingFunctions[i] = func() error {
			// Map committee ID from V1 to V2 if present
			var committeeV2 *string
			if c.CommitteeID != nil && *c.CommitteeID != "" {
				mapped, err := s.idMapper.MapCommitteeV1ToV2(ctx, *c.CommitteeID)
				if err != nil {
					s.logger.WarnContext(ctx, "failed to map committee ID from V1 to V2, using V1 ID",
						"committee_v1_sfid", *c.CommitteeID,
						"error", err,
					)
					// Fall back to V1 ID if mapping fails
					committeeV2 = c.CommitteeID
				} else {
					committeeV2 = &mapped
				}
			}

			// Map project ID from V1 to V2 if present
			var projectV2 *string
			if c.ProjectID != nil && *c.ProjectID != "" {
				mapped, err := s.idMapper.MapProjectV1ToV2(ctx, *c.ProjectID)
				if err != nil {
					s.logger.WarnContext(ctx, "failed to map project ID from V1 to V2, using V1 ID",
						"project_v1_sfid", *c.ProjectID,
						"error", err,
					)
					// Fall back to V1 ID if mapping fails
					projectV2 = c.ProjectID
				} else {
					projectV2 = &mapped
				}
			}

			result[i] = &survey.SurveyCommittee{
				CommitteeName:   c.CommitteeName,
				CommitteeUID:    committeeV2,
				ProjectUID:      projectV2,
				ProjectName:     c.ProjectName,
				SurveyURL:       c.SurveyURL,
				TotalRecipients: c.TotalRecipients,
				TotalResponses:  c.TotalResponses,
				NpsValue:        c.NPSValue,
			}

			return nil
		}
	}

	// Execute all mapping functions concurrently
	if err := pool.Run(ctx, mappingFunctions...); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *SurveyService) mapPreviewSendResponseToResult(ctx context.Context, itxResponse *itx.PreviewSendResponse) (*survey.PreviewSendResult, error) {
	// Map projects with V1→V2 ID mapping
	projects, err := s.mapLFXProjectsToResult(ctx, itxResponse.AffectedProjects)
	if err != nil {
		return nil, err
	}

	// Map committees with V1→V2 ID mapping
	committees, err := s.mapExcludedCommitteesToResult(ctx, itxResponse.AffectedCommittees)
	if err != nil {
		return nil, err
	}

	return &survey.PreviewSendResult{
		AffectedProjects:   projects,
		AffectedCommittees: committees,
		AffectedRecipients: mapITXPreviewRecipientsToResult(itxResponse.AffectedRecipients),
	}, nil
}

func (s *SurveyService) mapLFXProjectsToResult(ctx context.Context, projects []itx.LFXProject) ([]*survey.LFXProject, error) {
	// Always return an empty slice instead of nil to ensure JSON marshals as []
	if len(projects) == 0 {
		return make([]*survey.LFXProject, 0), nil
	}

	result := make([]*survey.LFXProject, len(projects))

	// Create worker pool with 5 workers
	pool := concurrent.NewWorkerPool(5)

	// Build mapping functions for each project
	mappingFunctions := make([]func() error, len(projects))
	for i, p := range projects {
		mappingFunctions[i] = func() error {
			// Map project ID from V1 to V2 if present
			projectV2 := p.ID
			if p.ID != "" {
				mapped, err := s.idMapper.MapProjectV1ToV2(ctx, p.ID)
				if err != nil {
					s.logger.WarnContext(ctx, "failed to map project ID from V1 to V2, using V1 ID",
						"project_v1_sfid", p.ID,
						"error", err,
					)
					// Fall back to V1 ID if mapping fails
				} else {
					projectV2 = mapped
				}
			}

			result[i] = &survey.LFXProject{
				ID:      projectV2,
				Name:    p.Name,
				Slug:    p.Slug,
				Status:  p.Status,
				LogoURL: p.LogoURL,
			}

			return nil
		}
	}

	// Execute all mapping functions concurrently
	if err := pool.Run(ctx, mappingFunctions...); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *SurveyService) mapExcludedCommitteesToResult(ctx context.Context, committees []itx.ExcludedCommittee) ([]*survey.ExcludedCommittee, error) {
	// Always return an empty slice instead of nil to ensure JSON marshals as []
	if len(committees) == 0 {
		return make([]*survey.ExcludedCommittee, 0), nil
	}

	result := make([]*survey.ExcludedCommittee, len(committees))

	// Create worker pool with 5 workers
	pool := concurrent.NewWorkerPool(5)

	// Build mapping functions for each committee
	mappingFunctions := make([]func() error, len(committees))
	for i, c := range committees {
		mappingFunctions[i] = func() error {
			// Map committee ID from V1 to V2 if present
			committeeV2 := c.CommitteeID
			if c.CommitteeID != "" {
				mapped, err := s.idMapper.MapCommitteeV1ToV2(ctx, c.CommitteeID)
				if err != nil {
					s.logger.WarnContext(ctx, "failed to map committee ID from V1 to V2, using V1 ID",
						"committee_v1_sfid", c.CommitteeID,
						"error", err,
					)
					// Fall back to V1 ID if mapping fails
				} else {
					committeeV2 = mapped
				}
			}

			// Map project ID from V1 to V2 if present
			projectV2 := c.ProjectID
			if c.ProjectID != "" {
				mapped, err := s.idMapper.MapProjectV1ToV2(ctx, c.ProjectID)
				if err != nil {
					s.logger.WarnContext(ctx, "failed to map project ID from V1 to V2, using V1 ID",
						"project_v1_sfid", c.ProjectID,
						"error", err,
					)
					// Fall back to V1 ID if mapping fails
				} else {
					projectV2 = mapped
				}
			}

			result[i] = &survey.ExcludedCommittee{
				ProjectUID:        projectV2,
				ProjectName:       c.ProjectName,
				CommitteeUID:      committeeV2,
				CommitteeName:     c.CommitteeName,
				CommitteeCategory: c.CommitteeCategory,
			}

			return nil
		}
	}

	// Execute all mapping functions concurrently
	if err := pool.Run(ctx, mappingFunctions...); err != nil {
		return nil, err
	}

	return result, nil
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

func (s *SurveyService) mapExclusionToResult(ctx context.Context, itxExclusion *itx.Exclusion) (*survey.ExclusionResult, error) {
	// Map committee ID from V1 to V2 if present
	var committeeV2 *string
	if itxExclusion.CommitteeID != nil && *itxExclusion.CommitteeID != "" {
		mapped, err := s.idMapper.MapCommitteeV1ToV2(ctx, *itxExclusion.CommitteeID)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to map committee ID from V1 to V2, using V1 ID",
				"committee_v1_sfid", *itxExclusion.CommitteeID,
				"error", err,
			)
			// Fall back to V1 ID if mapping fails
			committeeV2 = itxExclusion.CommitteeID
		} else {
			committeeV2 = &mapped
		}
	}

	return &survey.ExclusionResult{
		UID:             itxExclusion.ID,
		Email:           itxExclusion.Email,
		SurveyUID:       itxExclusion.SurveyID,
		CommitteeUID:    committeeV2,
		GlobalExclusion: itxExclusion.GlobalExclusion,
		UserID:          itxExclusion.UserID,
	}, nil
}

func (s *SurveyService) mapExtendedExclusionToResult(ctx context.Context, itxExclusion *itx.ExtendedExclusion) (*survey.ExtendedExclusionResult, error) {
	// Map committee ID from V1 to V2 if present
	var committeeV2 *string
	if itxExclusion.CommitteeID != nil && *itxExclusion.CommitteeID != "" {
		mapped, err := s.idMapper.MapCommitteeV1ToV2(ctx, *itxExclusion.CommitteeID)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to map committee ID from V1 to V2, using V1 ID",
				"committee_v1_sfid", *itxExclusion.CommitteeID,
				"error", err,
			)
			// Fall back to V1 ID if mapping fails
			committeeV2 = itxExclusion.CommitteeID
		} else {
			committeeV2 = &mapped
		}
	}

	result := &survey.ExtendedExclusionResult{
		UID:             itxExclusion.ID,
		Email:           itxExclusion.Email,
		SurveyUID:       itxExclusion.SurveyID,
		CommitteeUID:    committeeV2,
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

	return result, nil
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
