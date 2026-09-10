// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

// MockAuthenticator is a configurable test double for domain.Authenticator.
// Set Principal and Err to control the return values; all calls return the same values.
type MockAuthenticator struct {
	Principal string
	Err       error
}

// Compile-time assertion.
var _ domain.Authenticator = (*MockAuthenticator)(nil)

// ParsePrincipal returns the configured Principal and Err.
func (m *MockAuthenticator) ParsePrincipal(_ context.Context, _ string) (string, error) {
	return m.Principal, m.Err
}
