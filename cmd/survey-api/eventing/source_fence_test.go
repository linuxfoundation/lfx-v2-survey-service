// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestDeliveryFence(t *testing.T) {
	for _, tt := range []struct {
		name           string
		latest         uint64
		err            error
		proceed, retry bool
	}{
		{"current", 3, nil, true, false}, {"superseded", 4, nil, false, false}, {"lagging", 2, nil, false, true}, {"unavailable", 0, errors.New("private infrastructure details"), false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &deliveryFence{revision: 3, latest: func(context.Context) (uint64, error) { return tt.latest, tt.err }}
			ctx := domain.WithSourceFence(context.Background(), f)
			proceed, retry := sourceCurrent(ctx, discardLogger())
			require.Equal(t, tt.proceed, proceed)
			require.Equal(t, tt.retry, retry)
		})
	}
}

func TestDeleteRestoreDelayedRetry(t *testing.T) {
	ctx := context.Background()
	state := newFakeKV(nil)
	publisher, record := newRecordingPublisher(nil)
	latest := uint64(1)
	fence := &deliveryFence{revision: 1, latest: func(context.Context) (uint64, error) { return latest, nil }}
	oldCtx := domain.WithSourceFence(ctx, fence)
	// Delete publishes, but its processing-state write fails.
	state.putErr = domain.ErrEventStateUnavailable
	require.True(t, handleSurveyDelete(oldCtx, "one", publisher, state, discardLogger()))
	// A newer source PUT restores the state and downstream record.
	latest = 2
	state.putErr = nil
	data := surveyV1Data()
	data["id"] = "one"
	newCtx := domain.WithSourceFence(ctx, &deliveryFence{revision: 2, latest: fence.latest})
	require.False(t, handleSurveyUpdate(newCtx, "itx-surveys.one", data, publisher, v1IDMapper(), state, discardLogger()))
	require.False(t, handleSurveyDelete(oldCtx, "one", publisher, state, discardLogger()))
	require.Len(t, *record, 2, "only the initial deletion and newer restore must publish")
	entry, err := state.Get(ctx, "survey.one")
	require.NoError(t, err)
	require.Equal(t, "1", string(entry.Value()))
}

func TestProcessMessageSkipsSupersededRevision(t *testing.T) {
	url := startTestNATS(t)
	js := createBuckets(t, url, V1ObjectsBucket)
	objects, err := js.KeyValue(context.Background(), V1ObjectsBucket)
	require.NoError(t, err)
	_, err = objects.Put(context.Background(), "surveymonkey-surveys.one", []byte(`{}`))
	require.NoError(t, err)
	_, err = objects.Put(context.Background(), "surveymonkey-surveys.one", []byte(`{}`))
	require.NoError(t, err)
	ep := &EventProcessor{jsInstance: js, mappingsKV: newFakeKV(nil), config: processorConfig(url), logger: discardLogger()}
	msg := &fakeMsg{subject: "$KV.v1-objects.surveymonkey-surveys.one", headers: nats.Header{"KV-Operation": []string{"DEL"}}, revision: 1, delivered: 7}
	ep.processMessage(context.Background(), msg)
	require.True(t, msg.acked)
	require.False(t, msg.naked)
}
