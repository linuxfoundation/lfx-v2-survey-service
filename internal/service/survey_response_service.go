// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// DeleteSurveyResponse removes a recipient from survey and recalculates statistics
func (s *SurveyService) DeleteSurveyResponse(ctx context.Context, p *survey.DeleteSurveyResponsePayload) error {
	principal := principalFromCtx(ctx)

	s.logger.InfoContext(ctx, "deleting survey response",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"response_id", p.ResponseID,
	)

	// Call ITX API
	err := s.responseClient.DeleteResponse(ctx, p.SurveyUID, p.ResponseID)
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
	principal := principalFromCtx(ctx)

	s.logger.InfoContext(ctx, "resending survey response",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"response_id", p.ResponseID,
	)

	// Call ITX API
	err := s.responseClient.ResendResponse(ctx, p.SurveyUID, p.ResponseID)
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
	principal := principalFromCtx(ctx)

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
	err = s.surveyClient.DeleteRecipientGroup(ctx, p.SurveyUID, committeeV1, projectV1, p.FoundationID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "recipient group deleted successfully",
		"survey_uid", p.SurveyUID,
	)

	return nil
}

// ListSurveyResponses returns a paginated list of individual per-recipient responses for a survey
func (s *SurveyService) ListSurveyResponses(ctx context.Context, p *survey.ListSurveyResponsesPayload) (*survey.SurveyResponsesPage, error) {
	principal := principalFromCtx(ctx)

	s.logger.InfoContext(ctx, "listing survey responses",
		"principal", principal,
		"survey_uid", p.SurveyUID,
		"project_uid", p.ProjectUID,
		"project_uids", p.ProjectUids,
		"per_page", p.PerPage,
	)

	// project_uid and project_uids are mutually exclusive — reject early.
	if p.ProjectUID != nil && *p.ProjectUID != "" &&
		p.ProjectUids != nil && *p.ProjectUids != "" {
		return nil, mapDomainError(domain.NewValidationError(
			"project_uid and project_uids are mutually exclusive"))
	}

	// Build ITX params with optional V2→V1 ID mapping for project filters
	params := &itx.ListResponsesParams{
		PageToken: p.PageToken,
		PerPage:   p.PerPage,
	}

	if p.ProjectUID != nil && *p.ProjectUID != "" {
		projectV1, err := s.idMapper.MapProjectV2ToV1(ctx, *p.ProjectUID)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to map project_uid to V1",
				"project_uid", *p.ProjectUID,
				"error", err,
			)
			return nil, mapDomainError(err)
		}
		params.ProjectID = &projectV1
		s.logger.DebugContext(ctx, "mapped project_uid for responses filter",
			"project_v2_uid", *p.ProjectUID,
			"project_v1_sfid", projectV1,
		)
	}

	if p.ProjectUids != nil && *p.ProjectUids != "" {
		projectV1IDs, err := s.mapProjectUIDsV2ToV1(ctx, *p.ProjectUids)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to map project_uids to V1",
				"project_uids", *p.ProjectUids,
				"error", err,
			)
			return nil, mapDomainError(err)
		}
		params.ProjectIDs = &projectV1IDs
		s.logger.DebugContext(ctx, "mapped project_uids for responses filter",
			"project_v2_uids", *p.ProjectUids,
			"project_v1_sfids", projectV1IDs,
		)
	}

	// Call ITX API
	itxResponse, err := s.responseClient.ListResponses(ctx, p.SurveyUID, params)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// Map response back to Goa result (V1→V2 ID mapping per response)
	result, err := s.mapITXResponsesToPage(ctx, itxResponse)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to map ITX responses",
			"error", err,
		)
		return nil, mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "survey responses listed successfully",
		"survey_uid", p.SurveyUID,
		"count", len(result.Data),
	)

	return result, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Response mappers
// ──────────────────────────────────────────────────────────────────────────────

// mapITXResponsesToPage maps ITX paginated responses to a Goa result with V1→V2 ID mapping.
// Uses a worker pool to run per-item NATS ID-mapper lookups concurrently, matching the
// pattern used by mapSurveyCommitteesToResult and mapLFXProjectsToResult.
func (s *SurveyService) mapITXResponsesToPage(ctx context.Context, itxResponse *itx.PaginatedSurveyResponses) (*survey.SurveyResponsesPage, error) {
	data := make([]*survey.SurveyResponseItem, len(itxResponse.Data))

	pool := concurrent.NewWorkerPool(5)
	mappingFunctions := make([]func() error, len(itxResponse.Data))
	for i, r := range itxResponse.Data {
		i, r := i, r
		mappingFunctions[i] = func() error {
			item, err := s.mapITXRecipientResponseToItem(ctx, r)
			if err != nil {
				return err
			}
			data[i] = item
			return nil
		}
	}

	// pool.Run blocks until all functions complete; data[i] writes are index-disjoint and safe.
	if err := pool.Run(ctx, mappingFunctions...); err != nil {
		return nil, err
	}

	return &survey.SurveyResponsesPage{
		Data: data,
		Meta: &survey.SurveyResponsePageMeta{
			PageToken:    &itxResponse.Meta.PageToken,
			TotalPages:   &itxResponse.Meta.TotalPages,
			TotalResults: &itxResponse.Meta.TotalResults,
			PerPage:      &itxResponse.Meta.PerPage,
		},
	}, nil
}

// mapITXRecipientResponseToItem maps a single ITX SurveyRecipientResponse to a Goa SurveyResponseItem
func (s *SurveyService) mapITXRecipientResponseToItem(ctx context.Context, r itx.SurveyRecipientResponse) (*survey.SurveyResponseItem, error) {
	// Map project V1 ID → V2 UID if present; fall back to original on failure
	var projectResult *survey.SurveyResponseProj
	if r.Project != nil {
		var uid *string
		name := r.Project.Name
		if r.Project.ID != nil && *r.Project.ID != "" {
			mapped, err := s.idMapper.MapProjectV1ToV2(ctx, *r.Project.ID)
			if err != nil {
				s.logger.WarnContext(ctx, "failed to map project ID from V1 to V2 for response, using V1 ID",
					"project_v1_sfid", *r.Project.ID,
					"error", err,
				)
				uid = r.Project.ID
			} else {
				uid = &mapped
			}
		}
		projectResult = &survey.SurveyResponseProj{
			UID:  uid,
			Name: name,
		}
	}

	// Map committee V1 ID → V2 UID if present; fall back to original on failure
	var committeeUID *string
	if r.CommitteeID != nil && *r.CommitteeID != "" {
		mapped, err := s.idMapper.MapCommitteeV1ToV2(ctx, *r.CommitteeID)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to map committee ID from V1 to V2 for response, using V1 ID",
				"committee_v1_sfid", *r.CommitteeID,
				"error", err,
			)
			committeeUID = r.CommitteeID
		} else {
			committeeUID = &mapped
		}
	}

	// Map organization (no ID mapping needed — org IDs are not V1 SFIDs)
	var orgResult *survey.SurveyResponseOrg
	if r.Organization != nil {
		orgResult = &survey.SurveyResponseOrg{
			ID:   r.Organization.ID,
			Name: r.Organization.Name,
		}
	}

	// Map SurveyMonkey question answers — always an empty slice, never nil
	answers := make([]*survey.SurveyQuestionAnswer, 0, len(r.SurveyMonkeyQuestionAnswers))
	for _, qa := range r.SurveyMonkeyQuestionAnswers {
		choices := make([]*survey.SurveyAnswerChoice, 0, len(qa.Answers))
		for _, a := range qa.Answers {
			choices = append(choices, &survey.SurveyAnswerChoice{
				ChoiceID: a.ChoiceID,
				Text:     a.Text,
			})
		}
		answers = append(answers, &survey.SurveyQuestionAnswer{
			QuestionID:      qa.QuestionID,
			QuestionText:    qa.QuestionText,
			QuestionFamily:  qa.QuestionFamily,
			QuestionSubtype: qa.QuestionSubtype,
			Answers:         choices,
		})
	}

	return &survey.SurveyResponseItem{
		ID:                            r.ID,
		SurveyUID:                     r.SurveyID,
		SurveyLink:                    r.SurveyLink,
		CommitteeUID:                  committeeUID,
		Email:                         r.Email,
		FirstName:                     r.FirstName,
		LastName:                      r.LastName,
		Username:                      r.Username,
		Role:                          r.Role,
		JobTitle:                      r.JobTitle,
		MembershipTier:                r.MembershipTier,
		VotingStatus:                  r.VotingStatus,
		Organization:                  orgResult,
		Project:                       projectResult,
		ResponseStatus:                r.ResponseStatus,
		CreatedAt:                     r.CreatedAt,
		ResponseDatetime:              r.ResponseDatetime,
		LastReceivedTime:              r.LastReceivedTime,
		NumAutomatedRemindersReceived: r.NumAutomatedRemindersReceived,
		NpsValue:                      r.NPSValue,
		SurveyMonkeyRespondentID:      r.SurveyMonkeyRespondentID,
		SurveyMonkeyQuestionAnswers:   answers,
		// SES tracking
		SesMessageID:            r.SESMessageID,
		SesDeliverySuccessful:   r.SESDeliverySuccessful,
		SesBounceType:           r.SESBounceType,
		SesBounceSubtype:        r.SESBounceSubtype,
		SesBounceDiagnosticCode: r.SESBounceDiagnosticCode,
		SesComplaintExists:      r.SESComplaintExists,
		SesComplaintType:        r.SESComplaintType,
		SesComplaintDate:        r.SESComplaintDate,
		SesEmailOpened:          r.SESEmailOpened,
		SesEmailOpenedLastTime:  r.SESEmailOpenedLastTime,
		SesLinkClicked:          r.SESLinkClicked,
		SesLinkClickedLastTime:  r.SESLinkClickedLastTime,
	}, nil
}
