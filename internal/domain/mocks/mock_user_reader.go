// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

// MockUserReader is a configurable test double for domain.UserReader.
// Set Username and Err for a fixed response, or set UsernameByEmailFunc for dynamic behaviour.
type MockUserReader struct {
	Username            string
	Err                 error
	UsernameByEmailFunc func(ctx context.Context, email string) (string, error)
}

// Compile-time assertion.
var _ domain.UserReader = (*MockUserReader)(nil)

// UsernameByEmail returns the configured Username and Err.
func (m *MockUserReader) UsernameByEmail(ctx context.Context, email string) (string, error) {
	if m.UsernameByEmailFunc != nil {
		return m.UsernameByEmailFunc(ctx, email)
	}
	return m.Username, m.Err
}
