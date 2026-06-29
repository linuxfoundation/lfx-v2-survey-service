// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	surveyconstants "github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

type stubSurveyInviteUserReader struct {
	username string
	err      error
}

func (s stubSurveyInviteUserReader) UsernameByEmail(_ context.Context, _ string) (string, error) {
	return s.username, s.err
}

type stubSurveyInviteSender struct {
	result *domain.InviteResult
	err    error
	called bool
	last   inviteapi.SendInviteRequest
}

func (s *stubSurveyInviteSender) SendInvite(_ context.Context, req inviteapi.SendInviteRequest) (*domain.InviteResult, error) {
	s.called = true
	s.last = req
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func TestDecodeKVData(t *testing.T) {
	t.Run("decodes JSON", func(t *testing.T) {
		payload := map[string]any{"name": "Member Survey 2025"}
		raw, err := json.Marshal(payload)
		require.NoError(t, err)

		got, err := decodeKVValue(raw)
		require.NoError(t, err)
		assert.Equal(t, "Member Survey 2025", got["name"])
	})

	t.Run("decodes msgpack", func(t *testing.T) {
		payload := map[string]any{"name": "Member Survey 2025"}
		raw, err := msgpack.Marshal(payload)
		require.NoError(t, err)

		got, err := decodeKVValue(raw)
		require.NoError(t, err)
		assert.Equal(t, "Member Survey 2025", got["name"])
	})

	t.Run("returns combined error when both decoders fail", func(t *testing.T) {
		_, err := decodeKVValue([]byte("not-json-or-msgpack"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "json:")
		assert.Contains(t, err.Error(), "msgpack:")
	})
}

func TestShouldSendSurveyResponseInvite(t *testing.T) {
	assert.True(t, shouldSendSurveyResponseInvite(indexerConstants.ActionCreated, "", "guest@example.com"))
	assert.False(t, shouldSendSurveyResponseInvite(indexerConstants.ActionUpdated, "", "guest@example.com"))
	assert.False(t, shouldSendSurveyResponseInvite(indexerConstants.ActionCreated, "existing", "guest@example.com"))
	assert.False(t, shouldSendSurveyResponseInvite(indexerConstants.ActionCreated, "", ""))
}

func TestMaybeSendSurveyInvite(t *testing.T) {
	const (
		surveyResponseUID = "response-123"
		surveyID          = "survey-456"
		email             = "guest@example.com"
	)

	inviteSentKey := surveyResponseLFIDInviteSentKey(surveyResponseUID)
	surveyKey := "itx-surveys." + surveyID
	surveyPayload, err := json.Marshal(map[string]any{"name": "Member Survey 2025"})
	require.NoError(t, err)

	tests := []struct {
		name         string
		userReader   stubSurveyInviteUserReader
		setupObjects func(*mockKeyValue)
		setupMaps    func(*mockKeyValue)
		wantCalled   bool
	}{
		{
			name: "skips when invite already sent",
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).
					Return(mockKeyValueEntry{key: inviteSentKey, value: []byte("invite-old")}, nil)
			},
		},
		{
			name:       "skips when participant already has LFID",
			userReader: stubSurveyInviteUserReader{username: "existing"},
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).Return(nil, jetstream.ErrKeyNotFound)
			},
		},
		{
			name:       "skips when survey name cannot be resolved",
			userReader: stubSurveyInviteUserReader{err: domain.ErrUserNotFound},
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).Return(nil, jetstream.ErrKeyNotFound)
			},
			setupObjects: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, surveyKey).Return(nil, jetstream.ErrKeyNotFound)
			},
		},
		{
			name:       "sends invite and stores sent marker on success",
			userReader: stubSurveyInviteUserReader{err: domain.ErrUserNotFound},
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).Return(nil, jetstream.ErrKeyNotFound)
				kv.On("Put", mock.Anything, inviteSentKey, []byte("pending")).Return(uint64(1), nil)
				kv.On("Put", mock.Anything, inviteSentKey, []byte("invite-new")).Return(uint64(2), nil)
			},
			setupObjects: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, surveyKey).
					Return(mockKeyValueEntry{key: surveyKey, value: surveyPayload}, nil)
			},
			wantCalled: true,
		},
		{
			name:       "proceeds with invite on transient auth lookup failure",
			userReader: stubSurveyInviteUserReader{err: errors.New("auth unavailable")},
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).Return(nil, jetstream.ErrKeyNotFound)
				kv.On("Put", mock.Anything, inviteSentKey, []byte("pending")).Return(uint64(1), nil)
				kv.On("Put", mock.Anything, inviteSentKey, []byte("invite-new")).Return(uint64(2), nil)
			},
			setupObjects: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, surveyKey).
					Return(mockKeyValueEntry{key: surveyKey, value: surveyPayload}, nil)
			},
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectsKV := &mockKeyValue{}
			mappingsKV := &mockKeyValue{}
			if tt.setupObjects != nil {
				tt.setupObjects(objectsKV)
			}
			if tt.setupMaps != nil {
				tt.setupMaps(mappingsKV)
			}

			sender := &stubSurveyInviteSender{
				result: &domain.InviteResult{
					InviteUID:      "invite-new",
					RecipientEmail: email,
					ExpiresAt:      time.Now().Add(24 * time.Hour),
				},
			}

			h := &SurveyResponseInviteHandler{
				v1ObjectsKV:      objectsKV,
				v1MappingsKV:     mappingsKV,
				userReader:       tt.userReader,
				inviteSender:     sender,
				selfServeBaseURL: "https://app.dev.lfx.dev",
			}

			h.maybeSendInvite(context.Background(), slog.Default(), surveyResponseUID, email, "Guest", surveyID)

			assert.Equal(t, tt.wantCalled, sender.called)
			if tt.wantCalled {
				assert.Equal(t, surveyconstants.InviteRoleParticipant, sender.last.Role)
				assert.Equal(t, surveyID, sender.last.Resource.UID)
				assert.Equal(t, surveyconstants.ResourceTypeSurvey, sender.last.Resource.Type)
				assert.Equal(t, "Member Survey 2025", sender.last.Resource.Name)
			}

			objectsKV.AssertExpectations(t)
			mappingsKV.AssertExpectations(t)
		})
	}
}
