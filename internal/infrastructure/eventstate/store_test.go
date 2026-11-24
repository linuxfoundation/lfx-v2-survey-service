// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("SURVEY_STATE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SURVEY_STATE_TEST_DATABASE_URL to a disposable local Postgres database")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	name := fmt.Sprintf("survey_state_test_%d", time.Now().UnixNano())
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+name)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()
	s, err := Open(ctx, u.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		s.Close()
		_, err := admin.Exec(ctx, "DROP SCHEMA "+name+" CASCADE")
		if err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	require.NoError(t, s.ApplySchema(ctx))
	return s
}
func TestStore(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	require.Error(t, s.Ready(ctx))
	require.NoError(t, s.ApplySchema(ctx))
	t.Run("durable invitation intent is explicit", func(t *testing.T) {
		prepared, err := s.InvitationPrepared(ctx, "new-response")
		require.NoError(t, err)
		require.False(t, prepared)
		require.NoError(t, s.PrepareInvitation(ctx, "new-response"))
		require.NoError(t, s.PrepareInvitation(ctx, "new-response"))
		prepared, err = s.InvitationPrepared(ctx, "new-response")
		require.NoError(t, err)
		require.True(t, prepared)
		require.NoError(t, s.ResolveInvitation(ctx, "new-response"))
		require.NoError(t, s.PrepareInvitation(ctx, "new-response"))
		prepared, err = s.InvitationPrepared(ctx, "new-response")
		require.NoError(t, err)
		require.False(t, prepared, "resolved eligibility cannot be reactivated")
	})
	t.Run("resource locks serialize replicas and release", func(t *testing.T) {
		lockedCtx, release, err := s.LockResource(ctx, "itx-surveys.one")
		require.NoError(t, err)
		_, err = s.Put(lockedCtx, "survey.locked", []byte("1"))
		require.NoError(t, err)
		_, _, err = s.LockResource(ctx, "itx-surveys.one")
		require.ErrorIs(t, err, domain.ErrResourceBusy)
		_, other, err := s.LockResource(ctx, "itx-surveys.two")
		require.NoError(t, err)
		other()
		release()
		_, release, err = s.LockResource(ctx, "itx-surveys.one")
		require.NoError(t, err)
		release()
	})
	t.Run("resource DML does not exhaust locked pool", func(t *testing.T) {
		contexts := make([]context.Context, 0, 10)
		releases := make([]func(), 0, 10)
		defer func() {
			for _, release := range releases {
				release()
			}
		}()
		for i := range 10 {
			locked, release, err := s.LockResource(ctx, fmt.Sprintf("pool.%d", i))
			require.NoError(t, err)
			contexts = append(contexts, locked)
			releases = append(releases, release)
		}
		for i, locked := range contexts {
			_, err := s.Put(locked, fmt.Sprintf("survey.pool-%d", i), []byte("1"))
			require.NoError(t, err)
		}
	})
	t.Run("superseded source cannot write state", func(t *testing.T) {
		fenced := domain.WithSourceFence(ctx, staleSourceFence{})
		_, err := s.Put(fenced, "survey.stale", []byte("1"))
		require.ErrorIs(t, err, domain.ErrSourceSuperseded)
		require.ErrorIs(t, s.PrepareInvitation(fenced, "stale"), domain.ErrSourceSuperseded)
		require.ErrorIs(t, s.ResolveInvitation(fenced, "stale"), domain.ErrSourceSuperseded)
		_, err = s.Get(ctx, "survey.stale")
		require.ErrorIs(t, err, jetstream.ErrKeyNotFound)
	})
	for key, value := range map[string]string{"survey.test": "1", "survey_response.test": "!del", "survey_template.test": "", "v1_survey_response_lfid_invite_sent.test": "pending"} {
		t.Run(key, func(t *testing.T) {
			rev, err := s.Create(ctx, key, []byte(value))
			require.NoError(t, err)
			_, err = s.Create(ctx, key, []byte(value))
			require.ErrorIs(t, err, jetstream.ErrKeyExists)
			e, err := s.Get(ctx, key)
			require.NoError(t, err)
			require.Equal(t, value, string(e.Value()))
			require.Equal(t, rev, e.Revision())
			next, err := s.Update(ctx, key, []byte(value), rev)
			require.NoError(t, err)
			require.Greater(t, next, rev)
			_, err = s.Update(ctx, key, []byte(value), rev)
			require.ErrorIs(t, err, domain.ErrEventStateRevisionMismatch)
			require.NoError(t, s.Delete(ctx, key))
			fresh, err := s.Create(ctx, key, []byte(value))
			require.NoError(t, err)
			require.Greater(t, fresh, next)
			_, err = s.Update(ctx, key, []byte(value), next)
			require.ErrorIs(t, err, domain.ErrEventStateRevisionMismatch)
		})
	}
	t.Run("one invitation dispatcher", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 20)
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := s.Create(ctx, "v1_survey_response_lfid_invite_sent.concurrent", []byte("pending"))
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		wins := 0
		for err := range results {
			if err == nil {
				wins++
			} else {
				require.ErrorIs(t, err, jetstream.ErrKeyExists)
			}
		}
		require.Equal(t, 1, wins)
	})
}

type staleSourceFence struct{}

func (staleSourceFence) Current(context.Context) (bool, error) { return false, nil }
func TestBinding(t *testing.T) {
	for _, key := range []string{"project.sfid.test", "committee.sfid.test", "survey.bad>key"} {
		_, err := bind(key)
		require.Error(t, err)
	}
	_, err := Open(context.Background(), "")
	require.Error(t, err)
}
