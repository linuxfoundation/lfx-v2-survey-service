// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestImport(t *testing.T) {
	for _, mode := range []string{"stable", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
			require.NoError(t, err)
			go srv.Start()
			require.True(t, srv.ReadyForConnections(5*time.Second))
			t.Cleanup(func() { srv.Shutdown(); srv.WaitForShutdown() })
			nc, err := nats.Connect(srv.ClientURL())
			require.NoError(t, err)
			t.Cleanup(nc.Close)
			js, err := jetstream.New(nc)
			require.NoError(t, err)
			kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "v1-mappings", History: 10})
			require.NoError(t, err)
			want := map[string]string{"survey.test": "1", "survey_response.test": "1", "survey_template.test": "!del", "v1_survey_response_lfid_invite_sent.test": "pending"}
			for key, value := range want {
				_, err = kv.Put(ctx, "project.sfid.unrelated", []byte("project"))
				require.NoError(t, err)
				_, err = kv.Put(ctx, key, []byte("old"))
				require.NoError(t, err)
				_, err = kv.Put(ctx, key, []byte(value))
				require.NoError(t, err)
			}
			_, err = kv.Put(ctx, "survey.deleted", []byte("1"))
			require.NoError(t, err)
			require.NoError(t, kv.Delete(ctx, "survey.deleted"))
			_, err = kv.Put(ctx, "survey.purged", []byte("1"))
			require.NoError(t, err)
			require.NoError(t, kv.Purge(ctx, "survey.purged"))
			_, err = kv.Put(ctx, "survey.restored", []byte("old"))
			require.NoError(t, err)
			require.NoError(t, kv.Delete(ctx, "survey.restored"))
			_, err = kv.Put(ctx, "survey.restored", []byte("1"))
			require.NoError(t, err)
			if mode == "conflict" {
				_, err = s.Put(ctx, "survey.test", []byte("!del"))
				require.NoError(t, err)
			}
			n, err := s.Import(ctx, js, true)
			if mode == "conflict" {
				require.Error(t, err)
				require.Equal(t, "target_conflict", MigrationFailureCategory(err))
				require.Error(t, s.Ready(ctx))
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(want)+1, n)
			for key, value := range want {
				e, err := s.Get(ctx, key)
				require.NoError(t, err)
				require.Equal(t, value, string(e.Value()))
			}
			for _, key := range []string{"survey.deleted", "survey.purged"} {
				_, err := s.Get(ctx, key)
				require.ErrorIs(t, err, jetstream.ErrKeyNotFound)
			}
			e, err := s.Get(ctx, "survey.restored")
			require.NoError(t, err)
			require.Equal(t, "1", string(e.Value()))
			require.NoError(t, s.Ready(ctx))
			_, err = s.Import(ctx, js, true)
			require.Equal(t, "already_verified", MigrationFailureCategory(err))
		})
	}
}
