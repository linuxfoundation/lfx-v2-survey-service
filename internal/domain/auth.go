// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// Authenticator defines the interface for JWT authentication.
// The logger is intentionally absent: it is an adapter concern and must be
// handled by the concrete implementation, not by callers.
type Authenticator interface {
	// ParsePrincipal validates the JWT and returns the principal (user identifier).
	ParsePrincipal(ctx context.Context, token string) (string, error)
}
