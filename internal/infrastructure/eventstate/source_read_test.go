// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

func TestReadSourceMessage(t *testing.T) {
	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	require.NoError(t, err)
	go srv.Start()
	require.True(t, srv.ReadyForConnections(5*time.Second))
	t.Cleanup(func() { srv.Shutdown(); srv.WaitForShutdown() })
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	ctx := context.Background()
	js, err := jetstream.New(nc)
	require.NoError(t, err)
	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "source", History: 10})
	require.NoError(t, err)
	stream, err := js.Stream(ctx, "KV_source")
	require.NoError(t, err)
	require.True(t, stream.CachedInfo().Config.AllowDirect)
	_, err = kv.Put(ctx, "unrelated", []byte("other"))
	require.NoError(t, err)
	revision, err := kv.Put(ctx, "survey.one", []byte("first"))
	require.NoError(t, err)
	msg, err := ReadSourceMessage(ctx, js, "KV_source", 1, "$KV.source.survey.>", "")
	require.NoError(t, err)
	require.Equal(t, revision, msg.Sequence)
	require.Equal(t, []byte("first"), msg.Data)
	require.NoError(t, kv.Delete(ctx, "survey.one"))
	msg, err = ReadSourceMessage(ctx, js, "KV_source", 0, "", "$KV.source.survey.one")
	require.NoError(t, err)
	require.Equal(t, "DEL", msg.Header.Get("KV-Operation"))
	require.Greater(t, msg.Sequence, revision)
	_, err = ReadSourceMessage(ctx, js, "KV_source", 0, "", "$KV.source.missing")
	require.ErrorIs(t, err, jetstream.ErrMsgNotFound)

	// A custom API responder proves requests do not use DIRECT.GET and retains
	// custom prefix routing without relying on default server configuration.
	for _, prefix := range []string{"custom.API", "custom.API."} {
		t.Run(prefix, func(t *testing.T) {
			sub, err := nc.SubscribeSync(strings.TrimSuffix(prefix, ".") + ".>")
			require.NoError(t, err)
			defer func() { require.NoError(t, sub.Unsubscribe()) }()
			require.NoError(t, nc.Flush())
			custom, err := jetstream.NewWithAPIPrefix(nc, prefix)
			require.NoError(t, err)
			result := make(chan error, 1)
			go func() {
				_, err := ReadSourceMessage(ctx, custom, "KV_source", 8, "$KV.source.survey.>", "")
				result <- err
			}()
			request, err := sub.NextMsg(time.Second)
			require.NoError(t, err)
			require.Equal(t, strings.TrimSuffix(prefix, ".")+".STREAM.MSG.GET.KV_source", request.Subject)
			var body map[string]any
			require.NoError(t, json.Unmarshal(request.Data, &body))
			require.Equal(t, float64(8), body["seq"])
			require.Equal(t, "$KV.source.survey.>", body["next_by_subj"])
			require.NoError(t, request.Respond([]byte(`{"message":{"subject":"$KV.source.survey.one","seq":8,"data":"b2s="}}`)))
			require.NoError(t, <-result)
		})
	}
}
