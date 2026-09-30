// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
)

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "NATS timeout", err: nats.ErrTimeout, want: true},
		{name: "NATS no responders", err: nats.ErrNoResponders, want: true},
		{name: "NATS connection closed", err: nats.ErrConnectionClosed, want: true},
		{name: "NATS connection draining", err: nats.ErrConnectionDraining, want: true},
		{name: "context deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "message-based timeout", err: errors.New("request timeout"), want: true},
		{name: "permanent error", err: errors.New("invalid payload"), want: false},
		{
			name: "auth-service lookup failure is retryable",
			err:  fmt.Errorf("%w: resolve invitee email to LFX username: %w", domain.ErrAuthServiceLookupFailed, errors.New("email_to_username failed: auth0 unavailable")),
			want: true,
		},
		{
			name: "auth-service lookup failure is retryable through multiple wraps",
			err: fmt.Errorf("failed to publish survey response event: %w",
				fmt.Errorf("failed to send survey response access message: %w",
					fmt.Errorf("%w: resolve invitee email to LFX username: %w", domain.ErrAuthServiceLookupFailed, errors.New("email_to_username failed: 429 too many requests")))),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTransientError(tt.err))
		})
	}
}
