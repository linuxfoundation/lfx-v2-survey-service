// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

// MockEventPublisher is a configurable test double for domain.EventPublisher.
// Each method has a corresponding Func field; when nil the method panics.
type MockEventPublisher struct {
	PublishSurveyEventFunc         func(ctx context.Context, action string, survey *domain.SurveyData) error
	PublishSurveyResponseEventFunc func(ctx context.Context, action string, response *domain.SurveyResponseData) error
	PublishSurveyTemplateEventFunc func(ctx context.Context, action string, template *domain.SurveyTemplateData) error
	CloseFunc                      func() error
}

// Compile-time assertion.
var _ domain.EventPublisher = (*MockEventPublisher)(nil)

func (m *MockEventPublisher) PublishSurveyEvent(ctx context.Context, action string, survey *domain.SurveyData) error {
	if m.PublishSurveyEventFunc != nil {
		return m.PublishSurveyEventFunc(ctx, action, survey)
	}
	panic("MockEventPublisher.PublishSurveyEvent: not configured")
}

func (m *MockEventPublisher) PublishSurveyResponseEvent(ctx context.Context, action string, response *domain.SurveyResponseData) error {
	if m.PublishSurveyResponseEventFunc != nil {
		return m.PublishSurveyResponseEventFunc(ctx, action, response)
	}
	panic("MockEventPublisher.PublishSurveyResponseEvent: not configured")
}

func (m *MockEventPublisher) PublishSurveyTemplateEvent(ctx context.Context, action string, template *domain.SurveyTemplateData) error {
	if m.PublishSurveyTemplateEventFunc != nil {
		return m.PublishSurveyTemplateEventFunc(ctx, action, template)
	}
	panic("MockEventPublisher.PublishSurveyTemplateEvent: not configured")
}

func (m *MockEventPublisher) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
