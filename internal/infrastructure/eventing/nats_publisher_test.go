// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
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
		name          string
		data          *domain.SurveyResponseData
		wantPublished bool
		wantOwner     []string
		wantSurveyRef []string
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
			name: "omits owner relation for empty username",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				SurveyUID: "survey-1",
			},
			wantPublished: true,
			wantOwner:     nil,
			wantSurveyRef: []string{"survey-1"},
		},
		{
			name: "skips owner relation for invalid username",
			data: &domain.SurveyResponseData{
				UID:       "sr-1",
				Username:  "auth0|legacy",
				SurveyUID: "survey-1",
			},
			wantPublished: true,
			wantOwner:     nil,
			wantSurveyRef: []string{"survey-1"},
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
		})
	}
}

func TestBuildHeaders_TypedContextKeys(t *testing.T) {
	p := &NATSPublisher{}

	t.Run("fallback when context is empty", func(t *testing.T) {
		headers := p.buildHeaders(context.Background())
		assert.Equal(t, "Bearer survey-service", headers["authorization"])
		assert.Empty(t, headers["x-on-behalf-of"])
	})

	t.Run("extracts typed authorization and principal keys", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), constants.AuthorizationContextID, "Bearer real-token")
		ctx = context.WithValue(ctx, constants.PrincipalContextID, "user-123")

		headers := p.buildHeaders(ctx)
		assert.Equal(t, "Bearer real-token", headers["authorization"])
		assert.Equal(t, "user-123", headers["x-on-behalf-of"])
	})

	t.Run("untyped string keys do NOT match (regression guard)", func(t *testing.T) {
		// Storing under plain string must NOT be picked up — only the typed constant works.
		type plainKey string
		ctx := context.WithValue(context.Background(), plainKey("authorization"), "Bearer wrong")
		ctx = context.WithValue(ctx, plainKey("principal"), "wrong-user")

		headers := p.buildHeaders(ctx)
		assert.Equal(t, "Bearer survey-service", headers["authorization"], "plain-string key must not match typed contextKey")
		assert.Empty(t, headers["x-on-behalf-of"], "plain-string key must not match typed contextKey")
	})
}
