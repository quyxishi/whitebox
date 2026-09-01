package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quyxishi/whitebox/internal/api/v1/probe"
	"github.com/quyxishi/whitebox/internal/config"
	"github.com/quyxishi/whitebox/internal/metrics"
	"github.com/quyxishi/whitebox/internal/xraypool"

	"github.com/gin-gonic/gin"
)

// TestMetricsEndpoint asserts the process-level registry is actually wired to a
// route, and that the instance cache series reach it.
//
// These are the series used to confirm memory stays flat in production, so a
// silent regression in the wiring would be expensive to discover later
func TestMetricsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pool := probe.NewInstancePool(xraypool.Options{Enabled: true})
	t.Cleanup(func() { _ = pool.Close() })

	if err := metrics.Registry.Register(pool); err != nil {
		t.Fatalf("register pool: %v", err)
	}
	t.Cleanup(func() { metrics.Registry.Unregister(pool) })

	cfg := config.NewWhiteboxConfig()
	server := NewServer(config.NewConfigWrapper(&cfg), ":9116", pool)

	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, body: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()

	for _, want := range []string{
		// the cache's own series
		"whitebox_xray_instances",
		"whitebox_xray_cache_hits_total",
		"whitebox_xray_cache_misses_total",
		// runtime series, which are how a memory regression is caught
		"go_memstats_heap_inuse_bytes",
		"go_goroutines",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics is missing %q", want)
		}
	}
}
