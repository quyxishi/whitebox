package xraypool

import "github.com/prometheus/client_golang/prometheus"

// Gauges are computed from Stats at scrape time rather than mirrored into
// prometheus objects by a background updater, so they can never go stale
var (
	descInstances = prometheus.NewDesc(
		"whitebox_xray_instances",
		"Number of xray instances currently held by the cache",
		nil, nil,
	)
	descInstancesInUse = prometheus.NewDesc(
		"whitebox_xray_instances_in_use",
		"Number of cached xray instances currently leased by an in-flight probe",
		nil, nil,
	)
	descHits = prometheus.NewDesc(
		"whitebox_xray_cache_hits_total",
		"Total probes that reused an already-built xray instance",
		nil, nil,
	)
	descMisses = prometheus.NewDesc(
		"whitebox_xray_cache_misses_total",
		"Total probes that had to build an xray instance",
		nil, nil,
	)
	descEvictions = prometheus.NewDesc(
		"whitebox_xray_cache_evictions_total",
		"Total cached xray instances evicted, by reason",
		[]string{"reason"}, nil,
	)
	descOverflows = prometheus.NewDesc(
		"whitebox_xray_cache_overflow_total",
		"Total times max_entries was exceeded because every cached instance was in use",
		nil, nil,
	)
	descBuildFailures = prometheus.NewDesc(
		"whitebox_xray_instance_build_failures_total",
		"Total failures to construct or start an xray instance",
		nil, nil,
	)
	descCloseErrors = prometheus.NewDesc(
		"whitebox_xray_instance_close_errors_total",
		"Total errors returned while closing an xray instance",
		nil, nil,
	)
)

// Describe implements prometheus.Collector
func (p *Pool[T]) Describe(ch chan<- *prometheus.Desc) {
	ch <- descInstances
	ch <- descInstancesInUse
	ch <- descHits
	ch <- descMisses
	ch <- descEvictions
	ch <- descOverflows
	ch <- descBuildFailures
	ch <- descCloseErrors

	p.buildDuration.Describe(ch)
}

// Collect implements prometheus.Collector
func (p *Pool[T]) Collect(ch chan<- prometheus.Metric) {
	s := p.Stats()

	gauge := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v)
	}
	counter := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v, labels...)
	}

	gauge(descInstances, float64(s.Size))
	gauge(descInstancesInUse, float64(s.InUse))

	counter(descHits, float64(s.Hits))
	counter(descMisses, float64(s.Misses))
	counter(descEvictions, float64(s.EvictionsTTL), "ttl")
	counter(descEvictions, float64(s.EvictionsLRU), "lru")
	counter(descEvictions, float64(s.EvictionsReload), "reload")
	counter(descEvictions, float64(s.EvictionsShutdown), "shutdown")
	counter(descOverflows, float64(s.Overflows))
	counter(descBuildFailures, float64(s.BuildFailures))
	counter(descCloseErrors, float64(s.CloseErrors))

	p.buildDuration.Collect(ch)
}
