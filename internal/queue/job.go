// Package queue provides a bounded, context-aware job queue for scheduling
// transcription work with concurrency control.
package queue

import (
	"sync/atomic"

	"github.com/arbaz/thunderstt/internal/engine"
)

// Status represents the lifecycle state of a queued job.
type Status int32

const (
	// StatusPending means the job is waiting for a worker slot.
	StatusPending Status = iota
	// StatusRunning means the job is currently being executed.
	StatusRunning
	// StatusDone means the job completed (check Error for failures).
	StatusDone
)

// String returns a human-readable status label.
func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusDone:
		return "done"
	default:
		return "unknown"
	}
}

// Job represents a single transcription unit of work submitted to the queue.
type Job struct {
	// ID is a unique identifier for this job (typically the request ID).
	ID string

	// status tracks the job's lifecycle state atomically.
	status atomic.Int32

	// Result holds the transcription output once the job completes.
	Result *engine.Result

	// Error holds any error that occurred during execution.
	Error error

	// done is closed when the job finishes (successfully or with error).
	done chan struct{}
}

// NewJob creates a job with the given ID in pending state.
func NewJob(id string) *Job {
	j := &Job{
		ID:   id,
		done: make(chan struct{}),
	}
	j.status.Store(int32(StatusPending))
	return j
}

// Status returns the current job status.
func (j *Job) Status() Status {
	return Status(j.status.Load())
}

// Done returns a channel that is closed when the job completes.
func (j *Job) Done() <-chan struct{} {
	return j.done
}

// setRunning transitions the job to running state.
func (j *Job) setRunning() {
	j.status.Store(int32(StatusRunning))
}

// finish records the result and/or error, transitions to done, and
// closes the done channel.
func (j *Job) finish(result *engine.Result, err error) {
	j.Result = result
	j.Error = err
	j.status.Store(int32(StatusDone))
	close(j.done)
}
