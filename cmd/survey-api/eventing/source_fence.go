// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/eventstate"
	"github.com/nats-io/nats.go/jetstream"
)

type deliveryFence struct {
	revision   uint64
	latest     func(context.Context) (uint64, error)
	superseded atomic.Bool
}

func (f *deliveryFence) Current(ctx context.Context) (bool, error) {
	if f.superseded.Load() {
		return false, nil
	}
	latest, err := f.latest(ctx)
	if err != nil {
		return false, domain.NewUnavailableError("source revision unavailable")
	}
	if latest < f.revision {
		return false, domain.NewUnavailableError("source revision cannot prove current delivery")
	}
	if latest > f.revision {
		f.superseded.Store(true)
		return false, nil
	}
	return true, nil
}

func sourceCurrent(ctx context.Context, logger *slog.Logger) (proceed, retry bool) {
	err := domain.CheckSourceFence(ctx)
	if errors.Is(err, domain.ErrSourceSuperseded) {
		logger.InfoContext(ctx, "skipping superseded source delivery")
		return false, false
	}
	if err != nil {
		logger.With(errKey, err).WarnContext(ctx, "failed to verify source revision; retrying")
		return false, true
	}
	return true, false
}

func (ep *EventProcessor) processMessage(ctx context.Context, msg jetstream.Msg) {
	metadata, err := msg.Metadata()
	if err != nil || metadata == nil || metadata.Sequence.Stream == 0 {
		ep.logger.WarnContext(ctx, "source message revision unavailable; retrying")
		_ = msg.NakWithDelay(20 * time.Second)
		return
	}
	stopProgress := ep.keepAlive(ctx, msg)
	defer stopProgress()
	msg = progressMessage{Msg: msg, stop: stopProgress}
	key := strings.TrimPrefix(msg.Subject(), "$KV."+V1ObjectsBucket+".")
	locker, ok := ep.mappingsKV.(domain.ResourceLocker)
	if !ok {
		ep.logger.With("key", key).WarnContext(ctx, "resource locking unavailable; retrying")
		_ = msg.NakWithDelay(20 * time.Second)
		return
	}
	lockedCtx, release, err := locker.LockResource(ctx, key)
	if err != nil {
		ep.logger.With(errKey, err, "key", key).WarnContext(ctx, "resource processing fence unavailable; retrying")
		_ = msg.NakWithDelay(20 * time.Second)
		return
	}
	ctx = lockedCtx
	defer release()
	fence := &deliveryFence{revision: metadata.Sequence.Stream, latest: func(ctx context.Context) (uint64, error) {
		latest, err := eventstate.ReadSourceMessage(ctx, ep.jsInstance, ep.config.StreamName, 0, "", msg.Subject())
		if err != nil {
			return 0, err
		}
		return latest.Sequence, nil
	}}
	fencedCtx := domain.WithSourceFence(ctx, fence)
	if proceed, retry := sourceCurrent(fencedCtx, ep.logger.With("key", key)); !proceed {
		if retry {
			_ = msg.NakWithDelay(20 * time.Second)
		} else {
			_ = msg.Ack()
		}
		return
	}
	kvMessageHandler(fencedCtx, msg, ep.publisher, ep.idMapper, ep.mappingsKV, ep.v1ObjectsKV, ep.inviteHandler, ep.logger)
}

// keepAlive covers all I/O, including lock acquisition and initial source reads.
// Stop and join the heartbeat before ACK/NAK or releasing callback resources.
func (ep *EventProcessor) keepAlive(ctx context.Context, msg jetstream.Msg) func() {
	// A slow handler retains its current delivery while finishing I/O. Restarts
	// still redeliver durably, without a finite dependency-retry ceiling.
	done := make(chan struct{})
	exited := make(chan struct{})
	var stopOnce sync.Once
	stopProgress := func() { stopOnce.Do(func() { close(done); <-exited }) }
	interval := ep.config.AckWait / 3
	if interval <= 0 {
		interval = 10 * time.Second
	}
	go func() {
		defer close(exited)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := msg.InProgress(); err != nil {
					ep.logger.With(errKey, err, "subject", msg.Subject()).WarnContext(ctx, "failed to extend active source delivery")
				}
			}
		}
	}()
	return stopProgress
}

type progressMessage struct {
	jetstream.Msg
	stop func()
}

func (m progressMessage) Ack() error { m.stop(); return m.Msg.Ack() }
func (m progressMessage) NakWithDelay(delay time.Duration) error {
	m.stop()
	return m.Msg.NakWithDelay(delay)
}
