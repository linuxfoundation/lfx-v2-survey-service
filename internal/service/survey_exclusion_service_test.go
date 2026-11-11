// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newExclusionService(client *mocks.MockExclusionClient, mapper *mocks.MockIDMapper) *service.SurveyService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return service.NewSurveyService(&mocks.MockSurveyClient{}, client, &mocks.MockSurveyResponseClient{}, mapper, logger)
}

func TestCreateExclusion(t *testing.T) {
	var sent *itx.ExclusionRequest
	var respond *itx.Exclusion
	client := &mocks.MockExclusionClient{
		CreateExclusionFunc: func(_ context.Context, req *itx.ExclusionRequest) (*itx.Exclusion, error) {
			sent = req
			return respond, nil
		},
	}
	create := func(p *survey.CreateExclusionPayload, mapper *mocks.MockIDMapper) (*survey.ExclusionResult, error) {
		sent = nil
		return newExclusionService(client, mapper).CreateExclusion(ctxWithPrincipal("alice"), p)
	}

	t.Run("maps the request and the response", func(t *testing.T) {
		respond = &itx.Exclusion{
			ID: "ex-1", Email: strPtr("carol@example.com"), SurveyID: strPtr("s-1"),
			CommitteeID: strPtr("c1"), GlobalExclusion: strPtr("false"), UserID: strPtr("u-1"),
		}

		got, err := create(&survey.CreateExclusionPayload{
			Email: strPtr("carol@example.com"), UserID: strPtr("u-1"), SurveyUID: strPtr("s-1"),
			CommitteeUID: strPtr("c-uid"), GlobalExclusion: strPtr("false"),
		}, prefixMapper("", ""))

		require.NoError(t, err)
		assert.Equal(t, &itx.ExclusionRequest{
			Email: strPtr("carol@example.com"), UserID: strPtr("u-1"), SurveyID: strPtr("s-1"),
			CommitteeID: strPtr("v1-c-uid"), GlobalExclusion: strPtr("false"),
		}, sent)
		assert.Equal(t, &survey.ExclusionResult{
			UID: "ex-1", Email: strPtr("carol@example.com"), SurveyUID: strPtr("s-1"),
			CommitteeUID: strPtr("v2-c1"), GlobalExclusion: strPtr("false"), UserID: strPtr("u-1"),
		}, got)
	})

	t.Run("a global exclusion without a committee sends none", func(t *testing.T) {
		respond = &itx.Exclusion{ID: "ex-2", GlobalExclusion: strPtr("true")}

		got, err := create(&survey.CreateExclusionPayload{Email: strPtr("carol@example.com"), GlobalExclusion: strPtr("true")}, prefixMapper("", ""))

		require.NoError(t, err)
		assert.Nil(t, sent.CommitteeID)
		assert.Nil(t, got.CommitteeUID)
	})

	t.Run("response committee keeps its v1 id when mapping fails", func(t *testing.T) {
		respond = &itx.Exclusion{ID: "ex-3", CommitteeID: strPtr("c-bad")}

		got, err := create(&survey.CreateExclusionPayload{Email: strPtr("carol@example.com")}, prefixMapper("", "c-bad"))

		require.NoError(t, err)
		assert.Equal(t, "c-bad", *got.CommitteeUID)
	})

	t.Run("request committee mapping failure is returned and ITX is not called", func(t *testing.T) {
		_, err := create(&survey.CreateExclusionPayload{CommitteeUID: strPtr("c-bad")}, prefixMapper("c-bad", ""))

		assert.ErrorIs(t, err, errMapper)
		assert.Nil(t, sent)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewConflictError("already excluded")
		failing := newExclusionService(&mocks.MockExclusionClient{
			CreateExclusionFunc: func(context.Context, *itx.ExclusionRequest) (*itx.Exclusion, error) { return nil, itxErr },
		}, prefixMapper("", ""))

		got, err := failing.CreateExclusion(ctxWithPrincipal("alice"), &survey.CreateExclusionPayload{Email: strPtr("carol@example.com")})

		assert.Nil(t, got)
		assert.Same(t, itxErr, err)
	})
}

func TestDeleteExclusion(t *testing.T) {
	var sent *itx.ExclusionRequest
	client := &mocks.MockExclusionClient{
		DeleteExclusionFunc: func(_ context.Context, req *itx.ExclusionRequest) error {
			sent = req
			return nil
		},
	}
	svc := newExclusionService(client, prefixMapper("c-bad", ""))

	t.Run("maps the committee and forwards the other fields", func(t *testing.T) {
		sent = nil
		err := svc.DeleteExclusion(ctxWithPrincipal("alice"), &survey.DeleteExclusionPayload{
			Email: strPtr("carol@example.com"), UserID: strPtr("u-1"), SurveyUID: strPtr("s-1"),
			CommitteeUID: strPtr("c-uid"), GlobalExclusion: strPtr("false"),
		})

		require.NoError(t, err)
		assert.Equal(t, &itx.ExclusionRequest{
			Email: strPtr("carol@example.com"), UserID: strPtr("u-1"), SurveyID: strPtr("s-1"),
			CommitteeID: strPtr("v1-c-uid"), GlobalExclusion: strPtr("false"),
		}, sent)
	})

	t.Run("without a committee none is sent", func(t *testing.T) {
		sent = nil
		require.NoError(t, svc.DeleteExclusion(ctxWithPrincipal("alice"), &survey.DeleteExclusionPayload{Email: strPtr("carol@example.com")}))
		assert.Nil(t, sent.CommitteeID)
	})

	t.Run("committee mapping failure is returned and ITX is not called", func(t *testing.T) {
		sent = nil
		err := svc.DeleteExclusion(ctxWithPrincipal("alice"), &survey.DeleteExclusionPayload{CommitteeUID: strPtr("c-bad")})

		assert.ErrorIs(t, err, errMapper)
		assert.Nil(t, sent)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewNotFoundError("no such exclusion")
		failing := newExclusionService(&mocks.MockExclusionClient{
			DeleteExclusionFunc: func(context.Context, *itx.ExclusionRequest) error { return itxErr },
		}, prefixMapper("", ""))

		assert.Same(t, itxErr, failing.DeleteExclusion(ctxWithPrincipal("alice"), &survey.DeleteExclusionPayload{}))
	})
}

func TestGetExclusion(t *testing.T) {
	var gotID string
	var respond *itx.ExtendedExclusion
	client := &mocks.MockExclusionClient{
		GetExclusionFunc: func(_ context.Context, id string) (*itx.ExtendedExclusion, error) {
			gotID = id
			return respond, nil
		},
	}
	get := func(mapper *mocks.MockIDMapper) (*survey.ExtendedExclusionResult, error) {
		return newExclusionService(client, mapper).GetExclusion(ctxWithPrincipal("alice"), &survey.GetExclusionPayload{ExclusionID: "ex-1"})
	}

	t.Run("maps the exclusion, its committee and the user with all emails", func(t *testing.T) {
		respond = &itx.ExtendedExclusion{
			Exclusion: itx.Exclusion{
				ID: "ex-1", Email: strPtr("carol@example.com"), SurveyID: strPtr("s-1"),
				CommitteeID: strPtr("c1"), GlobalExclusion: strPtr("false"), UserID: strPtr("u-1"),
			},
			User: &itx.ExclusionUser{
				ID: strPtr("u-1"), Username: strPtr("carol"),
				Emails: []itx.UserEmail{
					{ID: strPtr("e-1"), EmailAddress: strPtr("carol@example.com"), IsPrimary: boolPtr(true)},
					{ID: strPtr("e-2"), EmailAddress: strPtr("carol@example.org"), IsPrimary: boolPtr(false)},
				},
			},
		}

		got, err := get(prefixMapper("", ""))

		require.NoError(t, err)
		assert.Equal(t, "ex-1", gotID)
		assert.Equal(t, &survey.ExtendedExclusionResult{
			UID: "ex-1", Email: strPtr("carol@example.com"), SurveyUID: strPtr("s-1"),
			CommitteeUID: strPtr("v2-c1"), GlobalExclusion: strPtr("false"), UserID: strPtr("u-1"),
			User: &survey.ExclusionUser{
				ID: strPtr("u-1"), Username: strPtr("carol"),
				Emails: []*survey.UserEmail{
					{ID: strPtr("e-1"), EmailAddress: strPtr("carol@example.com"), IsPrimary: boolPtr(true)},
					{ID: strPtr("e-2"), EmailAddress: strPtr("carol@example.org"), IsPrimary: boolPtr(false)},
				},
			},
		}, got)
	})

	t.Run("without a user the result has no user", func(t *testing.T) {
		respond = &itx.ExtendedExclusion{Exclusion: itx.Exclusion{ID: "ex-1"}}

		got, err := get(prefixMapper("", ""))

		require.NoError(t, err)
		assert.Nil(t, got.User)
		assert.Nil(t, got.CommitteeUID)
	})

	t.Run("a user with no emails gets an empty non-nil list", func(t *testing.T) {
		respond = &itx.ExtendedExclusion{Exclusion: itx.Exclusion{ID: "ex-1"}, User: &itx.ExclusionUser{ID: strPtr("u-1")}}

		got, err := get(prefixMapper("", ""))

		require.NoError(t, err)
		require.NotNil(t, got.User)
		assert.NotNil(t, got.User.Emails)
		assert.Empty(t, got.User.Emails)
	})

	t.Run("committee keeps its v1 id when mapping fails", func(t *testing.T) {
		respond = &itx.ExtendedExclusion{Exclusion: itx.Exclusion{ID: "ex-1", CommitteeID: strPtr("c-bad")}}

		got, err := get(prefixMapper("", "c-bad"))

		require.NoError(t, err)
		assert.Equal(t, "c-bad", *got.CommitteeUID)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewNotFoundError("no such exclusion")
		failing := newExclusionService(&mocks.MockExclusionClient{
			GetExclusionFunc: func(context.Context, string) (*itx.ExtendedExclusion, error) { return nil, itxErr },
		}, prefixMapper("", ""))

		got, err := failing.GetExclusion(ctxWithPrincipal("alice"), &survey.GetExclusionPayload{ExclusionID: "ex-1"})

		assert.Nil(t, got)
		assert.Same(t, itxErr, err)
	})
}

func TestDeleteExclusionByID(t *testing.T) {
	t.Run("forwards the exclusion id", func(t *testing.T) {
		var gotID string
		svc := newExclusionService(&mocks.MockExclusionClient{
			DeleteExclusionByIDFunc: func(_ context.Context, id string) error { gotID = id; return nil },
		}, prefixMapper("", ""))

		require.NoError(t, svc.DeleteExclusionByID(ctxWithPrincipal("alice"), &survey.DeleteExclusionByIDPayload{ExclusionID: "ex-1"}))
		assert.Equal(t, "ex-1", gotID)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewNotFoundError("no such exclusion")
		svc := newExclusionService(&mocks.MockExclusionClient{
			DeleteExclusionByIDFunc: func(context.Context, string) error { return itxErr },
		}, prefixMapper("", ""))

		assert.Same(t, itxErr, svc.DeleteExclusionByID(ctxWithPrincipal("alice"), &survey.DeleteExclusionByIDPayload{ExclusionID: "ex-1"}))
	})
}
