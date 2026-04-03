package queue

import (
	"context"
	"fmt"
	"sync"

	"github.com/arbaz/thunderstt/internal/engine"
)

// Queue provides bounded concurrency for transcription jobs. It uses a
// semaphore (buffered channel) to limit the number of goroutines that can
// execute transcription work simultaneously.
type Queue struct {
	// sem is a counting semaphore implemented as a buffered channel.
	// Its capacity equals maxConcurrent.
	sem chan struct{}

	// activeJobs tracks the number of in-flight jobs so Drain can
	// block until they all complete.
	activeJobs sync.WaitGroup
}

// NewQueue creates a queue that allows at most maxConcurrent jobs to run
// in parallel. maxConcurrent must be >= 1.
func NewQueue(maxConcurrent int) *Queue {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Queue{
		sem: make(chan struct{}, maxConcurrent),
	}
}

// Submit enqueues a transcription function for execution and blocks until
// the function completes or the context is cancelled. The caller's context
// is used for both the queue-wait phase and as a signal to the work function.
//
// If the context expires before a worker slot becomes available, Submit
// returns immediately with a context error. If the work function panics,
// the panic is recovered and returned as an error.
func (q *Queue) Submit(ctx context.Context, fn func() (*engine.Result, error)) (*engine.Result, error) {
	// Acquire a semaphore slot, respecting context cancellation.
	select {
	case q.sem <- struct{}{}:
		// Slot acquired.
	case <-ctx.Done():
		return nil, fmt.Errorf("queue: context cancelled while waiting for worker slot: %w", ctx.Err())
	}

	// Track this job for graceful drain.
	q.activeJobs.Add(1)

	// Ensure the slot is released and the job is marked done when work completes.
	defer func() {
		<-q.sem
		q.activeJobs.Done()
	}()

	// Check context again after acquiring the slot in case it expired
	// while we were waiting.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("queue: context cancelled after acquiring slot: %w", err)
	}

	// Execute the work function with panic recovery.
	return q.execute(fn)
}

// execute runs the work function with panic recovery.
func (q *Queue) execute(fn func() (*engine.Result, error)) (result *engine.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("queue: panic recovered during job execution: %v", r)
			result = nil
		}
	}()

	return fn()
}

// Drain blocks until all in-flight jobs complete or the context expires.
// It is intended to be called during graceful shutdown so that queued
// transcription work can finish before the process exits.
func (q *Queue) Drain(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		q.activeJobs.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Len returns the number of worker slots currently in use. This is useful
// for metrics and diagnostics.
func (q *Queue) Len() int {
	return len(q.sem)
}

// Cap returns the maximum number of concurrent workers.
func (q *Queue) Cap() int {
	return cap(q.sem)
}
