package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics_Registered(t *testing.T) {
	if HTTPRequestsTotal == nil {
		t.Fatal("HTTPRequestsTotal is nil")
	}
	if HTTPRequestDuration == nil {
		t.Fatal("HTTPRequestDuration is nil")
	}
	if HTTPRequestSizeBytes == nil {
		t.Fatal("HTTPRequestSizeBytes is nil")
	}
	if TranscriptionTotal == nil {
		t.Fatal("TranscriptionTotal is nil")
	}
	if TranscriptionDuration == nil {
		t.Fatal("TranscriptionDuration is nil")
	}
	if AudioDuration == nil {
		t.Fatal("AudioDuration is nil")
	}
	if QueueDepth == nil {
		t.Fatal("QueueDepth is nil")
	}
	if QueueCapacity == nil {
		t.Fatal("QueueCapacity is nil")
	}
	if ModelLoaded == nil {
		t.Fatal("ModelLoaded is nil")
	}
}

func TestHTTPRequestsTotal_Inc(t *testing.T) {
	// Incrementing a counter vec with label values should not panic.
	HTTPRequestsTotal.WithLabelValues("GET", "/health", "200").Inc()
	HTTPRequestsTotal.WithLabelValues("POST", "/transcribe", "500").Inc()
}

func TestTranscriptionDuration_Observe(t *testing.T) {
	// Observing a value on a histogram vec should not panic.
	TranscriptionDuration.WithLabelValues("base").Observe(1.5)
	TranscriptionDuration.WithLabelValues("large").Observe(3.75)
}

func TestQueueDepth_SetAndGet(t *testing.T) {
	QueueDepth.Set(7)
	val := testutil.ToFloat64(QueueDepth)
	if val != 7 {
		t.Fatalf("expected QueueDepth to be 7, got %f", val)
	}

	QueueDepth.Set(0)
	val = testutil.ToFloat64(QueueDepth)
	if val != 0 {
		t.Fatalf("expected QueueDepth to be 0, got %f", val)
	}
}

func TestQueueCapacity_SetAndGet(t *testing.T) {
	QueueCapacity.Set(16)
	val := testutil.ToFloat64(QueueCapacity)
	if val != 16 {
		t.Fatalf("expected QueueCapacity to be 16, got %f", val)
	}

	QueueCapacity.Set(32)
	val = testutil.ToFloat64(QueueCapacity)
	if val != 32 {
		t.Fatalf("expected QueueCapacity to be 32, got %f", val)
	}
}
