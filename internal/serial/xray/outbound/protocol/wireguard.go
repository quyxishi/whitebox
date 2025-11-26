package protocol

import (
	"github.com/quyxishi/whitebox/internal/serial/xray/outbound/extra"
)

/*
[Interface]
PrivateKey = SNnNN7Ixc3tzlXKaI4f86q28V3nxFKf3rchakxmgBls=
Address = 10.0.0.2/32
DNS = 1.1.1.1, 1.0.0.1
MTU = 1420

# -1
[Peer]
PublicKey = y617dCgM3X6lKDjpdt5aGcAZdNYnNOAp0S3jaTljfg0=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 1.2.3.4:27789
*/

/*
{
  "secretKey": "PRIVATE_KEY",
  "address": ["10.0.0.2/32"],
  "peers": [
    {
      "endpoint": "ENDPOINT_ADDR",
      "publicKey": "PUBLIC_KEY"
    }
  ],
  "noKernelTun": false,
  "mtu": 1420,
  "reserved": [1, 2, 3],
  "workers": 2, // optional, default runtime.NumCPU()
  "domainStrategy": "ForceIP"
}
*/
// skip! go:generate gonstructor --type=WireguardOutbound --type=WireguardPeer --constructorTypes=allArgs,builder --output=wireguard_gen.go
type WireguardOutbound struct {
	SecretKey      string           `json:"secretKey,omitempty"`
	Address        []string         `json:"address,omitempty"`
	Peers          []*WireguardPeer `json:"peers,omitempty"`
	NoKernelTun    bool             `json:"noKernelTun,omitempty"`
	MTU            int              `json:"mtu,omitempty"`
	Workers        int              `json:"workers,omitempty"`
	DomainStrategy string           `json:"domainStrategy,omitempty"`
}

type WireguardPeer struct {
	Endpoint  string `json:"endpoint,omitempty"`
	PublicKey string `json:"publicKey,omitempty"`
}

func ParseWireguardOutbound(con *extra.ConnectionExtra) WireguardOutbound {
	return WireguardOutbound{
		SecretKey: con.WireguardInner.Section("Interface").Key("PrivateKey").String(),
		Address:   []string{con.WireguardInner.Section("Interface").Key("Address").String()},
		Peers: []*WireguardPeer{
			{
				Endpoint:  con.WireguardInner.Section("Peer").Key("Endpoint").String(),
				PublicKey: con.WireguardInner.Section("Peer").Key("PublicKey").String(),
			},
		},
		NoKernelTun:    false,
		MTU:            con.WireguardInner.Section("Interface").Key("MTU").MustInt(1420),
		Workers:        2,
		DomainStrategy: "ForceIP",
	}
}
