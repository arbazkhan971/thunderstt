package queue

import (
	"testing"
	"time"

	"github.com/arbaz/thunderstt/internal/engine"
)

func TestNewJob(t *testing.T) {
	t.Parallel()

	j := NewJob("job-001")

	if j.ID != "job-001" {
		t.Errorf("ID = %q, want %q", j.ID, "job-001")
	}
	if got := j.Status(); got != StatusPending {
		t.Errorf("Status() = %v, want %v (StatusPending)", got, StatusPending)
	}
	if j.Result != nil {
		t.Errorf("Result should be nil on a new job, got %+v", j.Result)
	}
	if j.Error != nil {
		t.Errorf("Error should be nil on a new job, got %v", j.Error)
	}
}

func TestJob_StatusTransitions(t *testing.T) {
	t.Parallel()

	j := NewJob("job-002")

	// Pending -> Running
	j.setRunning()
	if got := j.Status(); got != StatusRunning {
		t.Errorf("after setRunning(): Status() = %v, want %v (StatusRunning)", got, StatusRunning)
	}

	// Running -> Done (with result)
	result := &engine.Result{Language: "en", Duration: 2.0}
	j.finish(result, nil)

	if got := j.Status(); got != StatusDone {
		t.Errorf("after finish(): Status() = %v, want %v (StatusDone)", got, StatusDone)
	}
	if j.Result != result {
		t.Errorf("Result = %+v, want %+v", j.Result, result)
	}
	if j.Error != nil {
		t.Errorf("Error = %v, want nil", j.Error)
	}
}

func TestJob_StatusTransitions_withError(t *testing.T) {
	t.Parallel()

	j := NewJob("job-003")
	j.setRunning()

	wantErr := &engine.ErrEngineNotFound{Name: "missing"}
	j.finish(nil, wantErr)

	if got := j.Status(); got != StatusDone {
		t.Errorf("Status() = %v, want %v (StatusDone)", got, StatusDone)
	}
	if j.Result != nil {
		t.Errorf("Result = %+v, want nil", j.Result)
	}
	if j.Error != wantErr {
		t.Errorf("Error = %v, want %v", j.Error, wantErr)
	}
}

func TestJob_Done(t *testing.T) {
	t.Parallel()

	j := NewJob("job-004")

	// Done channel should not be closed yet.
	select {
	case <-j.Done():
		t.Fatal("Done() channel should not be closed before finish()")
	default:
		// expected
	}

	// Finish the job and verify the channel closes.
	result := &engine.Result{Language: "de", Duration: 3.0}
	j.finish(result, nil)

	select {
	case <-j.Done():
		// expected: channel is closed
	case <-time.After(time.Second):
		t.Fatal("Done() channel should be closed after finish(), timed out waiting")
	}

	// Reading from the Done channel again should succeed immediately (closed channel).
	select {
	case <-j.Done():
		// expected
	default:
		t.Fatal("Done() channel should remain closed on subsequent reads")
	}
}

func TestStatus_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status Status
		want   string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusDone, "done"},
		{Status(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.status.String(); got != tt.want {
				t.Errorf("Status(%d).String() = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
