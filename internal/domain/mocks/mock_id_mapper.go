// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

// MockIDMapper is a configurable test double for domain.IDMapper.
// Each method has a corresponding Func field; when nil the method panics.
type MockIDMapper struct {
	MapProjectV2ToV1Func   func(ctx context.Context, v2UID string) (string, error)
	MapProjectV1ToV2Func   func(ctx context.Context, v1SFID string) (string, error)
	MapCommitteeV2ToV1Func func(ctx context.Context, v2UID string) (string, error)
	MapCommitteeV1ToV2Func func(ctx context.Context, v1SFID string) (string, error)
}

// Compile-time assertion.
var _ domain.IDMapper = (*MockIDMapper)(nil)

func (m *MockIDMapper) MapProjectV2ToV1(ctx context.Context, v2UID string) (string, error) {
	if m.MapProjectV2ToV1Func != nil {
		return m.MapProjectV2ToV1Func(ctx, v2UID)
	}
	panic("MockIDMapper.MapProjectV2ToV1: not configured")
}

func (m *MockIDMapper) MapProjectV1ToV2(ctx context.Context, v1SFID string) (string, error) {
	if m.MapProjectV1ToV2Func != nil {
		return m.MapProjectV1ToV2Func(ctx, v1SFID)
	}
	panic("MockIDMapper.MapProjectV1ToV2: not configured")
}

func (m *MockIDMapper) MapCommitteeV2ToV1(ctx context.Context, v2UID string) (string, error) {
	if m.MapCommitteeV2ToV1Func != nil {
		return m.MapCommitteeV2ToV1Func(ctx, v2UID)
	}
	panic("MockIDMapper.MapCommitteeV2ToV1: not configured")
}

func (m *MockIDMapper) MapCommitteeV1ToV2(ctx context.Context, v1SFID string) (string, error) {
	if m.MapCommitteeV1ToV2Func != nil {
		return m.MapCommitteeV1ToV2Func(ctx, v1SFID)
	}
	panic("MockIDMapper.MapCommitteeV1ToV2: not configured")
}
