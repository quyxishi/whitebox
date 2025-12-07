package probe

import (
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"
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
}

type RoundTransport struct {
	Transport http.RoundTripper

	mu     sync.Mutex
	Actual *RoundTrace
	Traces []*RoundTrace
}

func (h *RoundTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	trace := &RoundTrace{Tls: req.URL.Scheme == "https"}
	h.Actual = trace
	h.Traces = append(h.Traces, trace)

	return h.Transport.RoundTrip(req)
}

func (h *RoundTransport) TunEntry() {

}

func (h *RoundTransport) DNSStart(_ httptrace.DNSStartInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Actual.Init = time.Now()
}

func (h *RoundTransport) DNSDone(_ httptrace.DNSDoneInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Actual.DnsExit = time.Now()
}

func (h *RoundTransport) TLSHandshakeStart() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Actual.TlsEntry = time.Now()
}

func (h *RoundTransport) TLSHandshakeDone(_ tls.ConnectionState, _ error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Actual.TlsExit = time.Now()
}

func (h *RoundTransport) ConnectStart(_, _ string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// no dns resolution because we connected to ip directly
	if h.Actual.DnsExit.IsZero() {
		h.Actual.Init = time.Now()
		h.Actual.DnsExit = h.Actual.Init
	}
}

func (h *RoundTransport) ConnectDone(_, _ string, _ error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Actual.ConnectExit = time.Now()
}

func (h *RoundTransport) GotConn(_ httptrace.GotConnInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Actual.GotConnect = time.Now()
}

func (h *RoundTransport) GotFirstResponseByte() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Actual.GotFirstByte = time.Now()
}
