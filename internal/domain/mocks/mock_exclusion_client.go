// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// MockExclusionClient is a configurable test double for domain.ExclusionClient.
// Each method has a corresponding Func field; when nil the method panics.
type MockExclusionClient struct {
	CreateExclusionFunc    func(ctx context.Context, req *itx.ExclusionRequest) (*itx.Exclusion, error)
	DeleteExclusionFunc    func(ctx context.Context, req *itx.ExclusionRequest) error
	GetExclusionFunc       func(ctx context.Context, exclusionID string) (*itx.ExtendedExclusion, error)
	DeleteExclusionByIDFunc func(ctx context.Context, exclusionID string) error
}

// Compile-time assertion.
var _ domain.ExclusionClient = (*MockExclusionClient)(nil)

func (m *MockExclusionClient) CreateExclusion(ctx context.Context, req *itx.ExclusionRequest) (*itx.Exclusion, error) {
	if m.CreateExclusionFunc != nil {
		return m.CreateExclusionFunc(ctx, req)
	}
	panic("MockExclusionClient.CreateExclusion: not configured")
}

func (m *MockExclusionClient) DeleteExclusion(ctx context.Context, req *itx.ExclusionRequest) error {
	if m.DeleteExclusionFunc != nil {
		return m.DeleteExclusionFunc(ctx, req)
	}
	panic("MockExclusionClient.DeleteExclusion: not configured")
}

func (m *MockExclusionClient) GetExclusion(ctx context.Context, exclusionID string) (*itx.ExtendedExclusion, error) {
	if m.GetExclusionFunc != nil {
		return m.GetExclusionFunc(ctx, exclusionID)
	}
	panic("MockExclusionClient.GetExclusion: not configured")
}

func (m *MockExclusionClient) DeleteExclusionByID(ctx context.Context, exclusionID string) error {
	if m.DeleteExclusionByIDFunc != nil {
		return m.DeleteExclusionByIDFunc(ctx, exclusionID)
	}
	panic("MockExclusionClient.DeleteExclusionByID: not configured")
}
