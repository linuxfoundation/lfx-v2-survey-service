// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

// MockInviteAcceptanceClient is a configurable test double for domain.InviteAcceptanceClient.
// Set Err for a fixed error response, or set AcceptInviteFunc for dynamic behaviour.
type MockInviteAcceptanceClient struct {
	Err               error
	AcceptInviteFunc  func(ctx context.Context, email, username string) error
}

// Compile-time assertion.
var _ domain.InviteAcceptanceClient = (*MockInviteAcceptanceClient)(nil)

// AcceptInvite returns the configured Err.
func (m *MockInviteAcceptanceClient) AcceptInvite(ctx context.Context, email, username string) error {
	if m.AcceptInviteFunc != nil {
		return m.AcceptInviteFunc(ctx, email, username)
	}
	return m.Err
}
