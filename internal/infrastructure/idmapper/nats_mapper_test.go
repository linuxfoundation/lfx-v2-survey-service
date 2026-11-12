// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package idmapper

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// lookupRequest is one request seen by the fake v1-sync-helper.
type lookupRequest struct {
	Key    string
	Header nats.Header
}

// fakeSyncHelper answers lookups on lookupSubject. A nil reply func means no
// responder is registered; a reply func returning ok=false never answers.
type fakeSyncHelper struct {
	mu       sync.Mutex
	requests []lookupRequest
}

func (f *fakeSyncHelper) seen() []lookupRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]lookupRequest(nil), f.requests...)
}

// startNATS runs an in-process NATS server and returns its client URL.
func startNATS(t *testing.T) string {
	t.Helper()
	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true})
	require.NoError(t, err)
	go ns.Start()
	t.Cleanup(ns.Shutdown)
	require.True(t, ns.ReadyForConnections(10*time.Second), "NATS server not ready")
	return ns.ClientURL()
}

// newMapper returns a mapper wired to an in-process server. reply decides the
// answer for each lookup key; pass nil to leave the subject without a responder.
func newMapper(t *testing.T, reply func(key string) (string, bool)) (*NATSMapper, *fakeSyncHelper) {
	t.Helper()
	url := startNATS(t)
	helper := &fakeSyncHelper{}

	if reply != nil {
		nc, err := nats.Connect(url)
		require.NoError(t, err)
		t.Cleanup(nc.Close)
		_, err = nc.Subscribe(lookupSubject, func(m *nats.Msg) {
			helper.mu.Lock()
			helper.requests = append(helper.requests, lookupRequest{Key: string(m.Data), Header: m.Header})
			helper.mu.Unlock()
			if answer, ok := reply(string(m.Data)); ok {
				_ = m.Respond([]byte(answer))
			}
		})
		require.NoError(t, err)
		require.NoError(t, nc.Flush())
	}

	mapper, err := NewNATSMapper(Config{URL: url})
	require.NoError(t, err)
	t.Cleanup(mapper.Close)
	return mapper, helper
}

// echo answers every lookup with "<key>-answer".
func echo(key string) (string, bool) { return key + "-answer", true }

func requireDomainError(t *testing.T, err error, want domain.ErrorType, contains string) {
	t.Helper()
	var de *domain.DomainError
	require.ErrorAs(t, err, &de)
	assert.Equal(t, want, de.Type)
	assert.Contains(t, err.Error(), contains)
}

// ─── NewNATSMapper / Close ───────────────────────────────────────────────────

func TestNewNATSMapper(t *testing.T) {
	t.Run("a missing URL is rejected", func(t *testing.T) {
		m, err := NewNATSMapper(Config{})
		assert.Nil(t, m)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "NATS URL is required")
	})

	t.Run("an unreachable server is an error", func(t *testing.T) {
		m, err := NewNATSMapper(Config{URL: "nats://127.0.0.1:1"})
		assert.Nil(t, m)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect to NATS")
	})

	t.Run("the timeout defaults when unset and is kept when set", func(t *testing.T) {
		url := startNATS(t)

		def, err := NewNATSMapper(Config{URL: url})
		require.NoError(t, err)
		t.Cleanup(def.Close)
		assert.Equal(t, defaultTimeout, def.timeout)

		custom, err := NewNATSMapper(Config{URL: url, Timeout: 7 * time.Second})
		require.NoError(t, err)
		t.Cleanup(custom.Close)
		assert.Equal(t, 7*time.Second, custom.timeout)
	})
}

func TestNATSMapper_Close(t *testing.T) {
	t.Run("closes the connection", func(t *testing.T) {
		m, _ := newMapper(t, echo)
		m.Close()
		assert.True(t, m.conn.IsClosed())
	})

	t.Run("a mapper without a connection can be closed", func(t *testing.T) {
		assert.NotPanics(t, func() { (&NATSMapper{}).Close() })
	})
}

// ─── lookup keys ─────────────────────────────────────────────────────────────

func TestNATSMapper_RequestKeys(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		call    func(*NATSMapper) (string, error)
		wantKey string
	}{
		{"project v2 to v1", func(m *NATSMapper) (string, error) { return m.MapProjectV2ToV1(ctx, "id-1") }, "project.uid.id-1"},
		{"project v1 to v2", func(m *NATSMapper) (string, error) { return m.MapProjectV1ToV2(ctx, "id-2") }, "project.sfid.id-2"},
		{"committee v1 to v2", func(m *NATSMapper) (string, error) { return m.MapCommitteeV1ToV2(ctx, "id-3") }, "committee.sfid.id-3"},
		{"committee v2 to v1", func(m *NATSMapper) (string, error) { return m.MapCommitteeV2ToV1(ctx, "id-4") }, "committee.uid.id-4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, helper := newMapper(t, echo)

			got, err := tt.call(m)

			require.NoError(t, err)
			assert.Equal(t, tt.wantKey+"-answer", got, "the reply is returned unchanged")
			require.Len(t, helper.seen(), 1)
			assert.Equal(t, tt.wantKey, helper.seen()[0].Key)
		})
	}
}

func TestNATSMapper_EmptyIDsAreRejectedWithoutALookup(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		call func(*NATSMapper) (string, error)
		want string
	}{
		{"project v2 to v1", func(m *NATSMapper) (string, error) { return m.MapProjectV2ToV1(ctx, "") }, "v2 project UID is required"},
		{"project v1 to v2", func(m *NATSMapper) (string, error) { return m.MapProjectV1ToV2(ctx, "") }, "v1 project SFID is required"},
		{"committee v2 to v1", func(m *NATSMapper) (string, error) { return m.MapCommitteeV2ToV1(ctx, "") }, "v2 committee UID is required"},
		{"committee v1 to v2", func(m *NATSMapper) (string, error) { return m.MapCommitteeV1ToV2(ctx, "") }, "v1 committee SFID is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, helper := newMapper(t, echo)

			got, err := tt.call(m)

			assert.Empty(t, got)
			requireDomainError(t, err, domain.ErrorTypeValidation, tt.want)
			assert.Empty(t, helper.seen())
		})
	}
}

// ─── committee v2 → v1 response parsing ──────────────────────────────────────

func TestNATSMapper_MapCommitteeV2ToV1_ResponseFormat(t *testing.T) {
	ctx := context.Background()

	t.Run("keeps only the committee SFID of project:committee", func(t *testing.T) {
		m, _ := newMapper(t, func(string) (string, bool) { return "project-sfid:committee-sfid", true })

		got, err := m.MapCommitteeV2ToV1(ctx, "uid-1")

		require.NoError(t, err)
		assert.Equal(t, "committee-sfid", got)
	})

	t.Run("a bare SFID is returned as is", func(t *testing.T) {
		m, _ := newMapper(t, func(string) (string, bool) { return "committee-sfid", true })

		got, err := m.MapCommitteeV2ToV1(ctx, "uid-1")

		require.NoError(t, err)
		assert.Equal(t, "committee-sfid", got)
	})

	t.Run("more than one separator is unavailable", func(t *testing.T) {
		m, _ := newMapper(t, func(string) (string, bool) { return "a:b:c", true })

		got, err := m.MapCommitteeV2ToV1(ctx, "uid-1")

		assert.Empty(t, got)
		requireDomainError(t, err, domain.ErrorTypeUnavailable, "unexpected committee mapping format: a:b:c")
	})

	t.Run("an empty committee SFID is unavailable", func(t *testing.T) {
		m, _ := newMapper(t, func(string) (string, bool) { return "project-sfid:", true })

		got, err := m.MapCommitteeV2ToV1(ctx, "uid-1")

		assert.Empty(t, got)
		requireDomainError(t, err, domain.ErrorTypeUnavailable, "committee SFID is empty")
	})

	t.Run("a lookup failure is returned unchanged", func(t *testing.T) {
		m, _ := newMapper(t, func(string) (string, bool) { return "", true })

		_, err := m.MapCommitteeV2ToV1(ctx, "uid-1")

		requireDomainError(t, err, domain.ErrorTypeValidation, "mapping not found for committee.uid.uid-1")
	})
}

// ─── lookup outcomes ─────────────────────────────────────────────────────────

func TestNATSMapper_LookupOutcomes(t *testing.T) {
	t.Run("an error reply is unavailable and carries the helper's message", func(t *testing.T) {
		m, _ := newMapper(t, func(string) (string, bool) { return "error: kv bucket missing", true })

		got, err := m.MapProjectV1ToV2(context.Background(), "sfid-1")

		assert.Empty(t, got)
		requireDomainError(t, err, domain.ErrorTypeUnavailable, "v1-sync-helper error: kv bucket missing")
	})

	t.Run("an empty reply means the ID is unknown and is a validation error", func(t *testing.T) {
		m, _ := newMapper(t, func(string) (string, bool) { return "", true })

		got, err := m.MapProjectV1ToV2(context.Background(), "sfid-1")

		assert.Empty(t, got)
		requireDomainError(t, err, domain.ErrorTypeValidation, "invalid ID: mapping not found for project.sfid.sfid-1")
	})

	t.Run("a reply that merely contains 'error' is a valid mapping", func(t *testing.T) {
		m, _ := newMapper(t, func(string) (string, bool) { return "error-prone-uid", true })

		got, err := m.MapProjectV1ToV2(context.Background(), "sfid-1")

		require.NoError(t, err)
		assert.Equal(t, "error-prone-uid", got)
	})

	t.Run("no responder is unavailable", func(t *testing.T) {
		m, _ := newMapper(t, nil)

		got, err := m.MapProjectV1ToV2(context.Background(), "sfid-1")

		assert.Empty(t, got)
		requireDomainError(t, err, domain.ErrorTypeUnavailable, "failed to lookup ID mapping")
	})

	t.Run("an unanswered request times out as unavailable", func(t *testing.T) {
		m, _ := newMapper(t, func(string) (string, bool) { return "", false })
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		got, err := m.MapProjectV1ToV2(ctx, "sfid-1")

		assert.Empty(t, got)
		requireDomainError(t, err, domain.ErrorTypeUnavailable, "v1-sync-helper lookup timed out")
		assert.True(t, errors.Is(err, context.DeadlineExceeded), "the cause is kept for callers")
	})

	t.Run("a closed connection is unavailable", func(t *testing.T) {
		m, _ := newMapper(t, echo)
		m.Close()

		_, err := m.MapProjectV1ToV2(context.Background(), "sfid-1")

		requireDomainError(t, err, domain.ErrorTypeUnavailable, "failed to lookup ID mapping")
	})
}

func TestNATSMapper_PropagatesTraceContext(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })

	traceID, err := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("b7ad6b7169203331")
	require.NoError(t, err)
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, Remote: true,
	}))
	m, helper := newMapper(t, echo)

	_, err = m.MapProjectV1ToV2(ctx, "sfid-1")

	require.NoError(t, err)
	require.Len(t, helper.seen(), 1)
	assert.Contains(t, helper.seen()[0].Header.Get("traceparent"), traceID.String())
}
