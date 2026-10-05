// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package concurrent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPool_Run(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(2)

	var counter int64
	functions := []func() error{
		func() error {
			atomic.AddInt64(&counter, 1)
			time.Sleep(10 * time.Millisecond) // Simulate work
			return nil
		},
		func() error {
			atomic.AddInt64(&counter, 2)
			time.Sleep(10 * time.Millisecond)
			return nil
		},
		func() error {
			atomic.AddInt64(&counter, 3)
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	err := pool.Run(ctx, functions...)
	require.NoError(t, err)
	assert.Equal(t, int64(6), atomic.LoadInt64(&counter))
}

func TestWorkerPool_Run_WithError(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(2)

	expectedError := errors.New("job failed")
	functions := []func() error{
		func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
		func() error {
			time.Sleep(5 * time.Millisecond)
			return expectedError
		},
		func() error {
			time.Sleep(20 * time.Millisecond)
			return nil
		},
	}

	err := pool.Run(ctx, functions...)
	require.Error(t, err)
	assert.Equal(t, expectedError, err)
}

func TestWorkerPool_Run_EmptyFunctions(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(2)

	err := pool.Run(ctx)
	require.NoError(t, err)
}

func TestWorkerPool_Run_WithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pool := NewWorkerPool(2)

	// Cancel context immediately
	cancel()

	functions := []func() error{
		func() error {
			return nil
		},
	}

	err := pool.Run(ctx, functions...)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestBatchMap_HappyPath(t *testing.T) {
	ctx := context.Background()
	in := []int{1, 2, 3, 4, 5}

	out, err := BatchMap(ctx, in, func(n int) (int, error) {
		return n * 2, nil
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []int{2, 4, 6, 8, 10}, out)
}

func TestBatchMap_EmptyInput(t *testing.T) {
	ctx := context.Background()

	out, err := BatchMap(ctx, []string{}, func(s string) (string, error) {
		return s, nil
	})

	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Empty(t, out)
}

func TestBatchMap_NilInput(t *testing.T) {
	ctx := context.Background()

	out, err := BatchMap(ctx, nil, func(s string) (string, error) {
		return s, nil
	})

	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Empty(t, out)
}

func TestBatchMap_PropagatesFirstError(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("mapping failed")
	in := []int{1, 2, 3}

	_, err := BatchMap(ctx, in, func(n int) (int, error) {
		if n == 2 {
			return 0, boom
		}
		return n, nil
	})

	require.Error(t, err)
	assert.Equal(t, boom, err)
}

func TestBatchMap_PreservesIndexOrder(t *testing.T) {
	ctx := context.Background()
	in := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}

	out, err := BatchMap(ctx, in, func(s string) (string, error) {
		return s + s, nil
	})

	require.NoError(t, err)
	require.Len(t, out, len(in))
	for i, s := range in {
		assert.Equal(t, s+s, out[i], "index %d mismatch", i)
	}
}
