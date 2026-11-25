// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package idmapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoOpMapper(t *testing.T) {
	ctx := context.Background()
	m := NewNoOpMapper()
	require.NotNil(t, m)

	tests := []struct {
		name string
		call func(context.Context, string) (string, error)
	}{
		{"MapProjectV2ToV1", m.MapProjectV2ToV1},
		{"MapProjectV1ToV2", m.MapProjectV1ToV2},
		{"MapCommitteeV2ToV1", m.MapCommitteeV2ToV1},
		{"MapCommitteeV1ToV2", m.MapCommitteeV1ToV2},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns the id unchanged", func(t *testing.T) {
			got, err := tt.call(ctx, "id-1")
			require.NoError(t, err)
			assert.Equal(t, "id-1", got)
		})

		t.Run(tt.name+" does not reject an empty id", func(t *testing.T) {
			got, err := tt.call(ctx, "")
			require.NoError(t, err)
			assert.Empty(t, got)
		})
	}

	t.Run("Close is a no-op", func(t *testing.T) {
		assert.NotPanics(t, m.Close)
	})
}
