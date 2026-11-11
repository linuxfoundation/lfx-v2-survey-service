// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/eventing"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createBuckets creates the two KV buckets the processor opens at construction
// and returns the JetStream handle used to write to them.
func createBuckets(t *testing.T, url string, buckets ...string) jetstream.JetStream {
	t.Helper()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	require.NoError(t, err)
	for _, b := range buckets {
		_, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{Bucket: b})
		require.NoError(t, err)
	}
	return js
}

func processorConfig(url string) eventing.Config {
	return eventing.Config{
		NATSURL:        url,
		ConsumerName:   "survey-test-consumer",
		StreamName:     "KV_" + V1ObjectsBucket,
		FilterSubjects: []string{"$KV." + V1ObjectsBucket + ".>"},
		MaxDeliver:     3,
		AckWait:        5 * time.Second,
		MaxAckPending:  10,
	}
}

// ─── NewEventProcessor ───────────────────────────────────────────────────────

func TestNewEventProcessor(t *testing.T) {
	t.Run("fails when NATS is unreachable", func(t *testing.T) {
		cfg := processorConfig("nats://127.0.0.1:1")

		ep, err := NewEventProcessor(cfg, &mocks.MockIDMapper{}, InviteFeatureConfig{}, nil, nil, discardLogger())

		require.Error(t, err)
		assert.Nil(t, ep)
		assert.Contains(t, err.Error(), "failed to connect to NATS")
	})

	t.Run("fails when the mappings bucket is missing", func(t *testing.T) {
		url := startTestNATS(t)
		createBuckets(t, url, V1ObjectsBucket)

		ep, err := NewEventProcessor(processorConfig(url), &mocks.MockIDMapper{}, InviteFeatureConfig{}, nil, nil, discardLogger())

		require.Error(t, err)
		assert.Nil(t, ep)
		assert.Contains(t, err.Error(), V1MappingsBucket)
	})

	t.Run("fails when the objects bucket is missing", func(t *testing.T) {
		url := startTestNATS(t)
		createBuckets(t, url, V1MappingsBucket)

		ep, err := NewEventProcessor(processorConfig(url), &mocks.MockIDMapper{}, InviteFeatureConfig{}, nil, nil, discardLogger())

		require.Error(t, err)
		assert.Nil(t, ep)
		assert.Contains(t, err.Error(), V1ObjectsBucket)
	})

	t.Run("invite handler wiring", func(t *testing.T) {
		url := startTestNATS(t)
		createBuckets(t, url, V1MappingsBucket, V1ObjectsBucket)
		sender := &mocks.MockInviteSender{}
		reader := &mocks.MockUserReader{}

		tests := []struct {
			name    string
			cfg     InviteFeatureConfig
			wantNil bool
		}{
			{"disabled", InviteFeatureConfig{Enabled: false, SelfServeBaseURL: "https://app.example.com"}, true},
			{"enabled without a base url", InviteFeatureConfig{Enabled: true}, true},
			{"enabled with a blank base url", InviteFeatureConfig{Enabled: true, SelfServeBaseURL: "   "}, true},
			{"enabled with a base url", InviteFeatureConfig{Enabled: true, SelfServeBaseURL: "https://app.example.com"}, false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ep, err := NewEventProcessor(processorConfig(url), &mocks.MockIDMapper{}, tt.cfg, sender, reader, discardLogger())
				require.NoError(t, err)
				t.Cleanup(func() { _ = ep.Stop() })

				if tt.wantNil {
					assert.Nil(t, ep.inviteHandler)
					return
				}
				require.NotNil(t, ep.inviteHandler)
				assert.True(t, ep.inviteHandler.inviteEnabled())
				assert.Equal(t, "https://app.example.com", ep.inviteHandler.selfServeBaseURL)
				assert.Same(t, ep.mappingsKV, ep.inviteHandler.v1MappingsKV)
				assert.Same(t, ep.v1ObjectsKV, ep.inviteHandler.v1ObjectsKV)
			})
		}
	})
}

// ─── Start / Stop ────────────────────────────────────────────────────────────

func TestEventProcessor_Start(t *testing.T) {
	t.Run("fails when the stream does not exist", func(t *testing.T) {
		url := startTestNATS(t)
		createBuckets(t, url, V1MappingsBucket, V1ObjectsBucket)
		cfg := processorConfig(url)
		cfg.StreamName = "NO_SUCH_STREAM"
		ep, err := NewEventProcessor(cfg, &mocks.MockIDMapper{}, InviteFeatureConfig{}, nil, nil, discardLogger())
		require.NoError(t, err)
		t.Cleanup(func() { _ = ep.Stop() })

		err = ep.Start(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create or update consumer")
	})

	t.Run("consumes KV entries, stores the mapping, and returns when the context is cancelled", func(t *testing.T) {
		url := startTestNATS(t)
		js := createBuckets(t, url, V1MappingsBucket, V1ObjectsBucket)
		ep, err := NewEventProcessor(processorConfig(url), &mocks.MockIDMapper{}, InviteFeatureConfig{}, nil, nil, discardLogger())
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		objects, err := js.KeyValue(ctx, V1ObjectsBucket)
		require.NoError(t, err)
		mappings, err := js.KeyValue(ctx, V1MappingsBucket)
		require.NoError(t, err)

		// The entry is written before the consumer exists, so it is only
		// delivered because the consumer replays existing entries. A survey
		// template needs no ID mapping, so this exercises the whole
		// consumer → kvMessageHandler → handler → publisher → mapping path.
		_, err = objects.Put(ctx, "surveymonkey-surveys.tpl-1", mustJSON(t, templateV1Data()))
		require.NoError(t, err)

		// Registered before any fatal assertion so a failure cannot leave Start
		// blocked or the NATS connection open for the rest of the test process.
		var startErr error
		exited := make(chan struct{})
		stopped := false
		stop := func() {
			cancel()
			select {
			case <-time.After(5 * time.Second):
				t.Error("Start did not return after the context was cancelled")
			case <-exited:
			}
			if !stopped {
				stopped = true
				assert.NoError(t, ep.Stop())
			}
		}
		t.Cleanup(stop)
		go func() {
			defer close(exited)
			startErr = ep.Start(ctx)
		}()

		require.Eventually(t, func() bool {
			_, err := mappings.Get(ctx, "survey_template."+templateV1Data()["id"].(string))
			return err == nil
		}, 10*time.Second, 50*time.Millisecond, "the template mapping is stored once the entry is processed")

		// DeliverAll would also deliver the single entry above, so pin the
		// replay policy on the consumer itself.
		consumer, err := js.Consumer(ctx, processorConfig(url).StreamName, processorConfig(url).ConsumerName)
		require.NoError(t, err)
		assert.Equal(t, jetstream.DeliverLastPerSubjectPolicy, consumer.CachedInfo().Config.DeliverPolicy)

		stop()
		assert.NoError(t, startErr)
	})
}

func TestEventProcessor_Stop(t *testing.T) {
	t.Run("closes the publisher and tolerates a processor that never started", func(t *testing.T) {
		closed := false
		ep := &EventProcessor{
			publisher: &mocks.MockEventPublisher{CloseFunc: func() error { closed = true; return nil }},
			logger:    discardLogger(),
		}

		assert.NoError(t, ep.Stop())
		assert.True(t, closed)
	})

	t.Run("a publisher close error is logged, not returned", func(t *testing.T) {
		ep := &EventProcessor{
			publisher: &mocks.MockEventPublisher{CloseFunc: func() error { return errors.New("close failed") }},
			logger:    discardLogger(),
		}

		assert.NoError(t, ep.Stop())
	})

	t.Run("stops the consumer and drains the connection", func(t *testing.T) {
		url := startTestNATS(t)
		nc, err := nats.Connect(url)
		require.NoError(t, err)
		consumeCtx := &fakeConsumeContext{}
		ep := &EventProcessor{natsConn: nc, consumeCtx: consumeCtx, logger: discardLogger()}

		assert.NoError(t, ep.Stop())
		assert.True(t, consumeCtx.stopped, "the consumer is stopped")
		assert.True(t, nc.IsClosed(), "the connection is closed")
	})
}

// fakeConsumeContext records Stop.
type fakeConsumeContext struct {
	jetstream.ConsumeContext
	stopped bool
}

func (f *fakeConsumeContext) Stop() { f.stopped = true }
