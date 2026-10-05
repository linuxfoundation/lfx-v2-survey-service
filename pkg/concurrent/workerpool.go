// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package concurrent

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// WorkerPool represents a pool of workers that can process jobs concurrently
type WorkerPool struct {
	workerCount int
}

// Run executes all functions using errgroup with goroutine limiting
// Returns the first error encountered, and cancels remaining work
func (wp *WorkerPool) Run(ctx context.Context, functions ...func() error) error {
	if len(functions) == 0 {
		return nil
	}

	// Create errgroup with context
	g, groupCtx := errgroup.WithContext(ctx)

	// Set the limit of concurrent goroutines
	g.SetLimit(wp.workerCount)

	// Submit all functions to the errgroup
	for _, fn := range functions {
		g.Go(func() error {
			// Check if context was cancelled before starting
			select {
			case <-groupCtx.Done():
				return groupCtx.Err()
			default:
			}

			return fn()
		})
	}

	// Wait for all functions to complete and return first error
	return g.Wait()
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workerCount int) *WorkerPool {
	if workerCount <= 0 {
		workerCount = 1
	}
	return &WorkerPool{
		workerCount: workerCount,
	}
}

// defaultBatchWorkers is the number of goroutines BatchMap uses internally.
// It matches the fan-out previously hard-coded at each call site.
const defaultBatchWorkers = 5

// BatchMap runs fn concurrently over every element of items using a pool of
// defaultBatchWorkers goroutines, collecting results into a slice that mirrors
// the input index order.
// It returns on the first error; partial results are discarded.
// If items is empty or nil, a non-nil empty slice is returned immediately.
func BatchMap[In, Out any](ctx context.Context, items []In, fn func(In) (Out, error)) ([]Out, error) {
	if len(items) == 0 {
		return make([]Out, 0), nil
	}

	result := make([]Out, len(items))
	fns := make([]func() error, len(items))
	for i, item := range items {
		fns[i] = func() error {
			r, err := fn(item)
			if err != nil {
				return err
			}
			result[i] = r
			return nil
		}
	}

	if err := NewWorkerPool(defaultBatchWorkers).Run(ctx, fns...); err != nil {
		return nil, err
	}
	return result, nil
}
