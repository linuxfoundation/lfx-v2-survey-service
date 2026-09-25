// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// CreateExclusion creates a survey or global exclusion
func (s *SurveyService) CreateExclusion(ctx context.Context, p *survey.CreateExclusionPayload) (*survey.ExclusionResult, error) {
	principal := principalFromCtx(ctx)

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
	itxResponse, err := s.exclusionClient.CreateExclusion(ctx, itxRequest)
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
	principal := principalFromCtx(ctx)

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
	err = s.exclusionClient.DeleteExclusion(ctx, itxRequest)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "exclusion deleted successfully")

	return nil
}

// GetExclusion retrieves an exclusion by ID
func (s *SurveyService) GetExclusion(ctx context.Context, p *survey.GetExclusionPayload) (*survey.ExtendedExclusionResult, error) {
	principal := principalFromCtx(ctx)

	s.logger.InfoContext(ctx, "getting exclusion",
		"principal", principal,
		"exclusion_id", p.ExclusionID,
	)

	// Call ITX API
	itxResponse, err := s.exclusionClient.GetExclusion(ctx, p.ExclusionID)
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
	principal := principalFromCtx(ctx)

	s.logger.InfoContext(ctx, "deleting exclusion by ID",
		"principal", principal,
		"exclusion_id", p.ExclusionID,
	)

	// Call ITX API
	err := s.exclusionClient.DeleteExclusionByID(ctx, p.ExclusionID)
	if err != nil {
		return mapDomainError(err)
	}

	s.logger.InfoContext(ctx, "exclusion deleted successfully",
		"exclusion_id", p.ExclusionID,
	)

	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Exclusion mappers
// ──────────────────────────────────────────────────────────────────────────────

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
