package protocol

import (
	"strconv"

	"github.com/quyxishi/whitebox/internal/serial/xray/outbound/extra"
)

/*
"vnext": [
	{
		"address": "1.2.3.4",
		"port": 443,
		"users": [
			{
				"id": "a0b642ef-0023-4961-b8d6-49aabf3b5c28",
				"email": "t@t.tt",
				"security": "auto",
				"encryption": "none"
			}
		]
	}
]
*/
// skip! go:generate gonstructor --type=Hysteria2Outbound --constructorTypes=allArgs,builder --output=hysteria2_gen.go
type Hysteria2Outbound struct {
	Address string `json:"address,omitempty"`
	Port    int    `json:"port,omitempty"`
	Version int    `json:"version,omitempty"`
}

func ParseHysteria2Outbound(con *extra.ConnectionExtra) (out Hysteria2Outbound, err error) {
	port, err := strconv.Atoi(con.URL.Port())
	if err != nil {
		return out, err
	}

	out = Hysteria2Outbound{
		Address: con.URL.Hostname(),
		Port:    port,
		Version: 2,
	}

	return out, nil
}
