// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	surveyconstants "github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
)

func replyMsg(data []byte) *natsgo.Msg { return &natsgo.Msg{Data: data} }

func TestNATSUserReader_UsernameByEmail(t *testing.T) {
	tests := []struct {
		name       string
		email      string
		reply      *natsgo.Msg
		replyErr   error
		wantUser   string
		wantErr    error
		wantErrStr string
	}{
		{
			name:     "plain-text username returned on success",
			reply:    replyMsg([]byte("alice")),
			wantUser: "alice",
		},
		{
			name:     "trailing newline trimmed from username",
			reply:    replyMsg([]byte("alice\n")),
			wantUser: "alice",
		},
		{
			name:    "empty body returns ErrUserNotFound",
			reply:   replyMsg([]byte("")),
			wantErr: domain.ErrUserNotFound,
		},
		{
			name:    "JSON error envelope without error field returns ErrUserNotFound",
			reply:   replyMsg([]byte(`{"success":false}`)),
			wantErr: domain.ErrUserNotFound,
		},
		{
			name:       "JSON error envelope with error field returns service error",
			reply:      replyMsg([]byte(`{"success":false,"error":"auth service unavailable"}`)),
			wantErrStr: "email_to_username failed: auth service unavailable",
		},
		{
			name:       "email addresses in service error text are redacted",
			reply:      replyMsg([]byte(`{"success":false,"error":"failed to search user: Get \"https://auth0.example/users-by-email?email=invitee%40example.com\": dial tcp: i/o timeout (invitee@example.com)"}`)),
			wantErrStr: "email=[redacted-email]\": dial tcp: i/o timeout ([redacted-email])",
		},
		{
			name:       "quoted email address in service error text is redacted",
			email:      `"john doe"@example.com`,
			reply:      replyMsg([]byte(`{"success":false,"error":"invalid address \"john doe\"@example.com rejected"}`)),
			wantErrStr: "invalid address [redacted-email] rejected",
		},
		{
			name:       "unicode email address in service error text is redacted",
			email:      "用户@example.com",
			reply:      replyMsg([]byte(`{"success":false,"error":"failed to search user 用户@example.com: 500 Internal Server Error"}`)),
			wantErrStr: "failed to search user [redacted-email]: 500 Internal Server Error",
		},
		{
			name:    "JSON user-not-found envelope returns ErrUserNotFound",
			reply:   replyMsg([]byte(`{"success":false,"error":"user not found"}`)),
			wantErr: domain.ErrUserNotFound,
		},
		{
			name:    "JSON user-not-found-by-criteria envelope returns ErrUserNotFound",
			reply:   replyMsg([]byte(`{"success":false,"error":"user not found by criteria"}`)),
			wantErr: domain.ErrUserNotFound,
		},
		{
			name:       "backend failure mentioning not found returns service error",
			reply:      replyMsg([]byte(`{"success":false,"error":"failed to search user: 404 Not Found"}`)),
			wantErrStr: "email_to_username failed: failed to search user: 404 Not Found",
		},
		{
			name:       "JSON envelope missing success field returns descriptive error",
			reply:      replyMsg([]byte(`{"error":"something unexpected"}`)),
			wantErrStr: "email_to_username response missing success field",
		},
		{
			name:       "JSON success envelope returns unexpected envelope error",
			reply:      replyMsg([]byte(`{"success":true,"username":"alice"}`)),
			wantErrStr: "unexpected email_to_username success envelope",
		},
		{
			name:       "malformed JSON object returns parse error",
			reply:      replyMsg([]byte(`{"success":"true"}`)),
			wantErrStr: "failed to parse email_to_username response",
		},
		{
			name:       "transport error is wrapped and returned",
			reply:      nil,
			replyErr:   errors.New("nats: connection closed"),
			wantErrStr: "email_to_username request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &MockRequester{}
			mockConn.On("RequestWithContext", mock.Anything, surveyconstants.AuthEmailToUsernameSubject, mock.Anything).
				Return(tt.reply, tt.replyErr)

			email := tt.email
			if email == "" {
				email = "test@example.com"
			}

			reader := NewUserReader(mockConn, slog.Default())
			got, err := reader.UsernameByEmail(context.Background(), email)

			switch {
			case tt.wantErr != nil:
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
			case tt.wantErrStr != "":
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrStr)
				assert.Empty(t, got)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.wantUser, got)
			}
			mockConn.AssertExpectations(t)
		})
	}
}
