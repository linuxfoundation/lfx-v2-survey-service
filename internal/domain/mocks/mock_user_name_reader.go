// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

// MockUserNameReader is a configurable test double for domain.UserNameReader.
// Set Name and Err for a fixed response, or set DisplayNameByUsernameFunc for dynamic behaviour.
type MockUserNameReader struct {
	Name                      string
	Err                       error
	DisplayNameByUsernameFunc func(ctx context.Context, username string) (string, error)
}

// Compile-time assertion.
var _ domain.UserNameReader = (*MockUserNameReader)(nil)

// DisplayNameByUsername returns the configured Name and Err.
func (m *MockUserNameReader) DisplayNameByUsername(ctx context.Context, username string) (string, error) {
	if m.DisplayNameByUsernameFunc != nil {
		return m.DisplayNameByUsernameFunc(ctx, username)
	}
	return m.Name, m.Err
}
