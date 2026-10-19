package meta

import (
	"context"
	"sync"
	"time"
)

// Metadata represents resolved contract token information or negative cache marker.
type Metadata struct {
	Name     string
	Symbol   string
	Decimals uint32
	IsToken  bool
}

type RPCClient interface {
	GetMetadata(ctx context.Context, contractID string) (Metadata, error)
}

type MetadataStore interface {
	Get(contractID string) (Metadata, bool)
	Put(contractID string, m Metadata, t time.Time)
}

// Defaults applied by NewWorker. They are exported so callers and tests can
// refer to them instead of duplicating magic numbers.
const (
	// DefaultRefreshInterval is how often the worker re-reads token metadata.
	DefaultRefreshInterval = 5 * time.Minute

	// DefaultBatchSize is how many contracts are refreshed per pass.
	DefaultBatchSize = 50

	// DefaultFetchTimeout bounds a single RPC metadata call.
	DefaultFetchTimeout = 10 * time.Second
)

// Options overrides the worker defaults. A zero value keeps the default, so
// callers only set what they care about.
type Options struct {
	Interval     time.Duration
	BatchSize    int
	FetchTimeout time.Duration
}

// Worker orchestrates contract metadata background refresh.
type Worker struct {
	interval     time.Duration
	batchSize    int
	fetchTimeout time.Duration
}

// NewWorker returns a worker with the package defaults applied.
func NewWorker() *Worker {
	return NewWorkerWithOptions(Options{})
}

// NewWorkerWithOptions returns a worker, replacing defaults only where the
// caller supplied a positive value. Validation stays here rather than in every
// caller: a zero or negative field is far more likely to be an unset option
// than a deliberate "no interval at all".
func NewWorkerWithOptions(opts Options) *Worker {
	w := &Worker{
		interval:     DefaultRefreshInterval,
		batchSize:    DefaultBatchSize,
		fetchTimeout: DefaultFetchTimeout,
	}

	if opts.Interval > 0 {
		w.interval = opts.Interval
	}
	if opts.BatchSize > 0 {
		w.batchSize = opts.BatchSize
	}
	if opts.FetchTimeout > 0 {
		w.fetchTimeout = opts.FetchTimeout
	}

	return w
}

// Interval reports how often the worker refreshes metadata.
func (w *Worker) Interval() time.Duration {
	return w.interval
}

// BatchSize reports how many contracts one pass covers.
func (w *Worker) BatchSize() int {
	return w.batchSize
}

// FetchTimeout reports the per-call RPC timeout.
func (w *Worker) FetchTimeout() time.Duration {
	return w.fetchTimeout
}

// ResolveMetadata resolves contract metadata with caching and interval checks.
func ResolveMetadata(ctx context.Context, rpc RPCClient, store MetadataStore, contractID string) (Metadata, error) {
	if m, ok := store.Get(contractID); ok {
		return m, nil
	}
	meta, err := rpc.GetMetadata(ctx, contractID)
	if err != nil {
		return Metadata{}, err
	}
	store.Put(contractID, meta, time.Now())
	return meta, nil
}

// ResolveMetadataWithFallbacks resolves metadata and falls back to existing store state on RPC failure.
func ResolveMetadataWithFallbacks(ctx context.Context, rpc RPCClient, store MetadataStore, contractID string) (Metadata, error) {
	meta, err := rpc.GetMetadata(ctx, contractID)
	if err != nil {
		if existing, ok := store.Get(contractID); ok {
			return existing, nil
		}
		return Metadata{}, err
	}
	store.Put(contractID, meta, time.Now())
	return meta, nil
}

// ResolveMetadataCachedSingleFlight ensures concurrent fetches for the same contract execute once.
func ResolveMetadataCachedSingleFlight(ctx context.Context, rpc RPCClient, store MetadataStore, contractID string) (Metadata, error) {
	if m, ok := store.Get(contractID); ok {
		return m, nil
	}
	// Simple single-flight simulation for test/robustness
	val, _ := workerSingleFlightMap.LoadOrStore(contractID, &singleFlightItem{
		once: &sync.Once{},
	})
	item := val.(*singleFlightItem)

	var meta Metadata
	var err error
	item.once.Do(func() {
		meta, err = rpc.GetMetadata(ctx, contractID)
		if err == nil {
			store.Put(contractID, meta, time.Now())
		}
		item.meta = meta
		item.err = err
	})

	return item.meta, item.err
}

type singleFlightItem struct {
	once *sync.Once
	meta Metadata
	err  error
}

var workerSingleFlightMap sync.Map
