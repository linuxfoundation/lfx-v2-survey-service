// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

type SurveyService struct {
	surveyClient    domain.SurveyClient
	exclusionClient domain.ExclusionClient
	responseClient  domain.SurveyResponseClient
	idMapper        domain.IDMapper
	logger          *slog.Logger
}

// NewSurveyService wires the service with the three focused sub-interfaces.
// In production, pass the same *proxy.Client for all three — it satisfies
// domain.ITXProxyClient, which embeds all three. In tests, supply a narrower
// mock for the sub-interface under test and a no-op stub for the others.
//
// Authentication is NOT a service concern: it is handled by JWTAuth in the
// Goa adapter layer, which stores the principal in context before any service
// method is invoked. Service methods read the principal via
// ctx.Value(constants.PrincipalContextID).
func NewSurveyService(
	surveyClient domain.SurveyClient,
	exclusionClient domain.ExclusionClient,
	responseClient domain.SurveyResponseClient,
	idMapper domain.IDMapper,
	logger *slog.Logger,
) *SurveyService {
	return &SurveyService{
		surveyClient:    surveyClient,
		exclusionClient: exclusionClient,
		responseClient:  responseClient,
		idMapper:        idMapper,
		logger:          logger,
	}
}

// ServiceReady returns an error if any required dependency was not injected.
// Call this during startup (before serving traffic) to fail fast on misconfiguration
// rather than panicking at the first request.
func (s *SurveyService) ServiceReady() error {
	if s.surveyClient == nil {
		return fmt.Errorf("SurveyService: surveyClient (SurveyClient) is nil")
	}
	if s.exclusionClient == nil {
		return fmt.Errorf("SurveyService: exclusionClient (ExclusionClient) is nil")
	}
	if s.responseClient == nil {
		return fmt.Errorf("SurveyService: responseClient (SurveyResponseClient) is nil")
	}
	if s.idMapper == nil {
		return fmt.Errorf("SurveyService: idMapper (IDMapper) is nil")
	}
	if s.logger == nil {
		return fmt.Errorf("SurveyService: logger is nil")
	}
	return nil
}

// ScheduleSurvey implements survey.Service.ScheduleSurvey
func (s *SurveyService) ScheduleSurvey(ctx context.Context, p *survey.ScheduleSurveyPayload) (*survey.SurveyScheduleResult, error) {
	principal := principalFromCtx(ctx)

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
	itxResponse, err := s.surveyClient.ScheduleSurvey(ctx, itxRequest)
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
	principal := principalFromCtx(ctx)

	s.logger.InfoContext(ctx, "getting survey",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"project_uid", p.ProjectUID,
		"project_uids", p.ProjectUids,
	)

	// project_uid and project_uids are mutually exclusive — reject early.
	if p.ProjectUID != nil && *p.ProjectUID != "" &&
		p.ProjectUids != nil && *p.ProjectUids != "" {
		return nil, mapDomainError(domain.NewValidationError(
			"project_uid and project_uids are mutually exclusive"))
	}

	// Build query parameters with V2 to V1 ID mapping
	var queryParams *itx.GetSurveyParams
	if p.ProjectUID != nil || p.ProjectUids != nil {
		queryParams = &itx.GetSurveyParams{}

		// Map single project_uid from V2 to V1
		if p.ProjectUID != nil && *p.ProjectUID != "" {
			projectV1, err := s.idMapper.MapProjectV2ToV1(ctx, *p.ProjectUID)
			if err != nil {
				s.logger.ErrorContext(ctx, "failed to map project_uid to V1",
					"project_uid", *p.ProjectUID,
					"error", err,
				)
				return nil, mapDomainError(err)
			}
			queryParams.ProjectID = &projectV1
			s.logger.DebugContext(ctx, "mapped project_uid",
				"project_v2_uid", *p.ProjectUID,
				"project_v1_sfid", projectV1,
			)
		}

		// Map project_uids from V2 to V1 (comma-delimited list)
		if p.ProjectUids != nil && *p.ProjectUids != "" {
			projectV1IDs, err := s.mapProjectUIDsV2ToV1(ctx, *p.ProjectUids)
			if err != nil {
				s.logger.ErrorContext(ctx, "failed to map project_uids to V1",
					"project_uids", *p.ProjectUids,
					"error", err,
				)
				return nil, mapDomainError(err)
			}
			queryParams.ProjectIDs = &projectV1IDs
			s.logger.DebugContext(ctx, "mapped project_uids",
				"project_v2_uids", *p.ProjectUids,
				"project_v1_sfids", projectV1IDs,
			)
		}
	}

	// Call ITX API
	itxResponse, err := s.surveyClient.GetSurvey(ctx, p.SurveyUID, queryParams)
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
	principal := principalFromCtx(ctx)

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
	itxResponse, err := s.surveyClient.UpdateSurvey(ctx, p.SurveyUID, itxRequest)
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
	principal := principalFromCtx(ctx)

	s.logger.InfoContext(ctx, "deleting survey",
		"principal", principal,
		"survey_uid", p.SurveyUID,
	)

	// Call ITX API
	err := s.surveyClient.DeleteSurvey(ctx, p.SurveyUID)
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
	principal := principalFromCtx(ctx)

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
	err := s.surveyClient.BulkResendSurvey(ctx, p.SurveyUID, itxRequest)
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
	principal := principalFromCtx(ctx)

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
	itxResponse, err := s.surveyClient.PreviewSend(ctx, p.SurveyUID, committeeV1)
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
	principal := principalFromCtx(ctx)

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
	err = s.surveyClient.SendMissingRecipients(ctx, p.SurveyUID, committeeV1)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey send to missing recipients dispatched successfully",
		"survey_uid", p.SurveyUID,
	)

	return nil
}

// ValidateEmail validates email template body and subject
func (s *SurveyService) ValidateEmail(ctx context.Context, p *survey.ValidateEmailPayload) (*survey.ValidateEmailResult, error) {
	principal := principalFromCtx(ctx)

	s.logger.InfoContext(ctx, "validating email template",
		"principal", principal,
	)

	// Build ITX request
	itxRequest := &itx.ValidateEmailRequest{
		Body:    p.Body,
		Subject: p.Subject,
	}

	// Call ITX API
	itxResponse, err := s.surveyClient.ValidateEmail(ctx, itxRequest)
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

// ──────────────────────────────────────────────────────────────────────────────
// Shared private helpers (used across multiple service files)
// ──────────────────────────────────────────────────────────────────────────────

// principalFromCtx reads the authenticated principal that JWTAuth stored in context.
// If no principal is present (e.g. in tests) an empty string is returned; the value
// is used only for structured logging, not for authorization decisions.
func principalFromCtx(ctx context.Context) string {
	principal, _ := ctx.Value(constants.PrincipalContextID).(string)
	return principal
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

// mapProjectUIDsV2ToV1 maps a comma-delimited list of project UIDs from V2 to V1.
// Mapping calls are fanned out concurrently using the shared WorkerPool — each goroutine
// writes to a pre-allocated, index-disjoint slot in v1IDs (the same pattern used by
// mapSurveyCommitteesToResult and mapITXResponsesToPage).
func (s *SurveyService) mapProjectUIDsV2ToV1(ctx context.Context, projectUIDs string) (string, error) {
	if projectUIDs == "" {
		return "", nil
	}

	// Split comma-delimited list and trim whitespace, discarding empty tokens.
	raw := strings.Split(projectUIDs, ",")
	validUIDs := make([]string, 0, len(raw))
	for _, u := range raw {
		if t := strings.TrimSpace(u); t != "" {
			validUIDs = append(validUIDs, t)
		}
	}
	if len(validUIDs) == 0 {
		return "", nil
	}

	// Pre-allocate result slice; goroutines write to index-disjoint slots.
	v1IDs := make([]string, len(validUIDs))
	pool := concurrent.NewWorkerPool(5)
	mappingFunctions := make([]func() error, len(validUIDs))
	for i, uid := range validUIDs {
		i, uid := i, uid
		mappingFunctions[i] = func() error {
			mapped, err := s.idMapper.MapProjectV2ToV1(ctx, uid)
			if err != nil {
				s.logger.ErrorContext(ctx, "failed to map project UID to V1",
					"project_v2_uid", uid,
					"error", err,
				)
				return err
			}
			v1IDs[i] = mapped
			return nil
		}
	}

	// pool.Run blocks until all functions complete; v1IDs[i] writes are index-disjoint and safe.
	if err := pool.Run(ctx, mappingFunctions...); err != nil {
		return "", err
	}

	// Join back into comma-delimited string
	return strings.Join(v1IDs, ","), nil
}

// mapDomainError converts a domain error into the appropriate Goa error type.
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

// ──────────────────────────────────────────────────────────────────────────────
// Survey response mappers (used by ScheduleSurvey, GetSurvey, UpdateSurvey)
// ──────────────────────────────────────────────────────────────────────────────

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
