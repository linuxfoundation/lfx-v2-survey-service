// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startTestNATSServer(t *testing.T) (*server.Server, string) {
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
	}

	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()

	if !ns.ReadyForConnections(4 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	return ns, ns.ClientURL()
}

func setupTestPublisher(t *testing.T) (*NATSPublisher, *nats.Conn, func()) {
	ns, url := startTestNATSServer(t)

	nc, err := nats.Connect(url)
	require.NoError(t, err)

	publisher := NewNATSPublisher(nc, slog.Default())

	cleanup := func() {
		nc.Close()
		ns.Shutdown()
	}

	return publisher, nc, cleanup
}

func TestIsValidLFXUsername(t *testing.T) {
	tests := []struct {
		username string
		want     bool
	}{
		{username: "testuser", want: true},
		{username: "user.name-123", want: true},
		{username: "", want: false},
		{username: "auth0|user", want: false},
		{username: "bad:user", want: false},
		{username: "has*star", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidLFXUsername(tt.username))
		})
	}
}

func TestSendSurveyResponseAccessMessage(t *testing.T) {
	tests := []struct {
		name             string
		data             *domain.SurveyResponseData
		wantPublished    bool
		wantOwner        []string
		wantSurveyRef    []string
		wantExcludeOwner bool
	}{
		{
			name: "sets owner relation for valid username",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Username:  "testuser",
				SurveyUID: "survey-1",
			},
			wantPublished: true,
			wantOwner:     []string{"testuser"},
			wantSurveyRef: []string{"survey-1"},
		},
		{
			name: "omits owner relation and preserves existing owner for empty username",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				SurveyUID: "survey-1",
			},
			wantPublished:    true,
			wantOwner:        nil,
			wantSurveyRef:    []string{"survey-1"},
			wantExcludeOwner: true,
		},
		{
			name: "skips owner relation and preserves existing owner for invalid username",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Username:  "auth0|legacy",
				SurveyUID: "survey-1",
			},
			wantPublished:    true,
			wantOwner:        nil,
			wantSurveyRef:    []string{"survey-1"},
			wantExcludeOwner: true,
		},
		{
			name: "skips publish when username and survey UID are empty",
			data: &domain.SurveyResponseData{
				UID: "sr-1",
			},
			wantPublished: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher, nc, cleanup := setupTestPublisher(t)
			defer cleanup()

			sub, err := nc.SubscribeSync(fgaconstants.GenericUpdateAccessSubject)
			require.NoError(t, err)

			err = publisher.sendSurveyResponseAccessMessage(context.Background(), tt.data)
			require.NoError(t, err)
			require.NoError(t, nc.Flush())

			msg, err := sub.NextMsg(250 * time.Millisecond)
			if !tt.wantPublished {
				assert.ErrorIs(t, err, nats.ErrTimeout)
				return
			}

			require.NoError(t, err)

			var accessMsg fgatypes.GenericFGAMessage
			err = json.Unmarshal(msg.Data, &accessMsg)
			require.NoError(t, err)

			assert.Equal(t, "survey_response", accessMsg.ObjectType)
			assert.Equal(t, "update_access", accessMsg.Operation)

			var accessData fgatypes.GenericAccessData
			err = accessMsg.UnmarshalData(&accessData)
			require.NoError(t, err)

			assert.Equal(t, tt.data.UID, accessData.UID)
			assert.Equal(t, tt.wantOwner, accessData.Relations["owner"])
			assert.Equal(t, tt.wantSurveyRef, accessData.References["survey"])
			if tt.wantExcludeOwner {
				assert.Contains(t, accessData.ExcludeRelations, "owner")
			} else {
				assert.Empty(t, accessData.ExcludeRelations)
			}
		})
	}
}

// respondAsAuthService installs a fake auth-service responder on the
// email_to_sub subject and returns a counter of requests it received.
func respondAsAuthService(t *testing.T, nc *nats.Conn, reply []byte) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	sub, err := nc.Subscribe(authServiceEmailToSubSubject, func(msg *nats.Msg) {
		calls.Add(1)
		_ = msg.Respond(reply)
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	return &calls
}

func TestSendSurveyResponseAccessMessageEmailFallback(t *testing.T) {
	handlerErrorEnvelope := []byte(`{"success":false,"error":"user not found"}`)
	transportNotFoundEnvelope := []byte(`{"error":"no user with that email"}`)
	transportErrorEnvelope := []byte(`{"error":"auth0 unavailable"}`)
	malformedEnvelope := []byte(`{not json`)

	tests := []struct {
		name             string
		data             *domain.SurveyResponseData
		authReply        []byte // nil = no auth-service responder installed
		wantAuthCalled   bool
		wantErr          bool // transient/unknown failure: no publish, event retries
		wantOwner        []string
		wantExcludeOwner bool
	}{
		{
			name: "resolves email to auth sub when username is empty",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			authReply:      []byte("auth0|abc123"),
			wantAuthCalled: true,
			wantOwner:      []string{"auth0|abc123"},
		},
		{
			name: "falls back to email when username is present but invalid",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Username:  "auth0|legacy",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			authReply:      []byte("auth0|abc123"),
			wantAuthCalled: true,
			wantOwner:      []string{"auth0|abc123"},
		},
		{
			name: "preserves owner when email does not resolve",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "ghost@example.com",
				SurveyUID: "survey-1",
			},
			authReply:        handlerErrorEnvelope,
			wantAuthCalled:   true,
			wantOwner:        nil,
			wantExcludeOwner: true,
		},
		{
			name: "preserves owner on transport-level not-found envelope",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "ghost@example.com",
				SurveyUID: "survey-1",
			},
			authReply:        transportNotFoundEnvelope,
			wantAuthCalled:   true,
			wantOwner:        nil,
			wantExcludeOwner: true,
		},
		{
			name: "errors on transient auth-service backend failure so the event retries",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			authReply:      transportErrorEnvelope,
			wantAuthCalled: true,
			wantErr:        true,
		},
		{
			name: "errors on unrecognized reply envelope so the event retries",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			authReply:      malformedEnvelope,
			wantAuthCalled: true,
			wantErr:        true,
		},
		{
			name: "does not call auth-service when username is present",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Username:  "testuser",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			authReply:      handlerErrorEnvelope, // must never be sent a request
			wantAuthCalled: false,
			wantOwner:      []string{"testuser"},
		},
		{
			name: "preserves owner when neither username nor email is present",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				SurveyUID: "survey-1",
			},
			authReply:        nil,
			wantAuthCalled:   false,
			wantOwner:        nil,
			wantExcludeOwner: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher, nc, cleanup := setupTestPublisher(t)
			defer cleanup()

			var authCalls *atomic.Int32
			if tt.authReply != nil {
				authCalls = respondAsAuthService(t, nc, tt.authReply)
			}

			sub, err := nc.SubscribeSync(fgaconstants.GenericUpdateAccessSubject)
			require.NoError(t, err)

			err = publisher.sendSurveyResponseAccessMessage(context.Background(), tt.data)
			if tt.wantErr {
				// Lookup failure must surface as an error (so the KV event is
				// NAKed and retried) and nothing may be published to fga-sync.
				require.Error(t, err)
				require.NoError(t, nc.Flush())
				_, msgErr := sub.NextMsg(100 * time.Millisecond)
				assert.ErrorIs(t, msgErr, nats.ErrTimeout)
				if tt.authReply != nil && tt.wantAuthCalled {
					assert.Greater(t, authCalls.Load(), int32(0))
				}
				return
			}
			require.NoError(t, err)
			require.NoError(t, nc.Flush())

			// All non-error rows publish: the survey reference alone is enough to send.
			msg, err := sub.NextMsg(250 * time.Millisecond)
			require.NoError(t, err)

			var accessMsg fgatypes.GenericFGAMessage
			err = json.Unmarshal(msg.Data, &accessMsg)
			require.NoError(t, err)

			var accessData fgatypes.GenericAccessData
			err = accessMsg.UnmarshalData(&accessData)
			require.NoError(t, err)

			assert.Equal(t, tt.wantOwner, accessData.Relations["owner"])
			if tt.wantExcludeOwner {
				assert.Contains(t, accessData.ExcludeRelations, "owner")
			} else {
				assert.Empty(t, accessData.ExcludeRelations)
			}
			if tt.authReply != nil {
				if tt.wantAuthCalled {
					assert.Greater(t, authCalls.Load(), int32(0))
				} else {
					assert.Equal(t, int32(0), authCalls.Load())
				}
			}
		})
	}
}

func TestSendSurveyResponseAccessMessageAuthServiceUnavailable(t *testing.T) {
	publisher, nc, cleanup := setupTestPublisher(t)
	defer cleanup()

	sub, err := nc.SubscribeSync(fgaconstants.GenericUpdateAccessSubject)
	require.NoError(t, err)

	// No auth-service responder: the lookup must fail (so the event can be
	// retried) rather than silently dropping the owner grant.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = publisher.sendSurveyResponseAccessMessage(ctx, &domain.SurveyResponseData{
		UID:       "sr-1",
		Email:     "invitee@example.com",
		SurveyUID: "survey-1",
	})
	require.Error(t, err)

	// Nothing should have been published to fga-sync.
	_, err = sub.NextMsg(100 * time.Millisecond)
	assert.ErrorIs(t, err, nats.ErrTimeout)
}
