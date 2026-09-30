// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestHandleSurveyResponseUpdate_InviteOnPublishFailure verifies that when publishing
// fails because of an auth-service outage (ErrAuthServiceLookupFailed), the handler
// still attempts the best-effort LFID invite before NAKing — otherwise MaxDeliver
// exhaustion would strand account-less invitees with neither an owner tuple nor an
// invite. Other publish failures must not trigger the invite.
func TestHandleSurveyResponseUpdate_InviteOnPublishFailure(t *testing.T) {
	const (
		responseUID = "resp-1"
		surveyID    = "surv-1"
		email       = "guest@example.com"
	)

	surveyMappingKey := fmt.Sprintf("survey.%s", surveyID)
	responseMappingKey := fmt.Sprintf("survey_response.%s", responseUID)
	inviteSentKey := surveyResponseLFIDInviteSentKey(responseUID)
	surveyKey := fmt.Sprintf("itx-surveys.%s", surveyID)

	surveyPayload, err := json.Marshal(map[string]any{"name": "Member Survey 2025"})
	require.NoError(t, err)

	authOutageErr := fmt.Errorf("%w: resolve invitee email to LFX username: %w",
		domain.ErrAuthServiceLookupFailed, errors.New("auth0 unavailable"))

	v1Data := map[string]any{
		"id":         responseUID,
		"survey_id":  surveyID,
		"email":      email,
		"first_name": "Guest",
		"project":    map[string]any{"id": "proj-sfid", "name": "Proj"},
	}

	tests := []struct {
		name        string
		publishErr  error
		username    string
		setupInvite func(mappingsKV *mockKeyValue)
		wantRetry   bool
		wantInvite  bool
	}{
		{
			name:       "auth-service outage still attempts LFID invite before NAK",
			publishErr: authOutageErr,
			setupInvite: func(mappingsKV *mockKeyValue) {
				mappingsKV.On("Get", mock.Anything, inviteSentKey).Return(nil, jetstream.ErrKeyNotFound)
				mappingsKV.On("Put", mock.Anything, inviteSentKey, []byte("pending")).Return(uint64(1), nil)
				mappingsKV.On("Put", mock.Anything, inviteSentKey, []byte("invite-new")).Return(uint64(2), nil)
			},
			wantRetry:  true,
			wantInvite: true,
		},
		{
			name:        "non-auth transient publish error does not attempt invite",
			publishErr:  fmt.Errorf("failed to send survey response indexer message: %w", nats.ErrNoResponders),
			setupInvite: nil, // invite path must not be touched
			wantRetry:   true,
			wantInvite:  false,
		},
		{
			name:        "auth outage skips invite when participant already has a username",
			publishErr:  authOutageErr,
			username:    "existing-user",
			setupInvite: nil, // shouldSendSurveyResponseInvite is false
			wantRetry:   true,
			wantInvite:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{}
			for k, v := range v1Data {
				data[k] = v
			}
			if tt.username != "" {
				data["username"] = tt.username
			}

			mappingsKV := &mockKeyValue{}
			objectsKV := &mockKeyValue{}

			// Parent survey mapping exists (not tombstoned); response mapping absent
			// so the action is ActionCreated (invite-eligible).
			mappingsKV.On("Get", mock.Anything, surveyMappingKey).
				Return(mockKeyValueEntry{key: surveyMappingKey, value: []byte("mapping")}, nil)
			mappingsKV.On("Get", mock.Anything, responseMappingKey).Return(nil, jetstream.ErrKeyNotFound)
			// Parent survey payload serves both denormalization and the invite name lookup.
			objectsKV.On("Get", mock.Anything, surveyKey).
				Return(mockKeyValueEntry{key: surveyKey, value: surveyPayload}, nil)
			if tt.setupInvite != nil {
				tt.setupInvite(mappingsKV)
			}

			publisher := &mocks.MockEventPublisher{
				PublishSurveyResponseEventFunc: func(_ context.Context, _ string, _ *domain.SurveyResponseData) error {
					return tt.publishErr
				},
			}
			idMapper := &mocks.MockIDMapper{
				MapProjectV1ToV2Func: func(_ context.Context, _ string) (string, error) {
					return "proj-uid", nil
				},
			}
			sender := &stubSurveyInviteSender{
				result: &domain.InviteResult{
					InviteUID:      "invite-new",
					RecipientEmail: email,
					ExpiresAt:      time.Now().Add(24 * time.Hour),
				},
			}
			inviteHandler := &SurveyResponseInviteHandler{
				v1ObjectsKV:      objectsKV,
				v1MappingsKV:     mappingsKV,
				userReader:       stubSurveyInviteUserReader{err: domain.ErrUserNotFound},
				inviteSender:     sender,
				selfServeBaseURL: "https://app.dev.lfx.dev",
			}

			gotRetry := handleSurveyResponseUpdate(
				context.Background(),
				fmt.Sprintf("itx-survey-responses.%s", responseUID),
				data,
				publisher,
				idMapper,
				mappingsKV,
				objectsKV,
				inviteHandler,
				slog.Default(),
			)

			assert.Equal(t, tt.wantRetry, gotRetry)
			assert.Equal(t, tt.wantInvite, sender.called)
			mappingsKV.AssertExpectations(t)
			objectsKV.AssertExpectations(t)
		})
	}
}
