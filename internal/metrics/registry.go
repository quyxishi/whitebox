// Package metrics owns whitebox's process-level prometheus registry.
//
// This is deliberately separate from the per-request registry that /probe
// builds: those metrics describe one probe through one tunnel, while these
// describe the exporter process itself.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is the process-level registry. A dedicated registry is used rather
// than prometheus.DefaultRegisterer, which is global mutable state shared with
// every dependency in the binary
var Registry = prometheus.NewRegistry()

func init() {
	// go_memstats_heap_inuse_bytes and go_goroutines are how a memory
	// regression is caught without having to reach for pprof
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

// Handler serves the process-level registry
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{Registry: Registry})
}
