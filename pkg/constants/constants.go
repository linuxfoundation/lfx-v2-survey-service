// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

import "slices"

// HealthPaths lists the HTTP paths used for health/liveness/readiness probes.
var HealthPaths = []string{"/health", "/livez", "/readyz"}

// IsHealthPath reports whether path is a health probe endpoint.
func IsHealthPath(path string) bool {
	return slices.Contains(HealthPaths, path)
}

type contextKey string

const (
	// HTTP Headers
	AuthorizationHeader = "authorization"
	RequestIDHeader     = "X-REQUEST-ID"
	EtagHeader          = "ETag"

	// Context Keys
	AuthorizationContextID contextKey = "authorization"
	RequestIDContextID     contextKey = "X-REQUEST-ID"
	PrincipalContextID     contextKey = "principal"
)
