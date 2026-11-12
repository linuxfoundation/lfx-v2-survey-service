// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainError_Error(t *testing.T) {
	t.Run("message only", func(t *testing.T) {
		err := &domain.DomainError{Message: "survey not found"}
		assert.Equal(t, "survey not found", err.Error())
	})

	t.Run("message is followed by the wrapped error", func(t *testing.T) {
		err := &domain.DomainError{Message: "failed to call ITX", Err: errors.New("connection refused")}
		assert.Equal(t, "failed to call ITX: connection refused", err.Error())
	})
}

func TestDomainError_Unwrap(t *testing.T) {
	cause := errors.New("disk full")

	t.Run("returns the wrapped error", func(t *testing.T) {
		err := &domain.DomainError{Message: "boom", Err: cause}
		assert.Same(t, cause, err.Unwrap())
	})

	t.Run("returns nil when nothing is wrapped", func(t *testing.T) {
		err := &domain.DomainError{Message: "boom"}
		assert.NoError(t, err.Unwrap())
	})

	t.Run("errors.Is reaches the cause", func(t *testing.T) {
		err := domain.NewInternalError("boom", cause)
		assert.ErrorIs(t, err, cause)
	})

	t.Run("errors.As finds the domain error through an outer wrap", func(t *testing.T) {
		wrapped := fmt.Errorf("handler: %w", domain.NewConflictError("already exists"))

		var domErr *domain.DomainError
		require.ErrorAs(t, wrapped, &domErr)
		assert.Equal(t, domain.ErrorTypeConflict, domErr.Type)
		assert.Equal(t, "already exists", domErr.Message)
	})
}

func TestDomainErrorConstructors(t *testing.T) {
	tests := []struct {
		name     string
		new      func(message string, err ...error) *domain.DomainError
		wantType domain.ErrorType
	}{
		{"NewValidationError", domain.NewValidationError, domain.ErrorTypeValidation},
		{"NewNotFoundError", domain.NewNotFoundError, domain.ErrorTypeNotFound},
		{"NewConflictError", domain.NewConflictError, domain.ErrorTypeConflict},
		{"NewInternalError", domain.NewInternalError, domain.ErrorTypeInternal},
		{"NewUnavailableError", domain.NewUnavailableError, domain.ErrorTypeUnavailable},
	}

	causeA := errors.New("cause a")
	causeB := errors.New("cause b")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("sets type and message", func(t *testing.T) {
				err := tt.new("something happened")

				assert.Equal(t, tt.wantType, err.Type)
				assert.Equal(t, "something happened", err.Message)
				assert.Equal(t, "something happened", err.Error())
			})

			t.Run("no wrapped error leaves Err nil", func(t *testing.T) {
				assert.NoError(t, tt.new("something happened").Err)
			})

			t.Run("a nil error argument is ignored", func(t *testing.T) {
				err := tt.new("something happened", nil)

				assert.NoError(t, err.Err)
				assert.Equal(t, "something happened", err.Error())
			})

			t.Run("wraps a single cause", func(t *testing.T) {
				err := tt.new("something happened", causeA)

				assert.ErrorIs(t, err, causeA)
				assert.Equal(t, "something happened: cause a", err.Error())
			})

			t.Run("wraps every cause when several are given", func(t *testing.T) {
				err := tt.new("something happened", causeA, causeB)

				assert.ErrorIs(t, err, causeA)
				assert.ErrorIs(t, err, causeB)
			})
		})
	}
}

func TestErrUserNotFound(t *testing.T) {
	wrapped := fmt.Errorf("lookup alice@example.com: %w", domain.ErrUserNotFound)

	assert.ErrorIs(t, wrapped, domain.ErrUserNotFound)
	assert.EqualError(t, domain.ErrUserNotFound, "user not found")
}
