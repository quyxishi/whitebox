package serial_test

import (
	"strings"
	"testing"

	"github.com/quyxishi/whitebox/internal/serial"
)

func parseURI(t *testing.T, uri string) string {
	t.Helper()

	out, err := serial.ParseURI(serial.CONFIG_BACKEND_XRAYCORE, uri, &serial.ParseParams{})
	if err != nil {
		t.Fatalf("ParseURI: %v", err)
	}
	return out
}

// TestXmuxRetiredPerRequest asserts xhttp configs retire their transport
// connection after one request.
//
// Instances are now reused across probes, so without this an xmux client would
// be kept for xray's default 600-900 requests and a probe would stop exercising
// the handshake to the vpn server
func TestXmuxRetiredPerRequest(t *testing.T) {
	for name, uri := range map[string]string{
		"vless-reality":  URI_VLESS_XHTTP_REALITY,
		"trojan-reality": URI_TROJAN_XHTTP_REALITY,
		"vmess-tls":      URI_VMESS_XHTTP_TLS,
		"shadowsocks":    URI_SHADOWSOCKS_XHTTP_TLS,
	} {
		t.Run(name, func(t *testing.T) {
			if out := parseURI(t, uri); !strings.Contains(out, `"hMaxRequestTimes":1`) {
				t.Errorf("generated config is missing the injected xmux:\n%s", out)
			}
		})
	}
}

// TestXmuxFromURIWins asserts an explicit xmux in the connection uri is left
// untouched, so an operator can still opt into connection reuse
func TestXmuxFromURIWins(t *testing.T) {
	out := parseURI(t, URI_VLESS_XHTTP_TLS_EXTRA)

	if strings.Contains(out, `"hMaxRequestTimes":1`) {
		t.Errorf("injected xmux overrode the one supplied in the uri:\n%s", out)
	}
	if !strings.Contains(out, `"cMaxReuseTimes"`) {
		t.Errorf("xmux supplied in the uri was dropped:\n%s", out)
	}
}

// TestXmuxOnlyForXhttp asserts non-xhttp transports are untouched: they have no
// dialer-level cache and already reconnect per probe
func TestXmuxOnlyForXhttp(t *testing.T) {
	for name, uri := range map[string]string{
		"raw-reality": URI_VLESS_RAW_REALITY,
		"websocket":   URI_VMESS_WEBSOCKET_TLS,
		"grpc":        URI_VMESS_GRPC_TLS,
		"kcp":         URI_VMESS_MKCP,
	} {
		t.Run(name, func(t *testing.T) {
			if out := parseURI(t, uri); strings.Contains(out, `"xmux"`) {
				t.Errorf("xmux leaked into a non-xhttp config:\n%s", out)
			}
		})
	}
}
