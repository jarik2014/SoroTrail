package meta

import (
	"testing"
	"time"
)

// TestNewWorkerAppliesDefaults pins every default the constructor applies, so a
// default changed by accident fails here instead of passing unnoticed (#1136).
func TestNewWorkerAppliesDefaults(t *testing.T) {
	w := NewWorker()

	if w == nil {
		t.Fatal("NewWorker returned nil")
	}
	if got, want := w.Interval(), DefaultRefreshInterval; got != want {
		t.Errorf("Interval() = %v, want %v", got, want)
	}
	if got, want := w.BatchSize(), DefaultBatchSize; got != want {
		t.Errorf("BatchSize() = %d, want %d", got, want)
	}
	if got, want := w.FetchTimeout(), DefaultFetchTimeout; got != want {
		t.Errorf("FetchTimeout() = %v, want %v", got, want)
	}
}

// The documented values themselves are part of the contract: a caller may rely
// on them without overriding anything.
func TestDefaultValuesAreStable(t *testing.T) {
	if DefaultRefreshInterval != 5*time.Minute {
		t.Errorf("DefaultRefreshInterval = %v, want 5m", DefaultRefreshInterval)
	}
	if DefaultBatchSize != 50 {
		t.Errorf("DefaultBatchSize = %d, want 50", DefaultBatchSize)
	}
	if DefaultFetchTimeout != 10*time.Second {
		t.Errorf("DefaultFetchTimeout = %v, want 10s", DefaultFetchTimeout)
	}
}

func TestNewWorkerWithOptionsOverridesEveryField(t *testing.T) {
	w := NewWorkerWithOptions(Options{
		Interval:     time.Minute,
		BatchSize:    7,
		FetchTimeout: 3 * time.Second,
	})

	if got := w.Interval(); got != time.Minute {
		t.Errorf("Interval() = %v, want 1m", got)
	}
	if got := w.BatchSize(); got != 7 {
		t.Errorf("BatchSize() = %d, want 7", got)
	}
	if got := w.FetchTimeout(); got != 3*time.Second {
		t.Errorf("FetchTimeout() = %v, want 3s", got)
	}
}

// A zero value must mean "keep the default", never "disable". Otherwise an
// unset config field would turn the worker into a busy loop with no timeout.
func TestNewWorkerWithOptionsZeroKeepsDefaults(t *testing.T) {
	w := NewWorkerWithOptions(Options{})

	if got := w.Interval(); got != DefaultRefreshInterval {
		t.Errorf("Interval() = %v, want default %v", got, DefaultRefreshInterval)
	}
	if got := w.BatchSize(); got != DefaultBatchSize {
		t.Errorf("BatchSize() = %d, want default %d", got, DefaultBatchSize)
	}
	if got := w.FetchTimeout(); got != DefaultFetchTimeout {
		t.Errorf("FetchTimeout() = %v, want default %v", got, DefaultFetchTimeout)
	}
}

func TestNewWorkerWithOptionsNegativeValuesAreIgnored(t *testing.T) {
	w := NewWorkerWithOptions(Options{
		Interval:     -time.Second,
		BatchSize:    -1,
		FetchTimeout: -time.Minute,
	})

	if got := w.Interval(); got != DefaultRefreshInterval {
		t.Errorf("Interval() = %v, want default %v", got, DefaultRefreshInterval)
	}
	if got := w.BatchSize(); got != DefaultBatchSize {
		t.Errorf("BatchSize() = %d, want default %d", got, DefaultBatchSize)
	}
	if got := w.FetchTimeout(); got != DefaultFetchTimeout {
		t.Errorf("FetchTimeout() = %v, want default %v", got, DefaultFetchTimeout)
	}
}

// Partial overrides must leave the other two fields at their defaults.
func TestNewWorkerWithOptionsPartialOverride(t *testing.T) {
	w := NewWorkerWithOptions(Options{BatchSize: 3})

	if got := w.BatchSize(); got != 3 {
		t.Errorf("BatchSize() = %d, want 3", got)
	}
	if got := w.Interval(); got != DefaultRefreshInterval {
		t.Errorf("Interval() = %v, want default %v", got, DefaultRefreshInterval)
	}
	if got := w.FetchTimeout(); got != DefaultFetchTimeout {
		t.Errorf("FetchTimeout() = %v, want default %v", got, DefaultFetchTimeout)
	}
}

// Each call must return an independent worker: sharing mutable state between
// goroutines would be a data race waiting to happen.
func TestNewWorkerReturnsIndependentInstances(t *testing.T) {
	first := NewWorker()
	second := NewWorkerWithOptions(Options{BatchSize: 1})

	if first == second {
		t.Fatal("NewWorker returned the same pointer twice")
	}
	if first.BatchSize() == second.BatchSize() {
		t.Errorf("instances share state: both have BatchSize %d", first.BatchSize())
	}
	if got := first.BatchSize(); got != DefaultBatchSize {
		t.Errorf("overriding one instance changed another: BatchSize() = %d", got)
	}
}
