// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// inviteRequest is a request whose every field has a distinct value, so a
// swapped pair of fields in the wire mapping cannot pass.
func inviteRequest() domain.InviteRequest {
	return domain.InviteRequest{
		Recipient:      domain.InviteRecipient{Email: "alice@example.com", Name: "Alice Example"},
		Resource:       domain.InviteResource{UID: "survey-1", Name: "Q4 NPS Survey", Type: "survey"},
		Role:           "participant",
		ReturnURL:      "https://app.example.com/surveys/survey-1",
		ExpirationDays: 30,
	}
}

// inviteReply builds a reply message from a JSON-encodable body.
func inviteReply(t *testing.T, body any) *natsgo.Msg {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	return &natsgo.Msg{Data: data}
}

func TestNATSInviteSender_SendInvite(t *testing.T) {
	expires := time.Date(2030, 3, 1, 12, 0, 0, 0, time.UTC)

	t.Run("sends the wire request and returns the invite metadata", func(t *testing.T) {
		var gotCtx context.Context
		var gotPayload []byte
		requester := new(MockRequester)
		requester.On("RequestWithContext", mock.Anything, inviteapi.SendInviteSubject, mock.Anything).
			Run(func(args mock.Arguments) {
				gotCtx = args.Get(0).(context.Context)
				gotPayload = args.Get(2).([]byte)
			}).
			Return(inviteReply(t, inviteapi.SendInviteResponse{
				InviteData: &inviteapi.InviteData{UID: "invite-1", Email: "reply@example.com", ExpiresAt: expires},
			}), nil)

		start := time.Now()
		result, err := NewInviteSender(requester, slog.Default()).SendInvite(context.Background(), inviteRequest())

		require.NoError(t, err)
		assert.Equal(t, &domain.InviteResult{InviteUID: "invite-1", RecipientEmail: "reply@example.com", ExpiresAt: expires}, result)

		var wire inviteapi.SendInviteRequest
		require.NoError(t, json.Unmarshal(gotPayload, &wire))
		require.NotNil(t, wire.Recipient)
		require.NotNil(t, wire.Resource)
		assert.Equal(t, "alice@example.com", wire.Recipient.Email)
		assert.Equal(t, "Alice Example", wire.Recipient.Name)
		assert.Equal(t, "survey-1", wire.Resource.UID)
		assert.Equal(t, "Q4 NPS Survey", wire.Resource.Name)
		assert.Equal(t, "survey", wire.Resource.Type)
		assert.Equal(t, "participant", wire.Role)
		assert.Equal(t, "https://app.example.com/surveys/survey-1", wire.ReturnURL)
		assert.Equal(t, 30, wire.ExpirationDays)

		deadline, ok := gotCtx.Deadline()
		require.True(t, ok, "the request is bounded by a timeout")
		assert.WithinDuration(t, start.Add(inviteSenderTimeout), deadline, 5*time.Second)
		requester.AssertExpectations(t)
	})

	t.Run("a request failure is wrapped", func(t *testing.T) {
		boom := errors.New("no responders")
		requester := new(MockRequester)
		requester.On("RequestWithContext", mock.Anything, inviteapi.SendInviteSubject, mock.Anything).Return(nil, boom)

		result, err := NewInviteSender(requester, slog.Default()).SendInvite(context.Background(), inviteRequest())

		assert.Nil(t, result)
		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.Contains(t, err.Error(), "invite service request failed")
	})

	t.Run("an unparsable reply is an error", func(t *testing.T) {
		requester := new(MockRequester)
		requester.On("RequestWithContext", mock.Anything, inviteapi.SendInviteSubject, mock.Anything).
			Return(&natsgo.Msg{Data: []byte("not json")}, nil)

		result, err := NewInviteSender(requester, slog.Default()).SendInvite(context.Background(), inviteRequest())

		assert.Nil(t, result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse invite service response")
	})

	t.Run("an error reply is surfaced with its message", func(t *testing.T) {
		requester := new(MockRequester)
		requester.On("RequestWithContext", mock.Anything, inviteapi.SendInviteSubject, mock.Anything).
			Return(inviteReply(t, inviteapi.SendInviteResponse{Error: "recipient already has an LFID"}), nil)

		result, err := NewInviteSender(requester, slog.Default()).SendInvite(context.Background(), inviteRequest())

		assert.Nil(t, result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invite service returned error: recipient already has an LFID")
	})

	t.Run("a reply with neither data nor an error is an error", func(t *testing.T) {
		requester := new(MockRequester)
		requester.On("RequestWithContext", mock.Anything, inviteapi.SendInviteSubject, mock.Anything).
			Return(&natsgo.Msg{Data: []byte(`{}`)}, nil)

		result, err := NewInviteSender(requester, slog.Default()).SendInvite(context.Background(), inviteRequest())

		assert.Nil(t, result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty response")
	})
}
