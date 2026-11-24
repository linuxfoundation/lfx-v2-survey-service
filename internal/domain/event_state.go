// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"
	"errors"
	"github.com/nats-io/nats.go/jetstream"
)

var ErrEventStateUnavailable = errors.New("survey event state unavailable")
var ErrEventStateRevisionMismatch = errors.New("survey event state revision mismatch")
var ErrSourceSuperseded = errors.New("survey source delivery superseded")
var ErrResourceBusy = errors.New("survey resource processing busy")

// EventState separates processing state from source-record KV. The adapter
// reconstructs legacy entries at this boundary but stores named domain columns.
type EventState interface {
	Get(context.Context, string) (jetstream.KeyValueEntry, error)
	Put(context.Context, string, []byte) (uint64, error)
	Create(context.Context, string, []byte, ...jetstream.KVCreateOpt) (uint64, error)
	Update(context.Context, string, []byte, uint64) (uint64, error)
}

// InvitationIntentState retains new-response invite eligibility independently
// of processed-state classification, without inviting historical imports.
type InvitationIntentState interface {
	PrepareInvitation(context.Context, string) error
	InvitationPrepared(context.Context, string) (bool, error)
	ResolveInvitation(context.Context, string) error
}

// ResourceLocker serializes handlers for the same source key across replicas.
// The returned context owns the locked session and is valid until release;
// handlers must use it sequentially for state operations.
type ResourceLocker interface {
	LockResource(context.Context, string) (context.Context, func(), error)
}

// SourceFence checks the current authoritative source revision before effects.
type SourceFence interface {
	Current(context.Context) (bool, error)
}
type sourceFenceKey struct{}

// WithSourceFence associates one source delivery with its freshness check.
func WithSourceFence(ctx context.Context, fence SourceFence) context.Context {
	return context.WithValue(ctx, sourceFenceKey{}, fence)
}

// CheckSourceFence is a no-op for direct/unit callers without a delivery fence.
func CheckSourceFence(ctx context.Context) error {
	if fence, ok := ctx.Value(sourceFenceKey{}).(SourceFence); ok {
		current, err := fence.Current(ctx)
		if err != nil {
			return err
		}
		if !current {
			return ErrSourceSuperseded
		}
	}
	return nil
}
