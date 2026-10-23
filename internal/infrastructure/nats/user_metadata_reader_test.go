// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	surveyconstants "github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
)

func TestNATSUserNameReader_DisplayNameByUsername(t *testing.T) {
	tests := []struct {
		name     string
		reply    string
		replyErr error
		wantName string
		wantErr  error
		anyErr   bool
	}{
		{name: "returns the name field", reply: `{"success":true,"data":{"name":"Alice Example"}}`, wantName: "Alice Example"},
		{name: "falls back to given and family name", reply: `{"success":true,"data":{"given_name":"Alice","family_name":"Example"}}`, wantName: "Alice Example"},
		{name: "success with no name returns ErrUserNotFound", reply: `{"success":true,"data":{}}`, wantErr: domain.ErrUserNotFound},
		{name: "unsuccessful reply returns ErrUserNotFound", reply: `{"success":false,"error":"user not found"}`, wantErr: domain.ErrUserNotFound},
		{name: "reply without success field is an error", reply: `{"data":{"name":"Alice Example"}}`, anyErr: true},
		{name: "malformed reply is an error", reply: `not json`, anyErr: true},
		{name: "request failure is an error", replyErr: errors.New("timeout"), anyErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nc := &MockRequester{}
			if tc.replyErr != nil {
				nc.On("RequestWithContext", mock.Anything, surveyconstants.AuthUserMetadataSubject, []byte("alice")).Return(nil, tc.replyErr)
			} else {
				nc.On("RequestWithContext", mock.Anything, surveyconstants.AuthUserMetadataSubject, []byte("alice")).Return(replyMsg([]byte(tc.reply)), nil)
			}
			reader := NewUserNameReader(nc, slog.Default())

			name, err := reader.DisplayNameByUsername(context.Background(), "alice")

			switch {
			case tc.wantErr != nil:
				assert.ErrorIs(t, err, tc.wantErr)
			case tc.anyErr:
				require.Error(t, err)
				assert.NotErrorIs(t, err, domain.ErrUserNotFound)
			default:
				require.NoError(t, err)
				assert.Equal(t, tc.wantName, name)
			}
		})
	}

	t.Run("blank username returns ErrUserNotFound without calling NATS", func(t *testing.T) {
		nc := &MockRequester{}
		reader := NewUserNameReader(nc, slog.Default())

		_, err := reader.DisplayNameByUsername(context.Background(), "  ")

		assert.ErrorIs(t, err, domain.ErrUserNotFound)
		nc.AssertNotCalled(t, "RequestWithContext", mock.Anything, mock.Anything, mock.Anything)
	})
}
