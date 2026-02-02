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

// JWTAuth implements survey.Auther.JWTAuth
// This is called by goa to validate JWT tokens before calling service methods
func (api *SurveyAPI) JWTAuth(ctx context.Context, token string, scheme *security.JWTScheme) (context.Context, error) {
	// The actual JWT validation is performed in the service layer
	// Here we just pass the context through since goa needs this method to exist
	return ctx, nil
}
