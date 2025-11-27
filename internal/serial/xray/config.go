package xray

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"github.com/quyxishi/whitebox/internal/serial/xray/outbound"
	"github.com/quyxishi/whitebox/internal/serial/xray/outbound/extra"
	"github.com/quyxishi/whitebox/internal/serial/xray/outbound/protocol"
	"github.com/quyxishi/whitebox/internal/serial/xray/outbound/stream"
	"gopkg.in/ini.v1"
)

/*
{
  "version": {},
  "log": {},
  "api": {},
  "dns": {},
  "routing": {},
  "policy": {},
  "inbounds": [],
  "outbounds": [],
  "transport": {},
  "stats": {},
  "reverse": {},
  "fakedns": {},
  "metrics": {},
  "observatory": {},
  "burstObservatory": {}
}
*/
// skip! go:generate gonstructor --type=XrayConfig --constructorTypes=allArgs,builder --output=config_gen.go
type XrayConfig struct {
	Log       *LogConfig                 `json:"log,omitempty"`
	Outbounds []*outbound.OutboundConfig `json:"outbounds,omitempty"`
}

func (h *XrayConfig) Parse(url *url.URL) (out string, err error) {
	con := extra.ConnectionExtra{
		URL:   url,
		Query: url.Query(),
	}

	switch url.Scheme {
	case extra.SchemeVmess:
		outer, err := base64.StdEncoding.DecodeString(url.Hostname())
		if err != nil {
			return out, err
		}

		var inner map[string]any
		err = json.Unmarshal(outer, &inner)
		if err != nil {
			return out, err
		}

		con.VmessInner = &inner
	case extra.SchemeWireguard:
		outer, err := base64.StdEncoding.DecodeString(url.Hostname())
		if err != nil {
			return out, err
		}

		con.WireguardInner, err = ini.Load(outer)
		if err != nil {
			return out, err
		}
	}

	conProtocol := url.Scheme
	if conProtocol == extra.SchemeShadowsocks {
		conProtocol = "shadowsocks"
	}

	outboundConfig := outbound.OutboundConfig{
		Tag:      "proxy",
		Protocol: conProtocol,
		Mux:      &outbound.MuxConfig{Enabled: false, Concurrency: -1},
	}

	// *

	var protocolOutbound any
	switch url.Scheme {
	case extra.SchemeVmess:
		protocolOutbound, err = protocol.ParseVmessOutbound(&con)
	case extra.SchemeVless:
		protocolOutbound, err = protocol.ParseVlessOutbound(&con)
	case extra.SchemeTrojan:
		protocolOutbound, err = protocol.ParseTrojanOutbound(&con)
	case extra.SchemeShadowsocks:
		protocolOutbound, err = protocol.ParseShadowsocksOutbound(&con)
	case extra.SchemeWireguard:
		protocolOutbound = protocol.ParseWireguardOutbound(&con)
	default:
		err = fmt.Errorf("unexpected schema: %s", url.Scheme)
		log.Printf("[ERROR] serial/xray/parse: %s\n", err)
		return "", err
	}

	if err != nil {
		return "", err
	}

	outboundConfig.Settings = &protocolOutbound

	// *

	stream, err := stream.ParseStreamConfig(&con)
	if err != nil {
		return "", err
	}

	outboundConfig.StreamSettings = &stream

	// *

	h.Outbounds = append(
		h.Outbounds,
		&outboundConfig,
		&outbound.OutboundConfig{Tag: "direct", Protocol: "freedom"},
		&outbound.OutboundConfig{Tag: "block", Protocol: "blackhole"},
	)

	outRaw, err := json.Marshal(h)
	if err != nil {
		return "", err
	}

	return string(outRaw), nil
}
