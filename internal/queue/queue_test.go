package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arbaz/thunderstt/internal/engine"
)

func TestNewQueue_defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"zero gets clamped to 1", 0, 1},
		{"negative gets clamped to 1", -5, 1},
		{"one stays 1", 1, 1},
		{"positive stays unchanged", 4, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := NewQueue(tt.input)
			if got := q.Cap(); got != tt.want {
				t.Errorf("NewQueue(%d).Cap() = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestQueue_Submit_success(t *testing.T) {
	t.Parallel()

	q := NewQueue(2)
	want := &engine.Result{Language: "en", Duration: 1.5}

	got, err := q.Submit(context.Background(), func() (*engine.Result, error) {
		return want, nil
	})

	if err != nil {
		t.Fatalf("Submit returned unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("Submit result = %+v, want %+v", got, want)
	}
}

func TestQueue_Submit_error(t *testing.T) {
	t.Parallel()

	q := NewQueue(1)
	wantErr := errors.New("transcription failed")

	got, err := q.Submit(context.Background(), func() (*engine.Result, error) {
		return nil, wantErr
	})

	if got != nil {
		t.Errorf("Submit result = %+v, want nil", got)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Submit error = %v, want %v", err, wantErr)
	}
}

func TestQueue_Submit_panic(t *testing.T) {
	t.Parallel()

	q := NewQueue(1)

	got, err := q.Submit(context.Background(), func() (*engine.Result, error) {
		panic("something went terribly wrong")
	})

	if got != nil {
		t.Errorf("Submit result = %+v, want nil on panic", got)
	}
	if err == nil {
		t.Fatal("Submit should return an error when the function panics")
	}
	if want := "panic recovered"; !containsSubstring(err.Error(), want) {
		t.Errorf("error %q should contain %q", err.Error(), want)
	}
}

func TestQueue_Submit_contextCancelled(t *testing.T) {
	t.Parallel()

	// Create a queue with capacity 1 and occupy the only slot with a
	// long-running job so the next Submit must wait.
	q := NewQueue(1)

	blocker := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		_, _ = q.Submit(context.Background(), func() (*engine.Result, error) {
			<-blocker // hold the slot
			return nil, nil
		})
	}()

	// Give the goroutine a moment to acquire the slot.
	time.Sleep(50 * time.Millisecond)

	// Now submit with an already-cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	got, err := q.Submit(ctx, func() (*engine.Result, error) {
		t.Error("work function should not have been called")
		return nil, nil
	})

	if got != nil {
		t.Errorf("Submit result = %+v, want nil", got)
	}
	if err == nil {
		t.Fatal("Submit should return an error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should wrap context.Canceled, got: %v", err)
	}

	// Unblock the first goroutine and wait for it to finish.
	close(blocker)
	wg.Wait()
}

func TestQueue_BoundedConcurrency(t *testing.T) {
	t.Parallel()

	const maxConcurrent = 3
	const totalJobs = 10

	q := NewQueue(maxConcurrent)

	var running atomic.Int32
	var peak atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < totalJobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = q.Submit(context.Background(), func() (*engine.Result, error) {
				cur := running.Add(1)
				// Track peak concurrency.
				for {
					old := peak.Load()
					if cur <= old || peak.CompareAndSwap(old, cur) {
						break
					}
				}
				// Simulate work so overlapping goroutines actually overlap.
				time.Sleep(20 * time.Millisecond)
				running.Add(-1)
				return &engine.Result{Language: "en", Duration: 0.5}, nil
			})
		}()
	}

	wg.Wait()

	if p := peak.Load(); p > int32(maxConcurrent) {
		t.Errorf("peak concurrency = %d, want <= %d", p, maxConcurrent)
	}
	// With 10 jobs and 3 slots, we expect at least 2 concurrent jobs at some point.
	if p := peak.Load(); p < 2 {
		t.Logf("warning: peak concurrency was only %d (expected >= 2 with %d jobs and %d slots)", p, totalJobs, maxConcurrent)
	}
}

func TestQueue_Len_Cap(t *testing.T) {
	t.Parallel()

	q := NewQueue(5)

	if got := q.Cap(); got != 5 {
		t.Errorf("Cap() = %d, want 5", got)
	}
	if got := q.Len(); got != 0 {
		t.Errorf("Len() on empty queue = %d, want 0", got)
	}

	// Occupy one slot and check Len.
	blocker := make(chan struct{})
	started := make(chan struct{})

	go func() {
		_, _ = q.Submit(context.Background(), func() (*engine.Result, error) {
			close(started)
			<-blocker
			return nil, nil
		})
	}()

	<-started // wait until the job is running

	if got := q.Len(); got != 1 {
		t.Errorf("Len() with 1 active job = %d, want 1", got)
	}
	if got := q.Cap(); got != 5 {
		t.Errorf("Cap() should remain 5, got %d", got)
	}

	close(blocker)
	// Give the goroutine time to release the slot.
	time.Sleep(50 * time.Millisecond)

	if got := q.Len(); got != 0 {
		t.Errorf("Len() after job completes = %d, want 0", got)
	}
}

func TestQueue_Drain_empty(t *testing.T) {
	t.Parallel()

	q := NewQueue(2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := q.Drain(ctx)
	if err != nil {
		t.Fatalf("drain on empty queue: %v", err)
	}
}

func TestQueue_Drain_waitsForJobs(t *testing.T) {
	t.Parallel()

	q := NewQueue(2)
	started := make(chan struct{})

	// Submit a slow job.
	go q.Submit(context.Background(), func() (*engine.Result, error) {
		close(started)
		time.Sleep(200 * time.Millisecond)
		return &engine.Result{}, nil
	})

	<-started // wait for job to start

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := q.Drain(ctx)
	if err != nil {
		t.Fatalf("drain should succeed: %v", err)
	}
}

// containsSubstring is a small helper to avoid importing strings in tests.
func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
