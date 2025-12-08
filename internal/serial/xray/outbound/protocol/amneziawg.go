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
Jc = 3
Jmin = 50
Jmax = 1000
S1 = 20
S2 = 78
H1 = 39131278
H2 = 832138185
H3 = 1436957857
H4 = 1635877746

[Peer]
PublicKey = y617dCgM3X6lKDjpdt5aGcAZdNYnNOAp0S3jaTljfg0=
PresharedKey = fbDzRFA/PGH05Uwl9EFcI42F8KABRpSKoqS+Xv2qOl8=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 1.2.3.4:27789
*/

/*
AmneziaWG JSON config for xray-core (amnezia fork):
{
  "secretKey": "PRIVATE_KEY",
  "address": ["10.0.0.2/32"],
  "peers": [
    {
      "endpoint": "ENDPOINT_ADDR",
      "publicKey": "PUBLIC_KEY",
      "preSharedKey": "PRESHARED_KEY"
    }
  ],
  "noKernelTun": false,
  "mtu": 1420,
  "workers": 2,
  "domainStrategy": "ForceIP",
  "jc": 3,
  "jmin": 50,
  "jmax": 1000,
  "s1": 20,
  "s2": 78,
  "h1": 39131278,
  "h2": 832138185,
  "h3": 1436957857,
  "h4": 1635877746
}
*/

// AmneziaWGOutbound represents AmneziaWG protocol configuration for xray-core
// This extends WireGuard with obfuscation parameters (Jc, Jmin, Jmax, S1, S2, H1-H4)
type AmneziaWGOutbound struct {
	SecretKey      string            `json:"secretKey,omitempty"`
	Address        []string          `json:"address,omitempty"`
	Peers          []*AmneziaWGPeer  `json:"peers,omitempty"`
	NoKernelTun    bool              `json:"noKernelTun,omitempty"`
	MTU            int               `json:"mtu,omitempty"`
	Workers        int               `json:"workers,omitempty"`
	DomainStrategy string            `json:"domainStrategy,omitempty"`

	// AmneziaWG obfuscation parameters
	// Jc - number of junk packets before handshake (0-128)
	Jc int `json:"jc,omitempty"`
	// Jmin - minimum junk packet size in bytes (0-1280)
	Jmin int `json:"jmin,omitempty"`
	// Jmax - maximum junk packet size in bytes (Jmin <= Jmax <= 1280)
	Jmax int `json:"jmax,omitempty"`
	// S1 - init packet junk size (random bytes added to init packet)
	S1 int `json:"s1,omitempty"`
	// S2 - response packet junk size (random bytes added to response packet)
	S2 int `json:"s2,omitempty"`
	// H1 - init packet magic header (replaces WG type 1)
	H1 uint32 `json:"h1,omitempty"`
	// H2 - response packet magic header (replaces WG type 2)
	H2 uint32 `json:"h2,omitempty"`
	// H3 - underload packet magic header (replaces WG type 3)
	H3 uint32 `json:"h3,omitempty"`
	// H4 - transport packet magic header (replaces WG type 4)
	H4 uint32 `json:"h4,omitempty"`
}

type AmneziaWGPeer struct {
	Endpoint     string `json:"endpoint,omitempty"`
	PublicKey    string `json:"publicKey,omitempty"`
	PreSharedKey string `json:"preSharedKey,omitempty"`
}

// ParseAmneziaWGOutbound parses AmneziaWG config from ini file format
func ParseAmneziaWGOutbound(con *extra.ConnectionExtra) AmneziaWGOutbound {
	iface := con.AmneziaWGInner.Section("Interface")
	peer := con.AmneziaWGInner.Section("Peer")

	return AmneziaWGOutbound{
		SecretKey: iface.Key("PrivateKey").String(),
		Address:   []string{iface.Key("Address").String()},
		Peers: []*AmneziaWGPeer{
			{
				Endpoint:     peer.Key("Endpoint").String(),
				PublicKey:    peer.Key("PublicKey").String(),
				PreSharedKey: peer.Key("PresharedKey").String(),
			},
		},
		NoKernelTun:    false,
		MTU:            iface.Key("MTU").MustInt(1420),
		Workers:        2,
		DomainStrategy: "ForceIP",

		// AmneziaWG obfuscation parameters with defaults for backward compatibility
		// If all are 0, it works as regular WireGuard
		Jc:   iface.Key("Jc").MustInt(0),
		Jmin: iface.Key("Jmin").MustInt(0),
		Jmax: iface.Key("Jmax").MustInt(0),
		S1:   iface.Key("S1").MustInt(0),
		S2:   iface.Key("S2").MustInt(0),
		H1:   uint32(iface.Key("H1").MustUint(1)), // Default WG type 1
		H2:   uint32(iface.Key("H2").MustUint(2)), // Default WG type 2
		H3:   uint32(iface.Key("H3").MustUint(3)), // Default WG type 3
		H4:   uint32(iface.Key("H4").MustUint(4)), // Default WG type 4
	}
}