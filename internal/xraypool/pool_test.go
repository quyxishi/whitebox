package xraypool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeInstance struct {
	id     int
	closes atomic.Int32
}

func (f *fakeInstance) Close() error {
	f.closes.Add(1)
	return nil
}

type builder struct {
	mu     sync.Mutex
	builds int
	delay  time.Duration
	err    error
	made   []*fakeInstance
}

func (b *builder) build(context.Context) (*fakeInstance, error) {
	b.mu.Lock()
	b.builds++
	id := b.builds
	delay, err := b.delay, b.err
	b.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	if err != nil {
		return nil, err
	}

	f := &fakeInstance{id: id}

	b.mu.Lock()
	b.made = append(b.made, f)
	b.mu.Unlock()

	return f, nil
}

func (b *builder) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.builds
}

// closedCount totals Close calls across every instance this builder produced
func (b *builder) closedCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	total := 0
	for _, f := range b.made {
		total += int(f.closes.Load())
	}
	return total
}

// clock is a manually advanced time source
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: time.Unix(1700000000, 0)} }

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newTestPool(t *testing.T, opts Options) (*Pool[*fakeInstance], *clock) {
	t.Helper()

	clk := newClock()
	p := New[*fakeInstance](opts)
	p.now = clk.now

	t.Cleanup(func() { _ = p.Close() })

	return p, clk
}

func acquire(t *testing.T, p *Pool[*fakeInstance], key string, b *builder) *Lease[*fakeInstance] {
	t.Helper()

	lease, err := p.Acquire(context.Background(), key, b.build)
	if err != nil {
		t.Fatalf("Acquire(%q): %v", key, err)
	}
	return lease
}

// TestSingleflight asserts that concurrent acquires of one key share a build
func TestSingleflight(t *testing.T) {
	p, _ := newTestPool(t, Options{Enabled: true})
	b := &builder{delay: 50 * time.Millisecond}

	const n = 100

	var wg sync.WaitGroup
	values := make([]*fakeInstance, n)

	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()

			lease := acquire(t, p, "k", b)
			values[i] = lease.Value()
			lease.Release()
		}()
	}
	wg.Wait()

	if got := b.count(); got != 1 {
		t.Errorf("builds = %d, want 1", got)
	}
	if got := b.closedCount(); got != 0 {
		t.Errorf("closes = %d, want 0", got)
	}
	for i, v := range values {
		if v != values[0] {
			t.Fatalf("lease %d returned a different value", i)
		}
	}
}

// TestReuse is the regression test for the leak itself: many probes of one
// config must construct exactly one instance
func TestReuse(t *testing.T) {
	p, _ := newTestPool(t, Options{Enabled: true})
	b := &builder{}

	const n = 10_000

	for range n {
		acquire(t, p, "k", b).Release()
	}

	if got := b.count(); got != 1 {
		t.Errorf("builds = %d, want 1", got)
	}

	s := p.Stats()
	if s.Size != 1 {
		t.Errorf("Size = %d, want 1", s.Size)
	}
	if s.Hits != n-1 {
		t.Errorf("Hits = %d, want %d", s.Hits, n-1)
	}
	if s.Misses != 1 {
		t.Errorf("Misses = %d, want 1", s.Misses)
	}
}

// TestNoCloseWhileInUse asserts that eviction picks a free entry over a leased
// one, and closes the leased one only once it is released
func TestNoCloseWhileInUse(t *testing.T) {
	p, _ := newTestPool(t, Options{Enabled: true, MaxEntries: 1})
	b := &builder{}

	held := acquire(t, p, "held", b)

	// over the cap, but both entries are in use: the cap must be exceeded
	// rather than a live instance torn out from under a probe
	free := acquire(t, p, "free", b)

	if got := b.closedCount(); got != 0 {
		t.Fatalf("closes = %d while both entries are leased, want 0", got)
	}
	free.Release()

	// now there is a legal victim, so the next insert must pick it
	acquire(t, p, "third", b).Release()

	if got := free.Value().closes.Load(); got != 1 {
		t.Errorf("free instance closes = %d, want 1", got)
	}
	if got := held.Value().closes.Load(); got != 0 {
		t.Errorf("held instance closes = %d, want 0", got)
	}

	held.Release()
}

// TestDoubleReleaseClosesOnce asserts a stray second Release cannot double-close
// or corrupt the refcount
func TestDoubleReleaseClosesOnce(t *testing.T) {
	p := New[*fakeInstance](Options{Enabled: true})
	b := &builder{}

	lease, err := p.Acquire(context.Background(), "k", b.build)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// detaches the entry while it is still leased, so Release owns the close
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	lease.Release()
	lease.Release()

	if got := lease.Value().closes.Load(); got != 1 {
		t.Errorf("closes after double release = %d, want 1", got)
	}
}

// TestOverflowRatherThanCloseInUse asserts the cap is exceeded, not violated,
// when every entry is leased
func TestOverflowRatherThanCloseInUse(t *testing.T) {
	p, _ := newTestPool(t, Options{Enabled: true, MaxEntries: 1})
	b := &builder{}

	a := acquire(t, p, "a", b)
	c := acquire(t, p, "b", b)

	if got := b.closedCount(); got != 0 {
		t.Fatalf("closes = %d, want 0 while both leases are held", got)
	}
	if s := p.Stats(); s.Overflows == 0 {
		t.Error("Overflows = 0, want > 0")
	}

	a.Release()
	c.Release()
}

// TestIdleTTL asserts the sweeper closes unreferenced, idle entries
func TestIdleTTL(t *testing.T) {
	p, clk := newTestPool(t, Options{Enabled: true, TTL: time.Minute})
	b := &builder{}

	lease := acquire(t, p, "k", b)
	lease.Release()

	clk.advance(30 * time.Second)
	p.sweepOnce()

	if got := b.closedCount(); got != 0 {
		t.Fatalf("closes = %d before TTL, want 0", got)
	}

	clk.advance(2 * time.Minute)
	p.sweepOnce()

	if got := b.closedCount(); got != 1 {
		t.Errorf("closes = %d after TTL, want 1", got)
	}
	if s := p.Stats(); s.Size != 0 {
		t.Errorf("Size = %d after TTL sweep, want 0", s.Size)
	}
}

// TestIdleTTLSkipsInUse asserts the sweeper leaves leased entries alone
func TestIdleTTLSkipsInUse(t *testing.T) {
	p, clk := newTestPool(t, Options{Enabled: true, TTL: time.Minute})
	b := &builder{}

	lease := acquire(t, p, "k", b)

	clk.advance(time.Hour)
	p.sweepOnce()

	if got := lease.Value().closes.Load(); got != 0 {
		t.Errorf("leased instance closed %d time(s) by the sweeper", got)
	}

	lease.Release()
}

// TestBuildFailureNotCached asserts a transient failure does not poison the key
func TestBuildFailureNotCached(t *testing.T) {
	p, _ := newTestPool(t, Options{Enabled: true})

	sentinel := errors.New("boom")
	b := &builder{err: sentinel}

	if _, err := p.Acquire(context.Background(), "k", b.build); !errors.Is(err, sentinel) {
		t.Fatalf("Acquire error = %v, want %v", err, sentinel)
	}
	if s := p.Stats(); s.Size != 0 {
		t.Errorf("Size = %d after failed build, want 0", s.Size)
	}
	if s := p.Stats(); s.BuildFailures != 1 {
		t.Errorf("BuildFailures = %d, want 1", s.BuildFailures)
	}

	b.mu.Lock()
	b.err = nil
	b.mu.Unlock()

	acquire(t, p, "k", b).Release()

	if got := b.count(); got != 2 {
		t.Errorf("builds = %d, want 2 (the failure must be retried)", got)
	}
}

// TestDisabled asserts the pass-through path builds and closes per acquire
func TestDisabled(t *testing.T) {
	p, _ := newTestPool(t, Options{Enabled: false})
	b := &builder{}

	const n = 5
	for range n {
		acquire(t, p, "k", b).Release()
	}

	if got := b.count(); got != n {
		t.Errorf("builds = %d, want %d", got, n)
	}
	if got := b.closedCount(); got != n {
		t.Errorf("closes = %d, want %d", got, n)
	}
	if s := p.Stats(); s.Size != 0 {
		t.Errorf("Size = %d, want 0", s.Size)
	}
}

// TestSetOptionsDisableDetachesAll asserts a SIGHUP that turns the cache off
// drains it without cutting off in-flight probes
func TestSetOptionsDisableDetachesAll(t *testing.T) {
	p, _ := newTestPool(t, Options{Enabled: true})
	b := &builder{}

	idle := acquire(t, p, "idle", b)
	idle.Release()

	held := acquire(t, p, "held", b)

	p.SetOptions(Options{Enabled: false})

	if got := idle.Value().closes.Load(); got != 1 {
		t.Errorf("idle instance closes = %d, want 1", got)
	}
	if got := held.Value().closes.Load(); got != 0 {
		t.Errorf("held instance closes = %d, want 0", got)
	}
	if s := p.Stats(); s.Size != 0 {
		t.Errorf("Size = %d, want 0", s.Size)
	}

	held.Release()
	if got := held.Value().closes.Load(); got != 1 {
		t.Errorf("held instance closes after release = %d, want 1", got)
	}
}

// TestCloseWithInflight asserts shutdown closes everything exactly once, even
// values still leased out
func TestCloseWithInflight(t *testing.T) {
	p := New[*fakeInstance](Options{Enabled: true})
	b := &builder{}

	lease, err := p.Acquire(context.Background(), "k", b.build)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := lease.Value().closes.Load(); got != 0 {
		t.Errorf("closes = %d while still leased, want 0", got)
	}

	lease.Release()
	if got := lease.Value().closes.Load(); got != 1 {
		t.Errorf("closes after release = %d, want 1", got)
	}

	if _, err := p.Acquire(context.Background(), "k", b.build); !errors.Is(err, ErrClosed) {
		t.Errorf("Acquire after Close = %v, want ErrClosed", err)
	}
}

// TestCloseStopsSweeper asserts Close does not leak its background goroutine
func TestCloseStopsSweeper(t *testing.T) {
	p := New[*fakeInstance](Options{Enabled: true, TTL: time.Second})

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Close waits on the sweeper's WaitGroup, so a second Close proves the
	// goroutine is gone rather than merely signalled
	if err := p.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestConcurrentDistinctKeys exercises the map, LRU and refcount under -race.
//
// While the workload runs, Size may legitimately exceed MaxEntries: more keys
// are leased concurrently than the cap allows, and in-use entries are never
// evicted. The invariant is that the cap is enforced again as soon as entries
// are free, which happens on the next insert
func TestConcurrentDistinctKeys(t *testing.T) {
	const maxEntries = 8

	p, _ := newTestPool(t, Options{Enabled: true, MaxEntries: maxEntries})
	b := &builder{}

	keys := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				acquire(t, p, keys[i%len(keys)], b).Release()
			}
		}()
	}
	wg.Wait()

	// everything is released now, so this insert can trim back to the cap
	acquire(t, p, "trigger", b).Release()

	if s := p.Stats(); s.Size > maxEntries {
		t.Errorf("Size = %d once quiesced, want <= %d", s.Size, maxEntries)
	}
	if got, want := b.closedCount(), b.count()-p.Stats().Size; got != want {
		t.Errorf("closes = %d, want %d (every evicted instance must be closed)", got, want)
	}
}

// TestAcquireUncached asserts the opt-out path builds per acquire and closes on
// release even with the cache on, and never lets the value into the cache
func TestAcquireUncached(t *testing.T) {
	p, _ := newTestPool(t, Options{Enabled: true})
	b := &builder{}

	const n = 5
	for range n {
		lease, err := p.AcquireUncached(context.Background(), b.build)
		if err != nil {
			t.Fatalf("AcquireUncached: %v", err)
		}
		lease.Release()
	}

	if got := b.count(); got != n {
		t.Errorf("builds = %d, want %d", got, n)
	}
	if got := b.closedCount(); got != n {
		t.Errorf("closes = %d, want %d", got, n)
	}

	s := p.Stats()
	if s.Size != 0 {
		t.Errorf("Size = %d, want 0", s.Size)
	}
	if s.Hits != 0 {
		t.Errorf("Hits = %d, want 0", s.Hits)
	}
	if s.Misses != n {
		t.Errorf("Misses = %d, want %d", s.Misses, n)
	}

	// the pooled path is untouched by it
	acquire(t, p, "k", b).Release()
	acquire(t, p, "k", b).Release()

	if got := b.count(); got != n+1 {
		t.Errorf("builds after two pooled acquires = %d, want %d", got, n+1)
	}
}

// TestAcquireUncachedAfterClose asserts the opt-out path refuses to build once
// the pool is shut down, like Acquire does
func TestAcquireUncachedAfterClose(t *testing.T) {
	p := New[*fakeInstance](Options{Enabled: true})
	b := &builder{}

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := p.AcquireUncached(context.Background(), b.build); !errors.Is(err, ErrClosed) {
		t.Errorf("AcquireUncached after Close = %v, want ErrClosed", err)
	}
	if got := b.count(); got != 0 {
		t.Errorf("builds = %d, want 0", got)
	}
}
