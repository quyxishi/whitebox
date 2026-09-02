// Package xraypool caches expensive, closable resources behind a refcounted
// keyed cache.
//
// It exists because xray-core's splithttp, grpc and hysteria transports cache
// per-connection state in package-level maps keyed by the *pointer* of the
// `*internet.MemoryStreamConfig` that `core.New` allocates, and never delete
// from those maps. Constructing one xray instance per probe therefore inserts a
// permanent entry per probe. Reusing instances keeps that pointer - and so those
// maps - bounded at the number of distinct configurations.
//
// The pool is deliberately generic over io.Closer and carries no xray
// dependency, so its interesting properties (singleflight, refcounting,
// eviction ordering, never-close-while-in-use) are testable against a fake.
package xraypool

import (
	"container/list"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ErrClosed is returned by Acquire once the pool has been shut down
var ErrClosed = errors.New("xraypool: pool is closed")

const (
	DefaultTTL = 10 * time.Minute

	// DefaultMaxEntries is deliberately modest: every entry holds a started
	// instance and whatever it allocated, so a large cap is its own memory
	// problem
	DefaultMaxEntries = 64

	maxSweepInterval = 30 * time.Second
	minSweepInterval = time.Second
)

// Options configures pool behaviour. The zero value is not usable; run it
// through Normalized first, which New and SetOptions do for you
type Options struct {
	// Enabled false turns the pool into a pass-through: every Acquire builds a
	// fresh value and Release closes it, so call sites stay identical
	Enabled bool

	// TTL is the idle time after which an unreferenced entry is closed
	TTL time.Duration

	// MaxEntries caps live entries. In-use entries are never evicted to satisfy
	// it; the cap is exceeded instead and Overflows is incremented
	MaxEntries int
}

// Normalized fills in defaults for unset fields
func (o Options) Normalized() Options {
	if o.TTL <= 0 {
		o.TTL = DefaultTTL
	}
	if o.MaxEntries <= 0 {
		o.MaxEntries = DefaultMaxEntries
	}

	return o
}

// Stats is a point-in-time snapshot of pool activity
type Stats struct {
	Size  int
	InUse int

	Hits   uint64
	Misses uint64

	EvictionsTTL      uint64
	EvictionsLRU      uint64
	EvictionsReload   uint64
	EvictionsShutdown uint64

	Overflows     uint64
	BuildFailures uint64
	CloseErrors   uint64
}

type entry[T io.Closer] struct {
	key string

	// `ready` is closed once build has returned; val/err/built are only safe to
	// read after it is closed
	ready chan struct{}
	val   T
	err   error
	built bool

	refs     int
	lastUsed time.Time
	elem     *list.Element

	// `detached` means the entry is no longer reachable from the map or the LRU
	// list. it is closed by whoever observes refs == 0 afterwards
	detached bool
	closed   bool
}

// Pool is a keyed, refcounted cache of io.Closer values
type Pool[T io.Closer] struct {
	mu      sync.Mutex
	opts    Options
	entries map[string]*entry[T]
	lru     *list.List // front = most recently used
	closed  bool

	hits              uint64
	misses            uint64
	evictionsTTL      uint64
	evictionsLRU      uint64
	evictionsReload   uint64
	evictionsShutdown uint64
	overflows         uint64
	buildFailures     uint64
	closeErrors       uint64

	buildDuration prometheus.Histogram

	// `now` is swappable so tests can drive TTL without sleeping
	now func() time.Time

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// New returns a running pool. Call Close to release its sweeper goroutine and
// every value it holds
func New[T io.Closer](opts Options) *Pool[T] {
	p := &Pool[T]{
		opts:    opts.Normalized(),
		entries: make(map[string]*entry[T]),
		lru:     list.New(),
		now:     time.Now,
		stop:    make(chan struct{}),
		buildDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "whitebox_xray_instance_build_duration_seconds",
			Help:    "Time spent constructing and starting an xray instance on a cache miss",
			Buckets: prometheus.ExponentialBuckets(0.001, 3, 8),
		}),
	}

	p.wg.Add(1)
	go p.sweepLoop()

	return p
}

// Acquire returns a lease on the value for key, building it with `build` on a
// miss. Concurrent Acquires for the same key share a single build. The caller
// must Release the returned lease
func (p *Pool[T]) Acquire(ctx context.Context, key string, build func(context.Context) (T, error)) (*Lease[T], error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, ErrClosed
	}

	if !p.opts.Enabled {
		p.misses++
		p.mu.Unlock()

		return p.acquireDetached(ctx, build)
	}

	if e, found := p.entries[key]; found {
		e.refs++
		e.lastUsed = p.now()
		p.lru.MoveToFront(e.elem)
		p.hits++
		p.mu.Unlock()

		select {
		case <-e.ready:
		case <-ctx.Done():
			p.release(e)
			return nil, ctx.Err()
		}

		if e.err != nil {
			p.release(e)
			return nil, e.err
		}

		return &Lease[T]{p: p, e: e, cached: true}, nil
	}

	e := &entry[T]{key: key, ready: make(chan struct{}), refs: 1, lastUsed: p.now()}
	p.entries[key] = e
	e.elem = p.lru.PushFront(e)
	p.misses++

	// evict now so the cap is enforced before we spend time building
	victims := p.evictOverCapLocked()
	p.mu.Unlock()

	p.closeAll(victims)

	// never build while holding p.mu: core.New for a wireguard outbound stands
	// up a gVisor netstack, which would serialize every concurrent scrape
	v, err := p.build(ctx, build)
	e.val, e.err, e.built = v, err, err == nil
	close(e.ready)

	if err != nil {
		// a failed build must not be cached, or one transient failure poisons
		// the key for a whole TTL
		p.mu.Lock()
		p.detachLocked(e)
		p.mu.Unlock()

		p.release(e)
		return nil, err
	}

	return &Lease[T]{p: p, e: e}, nil
}

// AcquireUncached builds a value that never enters the cache, whatever the
// pool's options say, and closes it on Release.
//
// It exists for values that must not outlive the request that built them. The
// returned lease behaves exactly like a pooled one, so call sites differ only
// in which method they call
func (p *Pool[T]) AcquireUncached(ctx context.Context, build func(context.Context) (T, error)) (*Lease[T], error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, ErrClosed
	}

	// counted as a miss like the disabled path is, so misses stays a count of
	// values actually constructed
	p.misses++
	p.mu.Unlock()

	return p.acquireDetached(ctx, build)
}

// acquireDetached builds a value outside the map and the LRU list. The lease
// owns what it built - it holds the only reference, so Release closes it - and
// so the call site does not care how the value was obtained
func (p *Pool[T]) acquireDetached(ctx context.Context, build func(context.Context) (T, error)) (*Lease[T], error) {
	v, err := p.build(ctx, build)
	if err != nil {
		return nil, err
	}

	e := &entry[T]{ready: closedChan(), val: v, built: true, refs: 1, detached: true}
	return &Lease[T]{p: p, e: e}, nil
}

func (p *Pool[T]) build(ctx context.Context, build func(context.Context) (T, error)) (T, error) {
	start := p.now()
	v, err := build(ctx)
	p.buildDuration.Observe(p.now().Sub(start).Seconds())

	if err != nil {
		p.mu.Lock()
		p.buildFailures++
		p.mu.Unlock()
	}

	return v, err
}

// SetOptions applies a new configuration, typically on SIGHUP
func (p *Pool[T]) SetOptions(o Options) {
	o = o.Normalized()

	p.mu.Lock()
	prev := p.opts
	p.opts = o

	var victims []T
	if prev.Enabled && !o.Enabled {
		// in-flight probes keep running on the instance they hold; it closes
		// once they release it
		victims = p.detachAllLocked(&p.evictionsReload)
	} else {
		victims = p.evictOverCapLocked()
	}
	p.mu.Unlock()

	p.closeAll(victims)
}

// Close stops the sweeper and closes every value the pool still owns. Values
// currently leased out are closed by their final Release
func (p *Pool[T]) Close() error {
	p.stopOnce.Do(func() { close(p.stop) })
	p.wg.Wait()

	p.mu.Lock()
	p.closed = true
	victims := p.detachAllLocked(&p.evictionsShutdown)
	p.mu.Unlock()

	p.closeAll(victims)
	return nil
}

// Stats returns a snapshot of pool activity
func (p *Pool[T]) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()

	inUse := 0
	for _, e := range p.entries {
		if e.refs > 0 {
			inUse++
		}
	}

	return Stats{
		Size:              len(p.entries),
		InUse:             inUse,
		Hits:              p.hits,
		Misses:            p.misses,
		EvictionsTTL:      p.evictionsTTL,
		EvictionsLRU:      p.evictionsLRU,
		EvictionsReload:   p.evictionsReload,
		EvictionsShutdown: p.evictionsShutdown,
		Overflows:         p.overflows,
		BuildFailures:     p.buildFailures,
		CloseErrors:       p.closeErrors,
	}
}

// * eviction and lifecycle. every *Locked helper requires p.mu held, and none of
//   them ever calls Close - they hand victims back to be closed unlocked,
//   because Close on a real instance fans out to every xray feature

func (p *Pool[T]) detachLocked(e *entry[T]) {
	if e.detached {
		return
	}

	e.detached = true

	if e.elem != nil {
		p.lru.Remove(e.elem)
		e.elem = nil
	}
	if cur, found := p.entries[e.key]; found && cur == e {
		delete(p.entries, e.key)
	}
}

// takeLocked claims the right to close e, if it is detached and unreferenced
func (p *Pool[T]) takeLocked(e *entry[T]) (T, bool) {
	var zero T

	if e.closed || !e.detached || e.refs > 0 {
		return zero, false
	}

	e.closed = true
	if !e.built {
		// build failed or never ran: nothing was constructed to close
		return zero, false
	}

	return e.val, true
}

func (p *Pool[T]) evictOverCapLocked() []T {
	if p.opts.MaxEntries <= 0 {
		return nil
	}

	var victims []T

	for p.lru.Len() > p.opts.MaxEntries {
		var target *entry[T]
		for el := p.lru.Back(); el != nil; el = el.Prev() {
			if e := el.Value.(*entry[T]); e.refs == 0 {
				target = e
				break
			}
		}

		if target == nil {
			// everything is in use. exceeding the cap is the only safe option,
			// and it is bounded by the number of concurrent probes
			p.overflows++
			slog.Warn("xraypool: over capacity, every entry is in use",
				"size", p.lru.Len(), "maxEntries", p.opts.MaxEntries)
			break
		}

		p.detachLocked(target)
		p.evictionsLRU++

		if v, ok := p.takeLocked(target); ok {
			victims = append(victims, v)
		}
	}

	return victims
}

func (p *Pool[T]) detachAllLocked(counter *uint64) []T {
	var victims []T

	for el := p.lru.Front(); el != nil; {
		next := el.Next()
		e := el.Value.(*entry[T])

		p.detachLocked(e)
		*counter++

		if v, ok := p.takeLocked(e); ok {
			victims = append(victims, v)
		}

		el = next
	}

	return victims
}

func (p *Pool[T]) release(e *entry[T]) {
	p.mu.Lock()

	if e.refs > 0 {
		e.refs--
	}
	if !e.detached {
		e.lastUsed = p.now()
	}

	v, ok := p.takeLocked(e)
	p.mu.Unlock()

	if ok {
		p.closeValue(v)
	}
}

func (p *Pool[T]) closeValue(v T) {
	if err := v.Close(); err != nil {
		p.mu.Lock()
		p.closeErrors++
		p.mu.Unlock()

		slog.Error("xraypool: failed to close pooled value", "err", err)
	}
}

func (p *Pool[T]) closeAll(victims []T) {
	for _, v := range victims {
		p.closeValue(v)
	}
}

// * idle sweeping. a lazy sweep on access would never run for a target that was
//   removed from service discovery, so its instance - and its live socket -
//   would survive until process exit

func (p *Pool[T]) sweepLoop() {
	defer p.wg.Done()

	for {
		p.mu.Lock()
		interval := p.opts.TTL / 4
		p.mu.Unlock()

		interval = min(max(interval, minSweepInterval), maxSweepInterval)

		timer := time.NewTimer(interval)
		select {
		case <-p.stop:
			timer.Stop()
			return
		case <-timer.C:
		}

		p.sweepOnce()
	}
}

func (p *Pool[T]) sweepOnce() {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return
	}

	now, ttl := p.now(), p.opts.TTL
	var victims []T

	for el := p.lru.Back(); el != nil; {
		prev := el.Prev()
		e := el.Value.(*entry[T])

		if e.refs == 0 && now.Sub(e.lastUsed) > ttl {
			p.detachLocked(e)
			p.evictionsTTL++

			if v, ok := p.takeLocked(e); ok {
				victims = append(victims, v)
			}
		}

		el = prev
	}
	p.mu.Unlock()

	p.closeAll(victims)
}

// * lease

// Lease is a borrowed reference to a pooled value. Release it exactly once
type Lease[T io.Closer] struct {
	p      *Pool[T]
	e      *entry[T]
	cached bool
	once   sync.Once
}

// Value returns the leased value
func (l *Lease[T]) Value() T { return l.e.val }

// Cached reports whether the value was reused rather than built for this lease
func (l *Lease[T]) Cached() bool { return l.cached }

// Release returns the lease to the pool. It is safe to call more than once
func (l *Lease[T]) Release() {
	l.once.Do(func() { l.p.release(l.e) })
}

func closedChan() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
