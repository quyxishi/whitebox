package probe

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type RoundTrace struct {
	Init         time.Time
	DnsExit      time.Time
	ConnectExit  time.Time
	GotConnect   time.Time
	GotFirstByte time.Time
	TlsEntry     time.Time
	TlsExit      time.Time
	Exit         time.Time

	Tls bool

	// `dnsSeen` records whether DNSStart actually fired for this round-trip
	dnsSeen bool
}

type RoundTransport struct {
	Transport http.RoundTripper

	mu     sync.Mutex
	Actual *RoundTrace
	Traces []*RoundTrace
}

func (h *RoundTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	trace := &RoundTrace{Tls: req.URL.Scheme == "https"}

	h.mu.Lock()
	h.Actual = trace
	h.Traces = append(h.Traces, trace)
	h.mu.Unlock()

	return h.Transport.RoundTrip(req)
}

// touch applies fn to the in-flight trace under `h.mu`
func (h *RoundTransport) touch(fn func(t *RoundTrace, now time.Time)) {
	// sample before the lock so contention cannot skew a timestamp
	now := time.Now()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.Actual == nil {
		return
	}

	fn(h.Actual, now)
}

// Finish stamps the end of the in-flight trace once its body has been consumed
func (h *RoundTransport) Finish() {
	h.touch(func(t *RoundTrace, now time.Time) { t.Exit = now })
}

func (h *RoundTransport) TunEntry() {

}

// `GetConn` is fired by `net/http` itself before the idle-pool lookup, so it is the
// earliest marker that exists regardless of what `DialContext` does. it is guarded
// because `net/http` retries `getConn` when it picks up a stale idle connection
func (h *RoundTransport) GetConn(_ string) {
	h.touch(func(t *RoundTrace, now time.Time) {
		if t.Init.IsZero() {
			t.Init = now
		}
	})
}

func (h *RoundTransport) DNSStart(_ httptrace.DNSStartInfo) {
	h.touch(func(t *RoundTrace, now time.Time) {
		t.dnsSeen = true

		if t.Init.IsZero() {
			t.Init = now
		}
	})
}

func (h *RoundTransport) DNSDone(_ httptrace.DNSDoneInfo) {
	h.touch(func(t *RoundTrace, now time.Time) { t.DnsExit = now })
}

func (h *RoundTransport) TLSHandshakeStart() {
	h.touch(func(t *RoundTrace, now time.Time) { t.TlsEntry = now })
}

func (h *RoundTransport) TLSHandshakeDone(_ tls.ConnectionState, _ error) {
	h.touch(func(t *RoundTrace, now time.Time) { t.TlsExit = now })
}

func (h *RoundTransport) ConnectStart(_, _ string) {
	h.touch(func(t *RoundTrace, now time.Time) {
		// no dns resolution because we connected to ip directly
		if !t.dnsSeen && t.DnsExit.IsZero() {
			t.DnsExit = now
		}
	})
}

func (h *RoundTransport) ConnectDone(_, _ string, _ error) {
	h.touch(func(t *RoundTrace, now time.Time) { t.ConnectExit = now })
}

// `GotConn` is the last hook of connection acquisition
func (h *RoundTransport) GotConn(_ httptrace.GotConnInfo) {
	h.touch(func(t *RoundTrace, now time.Time) {
		t.GotConnect = now

		// dns and connect hooks are nettrace-driven and only fire when the dial
		// chain reaches a `net.Dialer`; udp/quic transports (hysteria2, wireguard)
		// and pooled connection reuse emit none of them
		if t.Init.IsZero() {
			t.Init = now
		}
		if t.DnsExit.IsZero() {
			// no observable resolution step: resolve phase collapses to zero
			t.DnsExit = t.Init
		}
		if t.ConnectExit.IsZero() {
			if !t.TlsEntry.IsZero() {
				// prefer the handshake start so connect does not swallow tls
				t.ConnectExit = t.TlsEntry
			} else {
				t.ConnectExit = now
			}
		}
	})
}

func (h *RoundTransport) GotFirstResponseByte() {
	h.touch(func(t *RoundTrace, now time.Time) { t.GotFirstByte = now })
}

// `observeTraces` sums every observable phase across all traces onto vec
func observeTraces(vec *prometheus.GaugeVec, traces []*RoundTrace) {
	slog.Info(fmt.Sprintf("a total of %d trace(s) were encountered during probing", len(traces)))

	for i, trace := range traces {
		slog.Debug(
			"trace:",
			"roundtrip", i,
			"init", traceMilli(trace.Init),
			"dnsExit", traceMilli(trace.DnsExit),
			"connectExit", traceMilli(trace.ConnectExit),
			"gotConnect", traceMilli(trace.GotConnect),
			"gotFirstByte", traceMilli(trace.GotFirstByte),
			"tlsEntry", traceMilli(trace.TlsEntry),
			"tlsExit", traceMilli(trace.TlsExit),
			"exit", traceMilli(trace.Exit),
		)

		if !trace.Init.IsZero() && !trace.DnsExit.IsZero() {
			vec.WithLabelValues("resolve").Add(trace.DnsExit.Sub(trace.Init).Seconds())
		}

		// continue here if we never got a connection because a request failed
		if trace.GotConnect.IsZero() {
			continue
		}

		if trace.Tls {
			if !trace.ConnectExit.IsZero() && !trace.DnsExit.IsZero() {
				vec.WithLabelValues("connect").Add(trace.ConnectExit.Sub(trace.DnsExit).Seconds())
			}
			if !trace.TlsExit.IsZero() && !trace.TlsEntry.IsZero() {
				vec.WithLabelValues("tls").Add(trace.TlsExit.Sub(trace.TlsEntry).Seconds())
			}
		} else if !trace.DnsExit.IsZero() {
			vec.WithLabelValues("connect").Add(trace.GotConnect.Sub(trace.DnsExit).Seconds())
		}

		// continue here if we never got a response from the server
		if trace.GotFirstByte.IsZero() {
			continue
		}

		vec.WithLabelValues("processing").Add(trace.GotFirstByte.Sub(trace.GotConnect).Seconds())

		// continue here if we never read the full response from the server
		// usually this means that request either failed or was redirected
		if trace.Exit.IsZero() {
			continue
		}

		vec.WithLabelValues("transfer").Add(trace.Exit.Sub(trace.GotFirstByte).Seconds())
	}
}

// `traceMilli` keeps unrecorded timestamps readable in trace logs
func traceMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}

	return t.UnixMilli()
}
