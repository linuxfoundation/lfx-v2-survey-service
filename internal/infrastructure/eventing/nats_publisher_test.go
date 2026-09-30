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

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
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

func setupTestPublisher(t *testing.T, userReader domain.UserReader) (*NATSPublisher, *nats.Conn, func()) {
	ns, url := startTestNATSServer(t)

	nc, err := nats.Connect(url)
	require.NoError(t, err)

	publisher := NewNATSPublisher(nc, userReader, slog.Default())

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
			publisher, nc, cleanup := setupTestPublisher(t, &mocks.MockUserReader{Err: domain.ErrUserNotFound})
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

func TestSendSurveyResponseAccessMessageEmailFallback(t *testing.T) {
	tests := []struct {
		name             string
		data             *domain.SurveyResponseData
		readerUsername   string
		readerErr        error
		wantReaderCalled bool
		wantErrIs        []error // non-nil: lookup failure, no publish, event retries
		wantOwner        []string
		wantExcludeOwner bool
	}{
		{
			name: "resolves email to LFX username when username is empty",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			readerUsername:   "invitee",
			wantReaderCalled: true,
			wantOwner:        []string{"invitee"},
		},
		{
			name: "falls back to email when username is present but invalid",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Username:  "auth0|legacy",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			readerUsername:   "invitee",
			wantReaderCalled: true,
			wantOwner:        []string{"invitee"},
		},
		{
			name: "preserves owner when email does not resolve",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "ghost@example.com",
				SurveyUID: "survey-1",
			},
			readerErr:        domain.ErrUserNotFound,
			wantReaderCalled: true,
			wantExcludeOwner: true,
		},
		{
			name: "preserves owner when auth-service returns an empty username",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			readerUsername:   "",
			wantReaderCalled: true,
			wantExcludeOwner: true,
		},
		{
			name: "rejects an invalid username from auth-service as the owner principal",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			readerUsername:   "*",
			wantReaderCalled: true,
			wantExcludeOwner: true,
		},
		{
			name: "errors on auth-service backend failure so the event retries",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			readerErr:        errors.New("email_to_username failed: auth0 unavailable"),
			wantReaderCalled: true,
			wantErrIs:        []error{domain.ErrAuthServiceLookupFailed},
		},
		{
			name: "errors on transport failure and keeps the cause in the chain",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			readerErr:        fmt.Errorf("email_to_username request failed: %w", nats.ErrNoResponders),
			wantReaderCalled: true,
			wantErrIs:        []error{domain.ErrAuthServiceLookupFailed, nats.ErrNoResponders},
		},
		{
			name: "errors on context cancellation so the event retries",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			readerErr:        fmt.Errorf("email_to_username request failed: %w", context.Canceled),
			wantReaderCalled: true,
			wantErrIs:        []error{domain.ErrAuthServiceLookupFailed, context.Canceled},
		},
		{
			name: "does not call auth-service when username is present",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Username:  "testuser",
				Email:     "invitee@example.com",
				SurveyUID: "survey-1",
			},
			wantOwner: []string{"testuser"},
		},
		{
			name: "does not call auth-service for a whitespace-only email",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "   ",
				SurveyUID: "survey-1",
			},
			wantExcludeOwner: true,
		},
		{
			name: "does not call auth-service for a malformed email",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Email:     "not-an-email",
				SurveyUID: "survey-1",
			},
			wantExcludeOwner: true,
		},
		{
			name: "preserves owner when neither username nor email is present",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				SurveyUID: "survey-1",
			},
			wantExcludeOwner: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotEmails []string
			reader := &mocks.MockUserReader{
				UsernameByEmailFunc: func(_ context.Context, email string) (string, error) {
					gotEmails = append(gotEmails, email)
					return tt.readerUsername, tt.readerErr
				},
			}
			publisher, nc, cleanup := setupTestPublisher(t, reader)
			defer cleanup()

			sub, err := nc.SubscribeSync(fgaconstants.GenericUpdateAccessSubject)
			require.NoError(t, err)

			err = publisher.sendSurveyResponseAccessMessage(context.Background(), tt.data)

			if tt.wantReaderCalled {
				assert.Equal(t, []string{tt.data.Email}, gotEmails)
			} else {
				assert.Empty(t, gotEmails)
			}

			if tt.wantErrIs != nil {
				// Lookup failure must surface as an error (so the KV event is
				// NAKed and retried) and nothing may be published to fga-sync.
				require.Error(t, err)
				// The retry classification is structural: the error must carry the
				// sentinel that isTransientError (cmd/survey-api/eventing) matches,
				// and keep the original cause.
				for _, target := range tt.wantErrIs {
					assert.ErrorIs(t, err, target)
				}
				require.NoError(t, nc.Flush())
				_, msgErr := sub.NextMsg(100 * time.Millisecond)
				assert.ErrorIs(t, msgErr, nats.ErrTimeout)
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
		})
	}
}
