// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants_test

import (
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"github.com/stretchr/testify/assert"
)

// TestIsHealthPath pins exact-match behavior: the request logger and the
// middleware filter in main.go use it to skip probe traffic, so a prefix or
// case-insensitive match would silently drop logs for real requests.
func TestIsHealthPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/health", true},
		{"/livez", true},
		{"/readyz", true},
		{"", false},
		{"/", false},
		{"/healthz", false},
		{"/health/", false},
		{"/health/details", false},
		{"/HEALTH", false},
		{"health", false},
		{"/v1/surveys", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, constants.IsHealthPath(tt.path))
		})
	}
}
