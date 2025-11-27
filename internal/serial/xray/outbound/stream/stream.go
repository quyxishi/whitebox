package stream

import (
	"cmp"
	"errors"
	net "net/url"
	"strings"

	"github.com/quyxishi/whitebox/internal/serial/xray/outbound/extra"
)

const (
	NETWORK_TCP         string = "tcp"
	NETWORK_RAW         string = "raw"
	NETWORK_MKCP        string = "kcp"
	NETWORK_WEBSOCKET   string = "ws"
	NETWORK_GRPC        string = "grpc"
	NETWORK_HTTPUPGRADE string = "httpupgrade"
	NETWORK_XHTTP       string = "xhttp"

	SECURITY_TLS     string = "tls"
	SECURITY_REALITY string = "reality"
)

/* https://xtls.github.io/config/transport.html#streamsettingsobject */
// skip! go:generate gonstructor --type=StreamConfig --constructorTypes=allArgs,builder --output=stream_gen.go
type StreamConfig struct {
	Network  string `json:"network,omitempty"`
	Security string `json:"security,omitempty"`

	RawSettings         *RawConfig           `json:"rawSettings,omitempty"`
	TlsSettings         *TlsConfig           `json:"tlsSettings,omitempty"`
	XhttpSettings       *XhttpConfig         `json:"xhttpSettings,omitempty"`
	RealitySettings     *RealityConfig       `json:"realitySettings,omitempty"`
	GrpcSettings        *GrpcConfig          `json:"grpcSettings,omitempty"`
	WsSettings          *WsConfig            `json:"wsSettings,omitempty"`
	HttpupgradeSettings *HttpUpgradeSettings `json:"httpupgradeSettings,omitempty"`
	KcpSettings         *KcpConfig           `json:"kcpSettings,omitempty"`

	// todo! sockopt
}

func ParseStreamConfig(con *extra.ConnectionExtra) (out StreamConfig, err error) {
	query := con.Query

	switch con.URL.Scheme {
	case extra.SchemeVmess:
		if con.VmessInner == nil {
			return StreamConfig{}, errors.New("extra.ConnectionExtra.VmessInner is nil")
		}
		inner := *con.VmessInner

		mid := net.Values(map[string][]string{
			"type":        {extra.GetOrDefault[string, string](inner, "net")},
			"security":    {extra.GetOrDefault[string, string](inner, "tls")},
			"sni":         {extra.GetOrDefault[string, string](inner, "sni")},
			"fp":          {extra.GetOr(inner, "fp", "chrome")},
			"alpn":        {extra.GetOrDefault[string, string](inner, "alpn")},
			"headerType":  {extra.GetOr(inner, "type", "none")},
			"path":        {extra.GetOr(inner, "path", "/")},
			"host":        {extra.GetOrDefault[string, string](inner, "host")},
			"seed":        {extra.GetOrDefault[string, string](inner, "path")},
			"serviceName": {extra.GetOrDefault[string, string](inner, "path")},
		})
		query = mid
	case extra.SchemeWireguard:
		return StreamConfig{}, nil
	}

	out = StreamConfig{
		Network:  query.Get("type"),
		Security: query.Get("security"),
	}

	// todo! rawSettings parse

	switch out.Network {
	case NETWORK_TCP, NETWORK_RAW:
		out.RawSettings = &RawConfig{
			Header: &RawHeader{HeaderType: cmp.Or(query.Get("headerType"), "none")},
		}
	case NETWORK_MKCP:
		out.KcpSettings = &KcpConfig{
			MTU:              1350,
			TTI:              50,
			UplinkCapacity:   12,
			DownlinkCapacity: 100,
			Congestion:       false,
			ReadBufferSize:   2,
			WriteBufferSize:  2,
			Header:           &KcpHeader{HeaderType: cmp.Or(query.Get("headerType"), "none")},
			Seed:             query.Get("seed"),
		}
	case NETWORK_WEBSOCKET:
		out.WsSettings = &WsConfig{
			Path:    query.Get("path"),
			Host:    query.Get("host"),
			Headers: map[string]string{},
		}
	case NETWORK_GRPC:
		out.GrpcSettings = &GrpcConfig{
			ServiceName:         query.Get("serviceName"),
			MultiMode:           query.Get("mode") == "multi",
			IdleTimeout:         60,
			HealthCheckTimeout:  20,
			PermitWithoutStream: false,
			InitialWindowsSize:  0,
		}
	case NETWORK_HTTPUPGRADE:
		out.HttpupgradeSettings = &HttpUpgradeSettings{
			Path: query.Get("path"),
			Host: query.Get("host"),
		}
	case NETWORK_XHTTP:
		out.XhttpSettings = &XhttpConfig{
			Path: cmp.Or(query.Get("path"), "/"),
			Host: query.Get("host"),
			Mode: query.Get("mode"),
		}
	}

	switch out.Security {
	case SECURITY_TLS:
		out.TlsSettings = &TlsConfig{
			AllowInsecure: query.Get("allowInsecure") == "1",
			SNI:           query.Get("sni"),
			Alpn:          strings.Split(query.Get("alpn"), ","),
			Fingerprint:   query.Get("fp"),
			EchConfigList: query.Get("ech"),
		}
	case SECURITY_REALITY:
		out.RealitySettings = &RealityConfig{
			SNI:           query.Get("sni"),
			Fingerprint:   query.Get("fp"),
			Show:          false,
			PublicKey:     query.Get("pbk"),
			ShortId:       query.Get("sid"),
			SpiderX:       cmp.Or(query.Get("spx"), "/"),
			Mldsa65Verify: query.Get("pqv"),
		}
	}

	return out, nil
}
