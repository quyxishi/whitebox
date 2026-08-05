package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	neturl "net/url"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// `nettraceDial` mimics xray's tcp path: the `httptrace` context reaches a
// `net.Dialer`, so DNSStart/ConnectStart/ConnectDone fire
func nettraceDial(ctx context.Context, network, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, addr)
}

// `opaqueDial` mimics xray's udp/quic path (hysteria2, wireguard): `net.Dial` drops
// the caller's context internally, so no nettrace event is ever emitted
func opaqueDial(_ context.Context, network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

// `tracedClient` assembles the same stack as `probe.Probe`: a custom `DialContext`, the
// `RoundTransport` wrapper and the `httptrace` hooks bound to it
func tracedClient(dial func(context.Context, string, string) (net.Conn, error), maxRedirects int) (*http.Client, *RoundTransport) {
	redirects := RedirectCounter{Max: maxRedirects}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext:     dial,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: redirects.CheckRedirect,
	}

	tt := &RoundTransport{Transport: client.Transport}
	client.Transport = tt

	return client, tt
}

func tracedGet(t *testing.T, client *http.Client, tt *RoundTransport, url string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to construct request: %v", err)
	}

	trace := &httptrace.ClientTrace{
		GetConn:              tt.GetConn,
		DNSStart:             tt.DNSStart,
		DNSDone:              tt.DNSDone,
		TLSHandshakeStart:    tt.TLSHandshakeStart,
		TLSHandshakeDone:     tt.TLSHandshakeDone,
		ConnectStart:         tt.ConnectStart,
		ConnectDone:          tt.ConnectDone,
		GotConn:              tt.GotConn,
		GotFirstResponseByte: tt.GotFirstResponseByte,
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("failed to close body: %v", err)
		}
	}()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("failed to drain body: %v", err)
	}

	tt.Finish()
}

func requireNonZero(t *testing.T, name string, ts time.Time) {
	t.Helper()

	if ts.IsZero() {
		t.Errorf("%s: expected a recorded timestamp, got zero time.Time", name)
	}
}

// `requireSane` catches the saturating Sub
func requireSane(t *testing.T, name string, d time.Duration) {
	t.Helper()

	if d < 0 || d > time.Second {
		t.Errorf("%s: implausible duration %v (%.6fs)", name, d, d.Seconds())
	}
}

// `requireOrdered` asserts non-strict ordering: on Windows the clock quantises to
// roughly half a millisecond, so adjacent phases legitimately share a timestamp
func requireOrdered(t *testing.T, earlierName string, earlier time.Time, laterName string, later time.Time) {
	t.Helper()

	if earlier.After(later) {
		t.Errorf("%s (%v) should not be after %s (%v)", earlierName, earlier, laterName, later)
	}
}

// `TestRoundTrace_OpaqueDial_HTTP` is the regression test for the hysteria2 bug:
// with no nettrace events at all, every boundary must still be backfilled
func TestRoundTrace_OpaqueDial_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client, tt := tracedClient(opaqueDial, 0)
	tracedGet(t, client, tt, srv.URL)

	if len(tt.Traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(tt.Traces))
	}
	trace := tt.Traces[0]

	requireNonZero(t, "Init", trace.Init)
	requireNonZero(t, "DnsExit", trace.DnsExit)
	requireNonZero(t, "ConnectExit", trace.ConnectExit)
	requireNonZero(t, "GotConnect", trace.GotConnect)
	requireNonZero(t, "GotFirstByte", trace.GotFirstByte)
	requireNonZero(t, "Exit", trace.Exit)

	if !trace.DnsExit.Equal(trace.Init) {
		t.Errorf("no resolution was observable, so DnsExit should collapse onto Init")
	}
	if !trace.ConnectExit.Equal(trace.GotConnect) {
		t.Errorf("without tls, ConnectExit should fall back to GotConnect")
	}

	requireSane(t, "connect phase", trace.GotConnect.Sub(trace.DnsExit))
}

func TestRoundTrace_OpaqueDial_HTTPS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client, tt := tracedClient(opaqueDial, 0)
	tracedGet(t, client, tt, srv.URL)

	if len(tt.Traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(tt.Traces))
	}
	trace := tt.Traces[0]

	if !trace.Tls {
		t.Fatalf("expected the trace to be flagged as tls")
	}

	requireNonZero(t, "TlsEntry", trace.TlsEntry)
	requireNonZero(t, "TlsExit", trace.TlsExit)

	if !trace.ConnectExit.Equal(trace.TlsEntry) {
		t.Errorf("ConnectExit should fall back to TlsEntry, not GotConnect")
	}

	connect := trace.ConnectExit.Sub(trace.DnsExit)
	handshake := trace.TlsExit.Sub(trace.TlsEntry)

	requireSane(t, "connect phase", connect)
	requireSane(t, "tls phase", handshake)

	// connect must not swallow the handshake
	if connect+handshake > trace.GotConnect.Sub(trace.Init) {
		t.Errorf("connect (%v) + tls (%v) exceeds the acquisition span (%v): phases double count",
			connect, handshake, trace.GotConnect.Sub(trace.Init))
	}
}

// `TestRoundTrace_NettraceDial_HTTPS` guards the vless path: a transport that does
// report dns and connect must still produce a monotonic trace
func TestRoundTrace_NettraceDial_HTTPS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// `httptest` hands back a 127.0.0.1; resolving localhost instead makes the
	// dns hooks fire, which is what a domain target looks like through a tunnel
	url, err := neturl.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse server url: %v", err)
	}
	url.Host = net.JoinHostPort("localhost", url.Port())

	client, tt := tracedClient(nettraceDial, 0)
	tracedGet(t, client, tt, url.String())

	if len(tt.Traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(tt.Traces))
	}
	trace := tt.Traces[0]

	if !trace.dnsSeen {
		t.Errorf("expected DNSStart to fire when resolving a hostname")
	}

	requireNonZero(t, "DnsExit", trace.DnsExit)
	requireNonZero(t, "ConnectExit", trace.ConnectExit)

	requireOrdered(t, "Init", trace.Init, "DnsExit", trace.DnsExit)
	requireOrdered(t, "DnsExit", trace.DnsExit, "ConnectExit", trace.ConnectExit)
	requireOrdered(t, "ConnectExit", trace.ConnectExit, "TlsEntry", trace.TlsEntry)
	requireOrdered(t, "TlsExit", trace.TlsExit, "GotConnect", trace.GotConnect)

	requireSane(t, "resolve phase", trace.DnsExit.Sub(trace.Init))
	requireSane(t, "connect phase", trace.ConnectExit.Sub(trace.DnsExit))
	requireSane(t, "tls phase", trace.TlsExit.Sub(trace.TlsEntry))
}

// `TestRoundTrace_BackfillIsInert` drives the hooks directly, spaced far enough
// apart to beat the platform clock resolution
func TestRoundTrace_BackfillIsInert(t *testing.T) {
	trace := &RoundTrace{Tls: true}
	tt := &RoundTransport{Actual: trace, Traces: []*RoundTrace{trace}}

	const tick = 5 * time.Millisecond

	tt.GetConn("localhost:443")
	time.Sleep(tick)
	tt.DNSStart(httptrace.DNSStartInfo{})
	time.Sleep(tick)
	tt.DNSDone(httptrace.DNSDoneInfo{})
	time.Sleep(tick)
	tt.ConnectStart("tcp", "127.0.0.1:443")
	time.Sleep(tick)
	tt.ConnectDone("tcp", "127.0.0.1:443", nil)
	time.Sleep(tick)
	tt.TLSHandshakeStart()
	time.Sleep(tick)
	tt.TLSHandshakeDone(tls.ConnectionState{}, nil)
	time.Sleep(tick)
	tt.GotConn(httptrace.GotConnInfo{})

	if !trace.DnsExit.Before(trace.ConnectExit) {
		t.Errorf("DnsExit (%v) was overwritten: it should hold the DNSDone timestamp", trace.DnsExit)
	}
	if !trace.ConnectExit.Before(trace.TlsEntry) {
		t.Errorf("ConnectExit (%v) was backfilled from TlsEntry even though ConnectDone fired", trace.ConnectExit)
	}
	if !trace.Init.Before(trace.DnsExit) {
		t.Errorf("Init (%v) should hold the GetConn timestamp", trace.Init)
	}
}

// `TestRoundTrace_ConnectionReuse` covers the redirect case, which produces the
// same zero timestamps on tcp transports because the pooled connection emits no
// dns or connect hooks on the second round-trip
func TestRoundTrace_ConnectionReuse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusFound)
	})
	mux.HandleFunc("/final", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, tt := tracedClient(nettraceDial, 1)
	tracedGet(t, client, tt, srv.URL)

	if len(tt.Traces) != 2 {
		t.Fatalf("expected 2 traces, got %d", len(tt.Traces))
	}

	for i, trace := range tt.Traces {
		requireNonZero(t, "Init", trace.Init)
		requireNonZero(t, "DnsExit", trace.DnsExit)
		requireNonZero(t, "ConnectExit", trace.ConnectExit)
		requireNonZero(t, "GotConnect", trace.GotConnect)

		requireSane(t, "connect phase", trace.GotConnect.Sub(trace.DnsExit))
		requireSane(t, "resolve phase", trace.DnsExit.Sub(trace.Init))

		if i == 1 && !trace.DnsExit.Equal(trace.Init) {
			t.Errorf("the reused connection resolved nothing, so DnsExit should equal Init")
		}
	}
}

func TestRoundTrace_DialFailure(t *testing.T) {
	failingDial := func(_ context.Context, _, _ string) (net.Conn, error) {
		return nil, errors.New("boom")
	}

	client, tt := tracedClient(failingDial, 0)

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:1/", nil)
	if err != nil {
		t.Fatalf("failed to construct request: %v", err)
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
		GetConn: tt.GetConn,
		GotConn: tt.GotConn,
	}))

	if _, err := client.Do(req); err == nil {
		t.Fatalf("expected the request to fail")
	}

	if len(tt.Traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(tt.Traces))
	}
	trace := tt.Traces[0]

	// `GetConn` fires before any dial attempt, so the round-trip start is known
	// even though the connection was never established
	requireNonZero(t, "Init", trace.Init)

	if !trace.GotConnect.IsZero() {
		t.Errorf("GotConnect should stay zero when no connection was acquired")
	}
}

func TestRoundTrace_NoRoundTripYet(t *testing.T) {
	tt := &RoundTransport{}

	tt.GetConn("example.com:443")
	tt.DNSStart(httptrace.DNSStartInfo{})
	tt.DNSDone(httptrace.DNSDoneInfo{})
	tt.ConnectStart("tcp", "127.0.0.1:443")
	tt.ConnectDone("tcp", "127.0.0.1:443", nil)
	tt.TLSHandshakeStart()
	tt.TLSHandshakeDone(tls.ConnectionState{}, nil)
	tt.GotConn(httptrace.GotConnInfo{})
	tt.GotFirstResponseByte()
	tt.Finish()

	if len(tt.Traces) != 0 {
		t.Errorf("expected no traces to be recorded, got %d", len(tt.Traces))
	}
}

// `gaugeValue` reads a gauge without pulling in prometheus/testutil, which would
// add a module dependency for the sake of one assertion
func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()

	metric := &dto.Metric{}
	if err := g.Write(metric); err != nil {
		t.Fatalf("failed to read gauge: %v", err)
	}

	return metric.GetGauge().GetValue()
}

// `TestObserveTraces_ZeroTimestamps` feeds observeTraces the degenerate traces the
// guards exist for; no phase may ever be handed a zero `time.Time`
func TestObserveTraces_ZeroTimestamps(t *testing.T) {
	phases := []string{"resolve", "connect", "tls", "processing", "transfer"}

	now := time.Now()

	tests := []struct {
		name  string
		trace *RoundTrace
	}{
		{
			name:  "entirely unrecorded",
			trace: &RoundTrace{},
		},
		{
			name:  "unrecorded but flagged tls",
			trace: &RoundTrace{Tls: true},
		},
		{
			// the shape hysteria2 produced before the backfill
			name:  "plain http, only GotConn recorded",
			trace: &RoundTrace{GotConnect: now},
		},
		{
			name:  "tls, only GotConn recorded",
			trace: &RoundTrace{Tls: true, GotConnect: now},
		},
		{
			name:  "connected but never answered",
			trace: &RoundTrace{Init: now, DnsExit: now, ConnectExit: now, GotConnect: now},
		},
		{
			name: "answered but never drained",
			trace: &RoundTrace{
				Init: now, DnsExit: now, ConnectExit: now,
				GotConnect: now, GotFirstByte: now.Add(10 * time.Millisecond),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "tun_probe_http_duration_seconds",
				Help: "Duration of HTTP request by phase, summed over all traces",
			}, []string{"phase"})

			for _, phase := range phases {
				vec.WithLabelValues(phase)
			}

			observeTraces(vec, []*RoundTrace{tc.trace})

			for _, phase := range phases {
				got := gaugeValue(t, vec.WithLabelValues(phase))
				if got < 0 || got > 1 {
					t.Errorf("phase %q: implausible value %v", phase, got)
				}
			}
		})
	}
}
