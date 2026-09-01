package probe

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/quyxishi/whitebox/internal/config"
	"github.com/quyxishi/whitebox/internal/xraypool"

	"github.com/gin-gonic/gin"
	"github.com/xtls/xray-core/core"
)

const e2eClientUUID = "0f1c9c5e-1c2a-4f2b-9a1a-2f0f5b8d7c31"

// freePort returns a port that was free a moment ago. Racy in principle, but
// xray takes an address rather than a listener, so there is no better option
func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port

	if err := l.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return port
}

// startVlessServer brings up an in-process vless inbound with a freedom
// outbound, so a probe traverses a real tunnel
func startVlessServer(t *testing.T, port int) {
	t.Helper()

	// freedom blackholes private ranges by default behind a vless inbound, so
	// reaching the loopback target has to be allowed explicitly. Rules given
	// here are matched ahead of that default
	conf := fmt.Sprintf(`{
		"log": {"loglevel": "none"},
		"inbounds": [{
			"listen": "127.0.0.1",
			"port": %d,
			"protocol": "vless",
			"settings": {"clients": [{"id": %q}], "decryption": "none"},
			"streamSettings": {"network": "tcp", "security": "none"}
		}],
		"outbounds": [{
			"protocol": "freedom",
			"settings": {"finalRules": [{"action": "allow", "ip": ["127.0.0.0/8"]}]}
		}]
	}`, port, e2eClientUUID)

	xrayConf, err := core.LoadConfig("json", strings.NewReader(conf))
	if err != nil {
		t.Fatalf("load server config: %v", err)
	}

	instance, err := core.New(xrayConf)
	if err != nil {
		t.Fatalf("new server instance: %v", err)
	}
	if err := instance.Start(); err != nil {
		t.Fatalf("start server instance: %v", err)
	}

	t.Cleanup(func() {
		if err := instance.Close(); err != nil {
			t.Logf("close server instance: %v", err)
		}
	})
}

type e2eHarness struct {
	router *gin.Engine
	pool   *InstancePool
	query  string
}

func newE2EHarness(t *testing.T) *e2eHarness {
	t.Helper()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	t.Cleanup(target.Close)

	port := freePort(t)
	startVlessServer(t, port)

	pool := NewInstancePool(xraypool.Options{Enabled: true})
	t.Cleanup(func() { _ = pool.Close() })

	cfg := config.NewWhiteboxConfig()
	handler := NewProbeHandler(config.NewConfigWrapper(&cfg), pool)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/probe", handler.Probe)

	uri := fmt.Sprintf("vless://%s@127.0.0.1:%d?type=tcp&encryption=none&security=none#e2e",
		e2eClientUUID, port)

	query := neturl.Values{"ctx": {uri}, "target": {target.URL}}.Encode()

	return &e2eHarness{router: router, pool: pool, query: query}
}

// probe runs one scrape and returns the exported metrics body
func (h *e2eHarness) probe(t *testing.T) string {
	t.Helper()

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe?"+h.query, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("probe status = %d, body: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// TestProbeReusesInstance is the regression test for issue #10.
//
// xray-core's splithttp, grpc and hysteria dialers key package-level caches on
// the *internet.MemoryStreamConfig pointer that core.New allocates, and never
// evict from them. One instance per probe therefore leaked one permanent entry
// per probe, so the invariant that matters is a build count that does not grow
// with the probe count
func TestProbeReusesInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: stands up a real vless tunnel")
	}

	h := newE2EHarness(t)

	const probes = 200

	for i := range probes {
		body := h.probe(t)

		if !strings.Contains(body, "tun_probe_success 1") {
			t.Fatalf("probe %d did not succeed, body:\n%s", i, body)
		}

		// the first probe builds, every later one must reuse
		want := "tun_probe_instance_cached 1"
		if i == 0 {
			want = "tun_probe_instance_cached 0"
		}
		if !strings.Contains(body, want) {
			t.Fatalf("probe %d: want %q, body:\n%s", i, want, body)
		}
	}

	s := h.pool.Stats()

	if s.Misses != 1 {
		t.Errorf("Misses = %d after %d probes, want 1", s.Misses, probes)
	}
	if s.Hits != probes-1 {
		t.Errorf("Hits = %d, want %d", s.Hits, probes-1)
	}
	if s.Size != 1 {
		t.Errorf("Size = %d, want 1", s.Size)
	}
	if s.InUse != 0 {
		t.Errorf("InUse = %d once every probe returned, want 0", s.InUse)
	}
	if s.BuildFailures != 0 {
		t.Errorf("BuildFailures = %d, want 0", s.BuildFailures)
	}
}

// TestProbeGoroutinesDoNotScale asserts nothing per-probe outlives the probe.
//
// A ratio between two probe counts is used rather than an absolute threshold:
// runtime and gin keep a variable number of goroutines around, but a leak shows
// up as a delta that tracks the probe count
func TestProbeGoroutinesDoNotScale(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: stands up a real vless tunnel")
	}

	h := newE2EHarness(t)

	// tunnel teardown is asynchronous - closing the probe's idle connection
	// unwinds through the xray link, the vless inbound and the target server -
	// so sampling on a fixed delay measures connections still draining rather
	// than connections that leaked. wait for the count to stop moving instead
	settle := func() int {
		const stableFor = 5

		prev, stable := -1, 0
		deadline := time.Now().Add(30 * time.Second)

		for time.Now().Before(deadline) {
			runtime.GC()
			time.Sleep(200 * time.Millisecond)

			n := runtime.NumGoroutine()
			if n != prev {
				prev, stable = n, 0
				continue
			}
			if stable++; stable >= stableFor {
				return n
			}
		}

		t.Fatalf("goroutine count never settled, still at %d", runtime.NumGoroutine())
		return 0
	}

	// warm up first, so instance construction is not counted as growth
	h.probe(t)
	base := settle()

	for range 50 {
		h.probe(t)
	}
	small := settle() - base

	for range 250 {
		h.probe(t)
	}
	large := settle() - base

	t.Logf("goroutine delta: +50 probes = %d, +300 probes = %d", small, large)

	// six times the probes must not mean six times the goroutines
	if large > small+20 {
		t.Errorf("goroutines scale with probe count: %d after 50 probes, %d after 300", small, large)
	}
}
