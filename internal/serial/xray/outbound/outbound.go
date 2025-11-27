package outbound

import "github.com/quyxishi/whitebox/internal/serial/xray/outbound/stream"

/*
{
	"sendThrough": "0.0.0.0",
	"protocol": "vless",
	"settings": {},
	"tag": "tag",
	"streamSettings": {},
	"proxySettings": {
		"tag": "tag"
	},
	"mux": {}
}
*/
// skip! go:generate gonstructor --type=OutboundConfig --constructorTypes=allArgs,builder --output=outbound_gen.go
type OutboundConfig struct {
	SendThrough    string               `json:"sendThrough,omitempty"`
	Protocol       string               `json:"protocol,omitempty"`
	Settings       *any                 `json:"settings,omitempty"`
	Tag            string               `json:"tag,omitempty"`
	StreamSettings *stream.StreamConfig `json:"streamSettings,omitempty"`
	ProxySettings  *ProxyConfig         `json:"proxySettings,omitempty"`
	Mux            *MuxConfig           `json:"mux,omitempty"`
}
