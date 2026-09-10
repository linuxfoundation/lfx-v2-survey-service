// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

// MockInviteSender is a configurable test double for domain.InviteSender.
//
// Set Result and Err for a fixed response, or set SendInviteFunc for dynamic behaviour.
// Called tracks whether SendInvite was invoked; LastRequest captures the most recent call.
type MockInviteSender struct {
	Result          *domain.InviteResult
	Err             error
	Called          bool
	LastRequest     inviteapi.SendInviteRequest
	SendInviteFunc  func(ctx context.Context, req inviteapi.SendInviteRequest) (*domain.InviteResult, error)
}

// Compile-time assertion.
var _ domain.InviteSender = (*MockInviteSender)(nil)

// SendInvite records the call and returns the configured result.
func (m *MockInviteSender) SendInvite(ctx context.Context, req inviteapi.SendInviteRequest) (*domain.InviteResult, error) {
	m.Called = true
	m.LastRequest = req
	if m.SendInviteFunc != nil {
		return m.SendInviteFunc(ctx, req)
	}
	return m.Result, m.Err
}
