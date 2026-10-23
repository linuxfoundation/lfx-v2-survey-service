// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/idmapper"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

func ctxWithPrincipal(principal string) context.Context {
	return context.WithValue(context.Background(), constants.PrincipalContextID, principal)
}

func newCreatorTestService(client *mocks.MockSurveyClient, names domain.UserNameReader) *service.SurveyService {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := service.NewSurveyService(client, &mocks.MockExclusionClient{}, &mocks.MockSurveyResponseClient{}, idmapper.NewNoOpMapper(), logger)
	if names != nil {
		svc.WithUserNameReader(names)
	}
	return svc
}

func TestScheduleSurvey_CreatorFromPrincipal(t *testing.T) {
	forgedPayload := func() *survey.ScheduleSurveyPayload {
		return &survey.ScheduleSurveyPayload{
			CommitteeUID:    "committee-1",
			CreatorUsername: strPtr("victim"),
			CreatorName:     strPtr("Victim Fixture"),
			CreatorID:       strPtr("victim-id"),
		}
	}

	var sent *itx.ScheduleSurveyRequest
	client := &mocks.MockSurveyClient{
		ScheduleSurveyFunc: func(_ context.Context, req *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
			sent = req
			return &itx.SurveyScheduleResponse{}, nil
		},
	}

	t.Run("stamps username, id and looked-up name over forged body values", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, &mocks.MockUserNameReader{Name: "Alice Example"})

		_, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), forgedPayload())

		require.NoError(t, err)
		require.NotNil(t, sent)
		assert.Equal(t, "alice", *sent.CreatorUsername)
		assert.Equal(t, "alice", *sent.CreatorID)
		assert.Equal(t, "Alice Example", *sent.CreatorName)
	})

	t.Run("falls back to the username when the name lookup fails", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, &mocks.MockUserNameReader{Err: errors.New("nats unavailable")})

		_, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), forgedPayload())

		require.NoError(t, err)
		assert.Equal(t, "alice", *sent.CreatorName, "forged name must not be used when the lookup fails")
	})

	t.Run("falls back to the username when no name reader is configured", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, nil)

		_, err := svc.ScheduleSurvey(ctxWithPrincipal("alice"), forgedPayload())

		require.NoError(t, err)
		assert.Equal(t, "alice", *sent.CreatorName)
	})

	t.Run("rejects the request without an authenticated principal", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, &mocks.MockUserNameReader{Name: "Alice Example"})

		_, err := svc.ScheduleSurvey(context.Background(), forgedPayload())

		var domErr *domain.DomainError
		require.ErrorAs(t, err, &domErr)
		assert.Equal(t, domain.ErrorTypeValidation, domErr.Type)
		assert.Nil(t, sent, "ITX must not be called")
	})
}

func TestUpdateSurvey_CreatorFromPrincipal(t *testing.T) {
	var sent *itx.UpdateSurveyRequest
	client := &mocks.MockSurveyClient{
		UpdateSurveyFunc: func(_ context.Context, _ string, req *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
			sent = req
			return &itx.SurveyScheduleResponse{}, nil
		},
	}

	t.Run("stamps username and id from the principal over a forged creator_id", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, nil)

		_, err := svc.UpdateSurvey(ctxWithPrincipal("alice"), &survey.UpdateSurveyPayload{
			SurveyUID: "survey-1",
			CreatorID: strPtr("victim-id"),
		})

		require.NoError(t, err)
		require.NotNil(t, sent)
		assert.Equal(t, "alice", *sent.CreatorUsername)
		assert.Equal(t, "alice", *sent.CreatorID)
	})

	t.Run("rejects the request without an authenticated principal", func(t *testing.T) {
		sent = nil
		svc := newCreatorTestService(client, nil)

		_, err := svc.UpdateSurvey(context.Background(), &survey.UpdateSurveyPayload{SurveyUID: "survey-1"})

		var domErr *domain.DomainError
		require.ErrorAs(t, err, &domErr)
		assert.Equal(t, domain.ErrorTypeValidation, domErr.Type)
		assert.Nil(t, sent, "ITX must not be called")
	})
}
