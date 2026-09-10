// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package mocks contains test doubles for every domain interface in the survey
// service. Import this package from any test that needs to stub a domain seam;
// do not re-declare mock types in individual test files.
package mocks

import (
	"context"
	"log/slog"

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
func (m *MockAuthenticator) ParsePrincipal(_ context.Context, _ string, _ *slog.Logger) (string, error) {
	return m.Principal, m.Err
}
