// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// acceptedEvent builds an invite_accepted event for the given acceptor.
func acceptedEvent(email, username string) inviteapi.InviteServiceAcceptedEvent {
	return inviteapi.InviteServiceAcceptedEvent{Invite: inviteapi.Invite{
		Recipient:  inviteapi.Recipient{Email: email},
		AcceptedBy: username,
		Resource:   inviteapi.Resource{Type: "survey"},
	}}
}

// recordingAcceptClient records AcceptInvite calls.
type recordingAcceptClient struct {
	mu    sync.Mutex
	calls [][2]string
	err   error
}

func (r *recordingAcceptClient) client() *mocks.MockInviteAcceptanceClient {
	return &mocks.MockInviteAcceptanceClient{AcceptInviteFunc: func(_ context.Context, email, username string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.calls = append(r.calls, [2]string{email, username})
		return r.err
	}}
}

func (r *recordingAcceptClient) snapshot() [][2]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][2]string(nil), r.calls...)
}

// ─── processInviteAcceptedEvent ──────────────────────────────────────────────

func TestProcessInviteAcceptedEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("enriches the acceptor's records with their email and username", func(t *testing.T) {
		rec := &recordingAcceptClient{}

		err := processInviteAcceptedEvent(ctx, acceptedEvent("alice@example.com", "alice"), rec.client(), discardLogger())

		require.NoError(t, err)
		assert.Equal(t, [][2]string{{"alice@example.com", "alice"}}, rec.snapshot())
	})

	t.Run("an event missing the email or username is discarded without calling ITX", func(t *testing.T) {
		for name, evt := range map[string]inviteapi.InviteServiceAcceptedEvent{
			"no email":    acceptedEvent("", "alice"),
			"no username": acceptedEvent("alice@example.com", ""),
			"neither":     acceptedEvent("", ""),
		} {
			t.Run(name, func(t *testing.T) {
				rec := &recordingAcceptClient{}
				require.NoError(t, processInviteAcceptedEvent(ctx, evt, rec.client(), discardLogger()))
				assert.Empty(t, rec.snapshot())
			})
		}
	})

	t.Run("an ITX failure is returned to the caller", func(t *testing.T) {
		boom := errors.New("itx unavailable")
		rec := &recordingAcceptClient{err: boom}

		err := processInviteAcceptedEvent(ctx, acceptedEvent("alice@example.com", "alice"), rec.client(), discardLogger())

		assert.ErrorIs(t, err, boom)
	})
}

// ─── handle ──────────────────────────────────────────────────────────────────

func TestInviteAcceptedSubscriber_Handle(t *testing.T) {
	newSub := func(rec *recordingAcceptClient) *InviteAcceptedSubscriber {
		s := NewInviteAcceptedSubscriber(nil, rec.client(), discardLogger())
		s.ctx = context.Background()
		return s
	}

	t.Run("decodes the event and calls ITX", func(t *testing.T) {
		rec := &recordingAcceptClient{}
		newSub(rec).handle(&nats.Msg{Data: mustJSON(t, acceptedEvent("alice@example.com", "alice"))})

		assert.Equal(t, [][2]string{{"alice@example.com", "alice"}}, rec.snapshot())
	})

	t.Run("a malformed payload is discarded", func(t *testing.T) {
		rec := &recordingAcceptClient{}
		newSub(rec).handle(&nats.Msg{Data: []byte("not json")})

		assert.Empty(t, rec.snapshot())
	})

	t.Run("an ITX failure is swallowed", func(t *testing.T) {
		rec := &recordingAcceptClient{err: errors.New("itx unavailable")}

		assert.NotPanics(t, func() {
			newSub(rec).handle(&nats.Msg{Data: mustJSON(t, acceptedEvent("alice@example.com", "alice"))})
		})
		assert.Len(t, rec.snapshot(), 1, "the call is made once and not retried")
	})

	t.Run("the ITX call carries a deadline", func(t *testing.T) {
		var deadline time.Time
		var hasDeadline bool
		s := NewInviteAcceptedSubscriber(nil, &mocks.MockInviteAcceptanceClient{
			AcceptInviteFunc: func(ctx context.Context, _, _ string) error {
				deadline, hasDeadline = ctx.Deadline()
				return nil
			},
		}, discardLogger())
		s.ctx = context.Background()

		start := time.Now()
		s.handle(&nats.Msg{Data: mustJSON(t, acceptedEvent("alice@example.com", "alice"))})

		require.True(t, hasDeadline)
		assert.WithinDuration(t, start.Add(inviteAcceptedCallTimeout), deadline, 5*time.Second)
	})
}

// ─── Start / Stop ────────────────────────────────────────────────────────────

func TestInviteAcceptedSubscriber_StartStop(t *testing.T) {
	t.Run("receives published acceptance events until stopped", func(t *testing.T) {
		url := startTestNATS(t)
		nc, err := nats.Connect(url)
		require.NoError(t, err)
		t.Cleanup(nc.Close)

		rec := &recordingAcceptClient{}
		s := NewInviteAcceptedSubscriber(nc, rec.client(), discardLogger())
		require.NoError(t, s.Start(context.Background()))

		payload, err := json.Marshal(acceptedEvent("alice@example.com", "alice"))
		require.NoError(t, err)
		require.NoError(t, nc.Publish(inviteapi.InviteServiceAcceptedSubject, payload))
		require.NoError(t, nc.Flush())

		require.Eventually(t, func() bool { return len(rec.snapshot()) == 1 }, 5*time.Second, 20*time.Millisecond)
		assert.Equal(t, [][2]string{{"alice@example.com", "alice"}}, rec.snapshot())

		s.Stop()
		assert.Error(t, s.ctx.Err(), "Stop cancels the subscriber context")

		// After Stop the subscription is gone, so a later event is not processed.
		require.NoError(t, nc.Publish(inviteapi.InviteServiceAcceptedSubject, payload))
		require.NoError(t, nc.Flush())
		time.Sleep(200 * time.Millisecond)
		assert.Len(t, rec.snapshot(), 1)
	})

	t.Run("replicas share the queue group so each event is processed once", func(t *testing.T) {
		url := startTestNATS(t)
		nc, err := nats.Connect(url)
		require.NoError(t, err)
		t.Cleanup(nc.Close)

		rec := &recordingAcceptClient{}
		for range 2 {
			s := NewInviteAcceptedSubscriber(nc, rec.client(), discardLogger())
			require.NoError(t, s.Start(context.Background()))
			t.Cleanup(s.Stop)
		}

		require.NoError(t, nc.Publish(inviteapi.InviteServiceAcceptedSubject,
			mustJSON(t, acceptedEvent("alice@example.com", "alice"))))
		require.NoError(t, nc.Flush())

		require.Eventually(t, func() bool { return len(rec.snapshot()) >= 1 }, 5*time.Second, 20*time.Millisecond)
		time.Sleep(200 * time.Millisecond) // a second delivery would land by now
		assert.Len(t, rec.snapshot(), 1)
	})

	t.Run("Start fails and cancels its context when the subscription cannot be created", func(t *testing.T) {
		url := startTestNATS(t)
		nc, err := nats.Connect(url)
		require.NoError(t, err)
		nc.Close() // subscribing on a closed connection fails

		s := NewInviteAcceptedSubscriber(nc, &mocks.MockInviteAcceptanceClient{}, discardLogger())
		err = s.Start(context.Background())

		require.Error(t, err)
		assert.Nil(t, s.sub)
		assert.Error(t, s.ctx.Err(), "the context created by Start is cancelled on failure")
	})

	t.Run("Stop before Start is a no-op", func(t *testing.T) {
		s := NewInviteAcceptedSubscriber(nil, &mocks.MockInviteAcceptanceClient{}, discardLogger())
		assert.NotPanics(t, s.Stop)
	})
}
