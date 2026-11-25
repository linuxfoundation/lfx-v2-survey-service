// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go/jetstream"
)

// discardLogger returns a logger that drops every record.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// publishedEvent is one call recorded by newRecordingPublisher.
type publishedEvent struct {
	Action   string
	Survey   *domain.SurveyData
	Response *domain.SurveyResponseData
	Template *domain.SurveyTemplateData
}

// newRecordingPublisher returns a publisher that records every event and
// returns err from each publish call (nil for success).
func newRecordingPublisher(err error) (*mocks.MockEventPublisher, *[]publishedEvent) {
	events := &[]publishedEvent{}
	pub := &mocks.MockEventPublisher{
		PublishSurveyEventFunc: func(_ context.Context, action string, s *domain.SurveyData) error {
			*events = append(*events, publishedEvent{Action: action, Survey: s})
			return err
		},
		PublishSurveyResponseEventFunc: func(_ context.Context, action string, r *domain.SurveyResponseData) error {
			*events = append(*events, publishedEvent{Action: action, Response: r})
			return err
		},
		PublishSurveyTemplateEventFunc: func(_ context.Context, action string, tpl *domain.SurveyTemplateData) error {
			*events = append(*events, publishedEvent{Action: action, Template: tpl})
			return err
		},
	}
	return pub, events
}

// fakeKV is a stateful in-memory jetstream.KeyValue limited to Get and Put,
// which is all the event handlers use on the mappings bucket. Any other
// method panics (nil embedded interface), so an unexpected call fails loudly.
// Set putErr to make Put fail, or getErr[key] to make Get fail for one key.
type fakeKV struct {
	jetstream.KeyValue
	data   map[string][]byte
	putErr error
	getErr map[string]error
}

func newFakeKV(initial map[string]string) *fakeKV {
	kv := &fakeKV{data: map[string][]byte{}}
	for k, v := range initial {
		kv.data[k] = []byte(v)
	}
	return kv
}

func (f *fakeKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	if err, ok := f.getErr[key]; ok {
		return nil, err
	}
	v, ok := f.data[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return mockKeyValueEntry{key: key, value: v}, nil
}

func (f *fakeKV) Put(_ context.Context, key string, value []byte) (uint64, error) {
	if f.putErr != nil {
		return 0, f.putErr
	}
	f.data[key] = value
	return uint64(len(f.data)), nil
}

// value returns the stored value for key ("" if absent).
func (f *fakeKV) value(key string) string { return string(f.data[key]) }

// startTestNATS runs an in-process JetStream-enabled NATS server on a random
// port and returns its client URL. The server is shut down when the test ends.
func startTestNATS(t *testing.T) string {
	t.Helper()
	ns, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	})
	if err != nil {
		t.Fatalf("create NATS server: %v", err)
	}
	go ns.Start()
	t.Cleanup(ns.Shutdown)
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	return ns.ClientURL()
}
