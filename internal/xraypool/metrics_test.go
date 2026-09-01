package xraypool

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// gather registers the pool on a fresh registry and collects it
func gather(t *testing.T, p *Pool[*fakeInstance]) map[string]*dto.MetricFamily {
	t.Helper()

	reg := prometheus.NewRegistry()
	if err := reg.Register(p); err != nil {
		t.Fatalf("register pool collector: %v", err)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	out := make(map[string]*dto.MetricFamily, len(families))
	for _, f := range families {
		out[f.GetName()] = f
	}
	return out
}

// TestCollectorExposesCacheSeries guards the Describe/Collect pair: a mismatch
// between them surfaces as a scrape-time error rather than at build time
func TestCollectorExposesCacheSeries(t *testing.T) {
	p, _ := newTestPool(t, Options{Enabled: true})
	b := &builder{}

	acquire(t, p, "k", b).Release()
	acquire(t, p, "k", b).Release()

	families := gather(t, p)

	for _, name := range []string{
		"whitebox_xray_instances",
		"whitebox_xray_instances_in_use",
		"whitebox_xray_cache_hits_total",
		"whitebox_xray_cache_misses_total",
		"whitebox_xray_cache_evictions_total",
		"whitebox_xray_cache_overflow_total",
		"whitebox_xray_instance_build_failures_total",
		"whitebox_xray_instance_close_errors_total",
		"whitebox_xray_instance_build_duration_seconds",
	} {
		if _, ok := families[name]; !ok {
			t.Errorf("metric %q is not exposed", name)
		}
	}

	gaugeValue := func(name string) float64 {
		t.Helper()

		f, ok := families[name]
		if !ok || len(f.GetMetric()) != 1 {
			t.Fatalf("metric %q is missing or not a single series", name)
		}
		return f.GetMetric()[0].GetGauge().GetValue()
	}

	counterValue := func(name string) float64 {
		t.Helper()

		f, ok := families[name]
		if !ok || len(f.GetMetric()) != 1 {
			t.Fatalf("metric %q is missing or not a single series", name)
		}
		return f.GetMetric()[0].GetCounter().GetValue()
	}

	if got := gaugeValue("whitebox_xray_instances"); got != 1 {
		t.Errorf("whitebox_xray_instances = %v, want 1", got)
	}
	if got := gaugeValue("whitebox_xray_instances_in_use"); got != 0 {
		t.Errorf("whitebox_xray_instances_in_use = %v, want 0", got)
	}
	if got := counterValue("whitebox_xray_cache_hits_total"); got != 1 {
		t.Errorf("whitebox_xray_cache_hits_total = %v, want 1", got)
	}
	if got := counterValue("whitebox_xray_cache_misses_total"); got != 1 {
		t.Errorf("whitebox_xray_cache_misses_total = %v, want 1", got)
	}

	// evictions carry a reason label, so all four series must be present for a
	// dashboard to sum over them
	evictions, ok := families["whitebox_xray_cache_evictions_total"]
	if !ok {
		t.Fatal("whitebox_xray_cache_evictions_total is not exposed")
	}

	reasons := make(map[string]bool)
	for _, m := range evictions.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == "reason" {
				reasons[l.GetValue()] = true
			}
		}
	}
	for _, want := range []string{"ttl", "lru", "reload", "shutdown"} {
		if !reasons[want] {
			t.Errorf("evictions are missing the %q reason", want)
		}
	}
}
