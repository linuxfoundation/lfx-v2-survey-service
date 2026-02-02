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
	result := &survey.SurveyScheduleResult{
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

	s.logger.InfoContext(ctx, "survey scheduled successfully",
		"survey_id", result.ID,
		"survey_status", result.SurveyStatus,
	)

	return result, nil
}

// Helper functions

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
