package serial

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gvcgo/vpnparser/pkgs/outbound/xray"
	"github.com/gvcgo/vpnparser/pkgs/utils"
	"gopkg.in/ini.v1"
)

const SchemaWireguard string = "wireguard://"

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

type ParserWireguard struct {
	SecretKey  string
	Address    string
	Port       int
	DNS        []string
	MTU        int
	PublicKey  string
	AllowedIPs []string
	Endpoint   string
}

func (h *ParserWireguard) Parse(rawUri string) error {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(rawUri, SchemaWireguard))
	if err != nil {
		return err
	}

	cfg, err := ini.Load(raw)
	if err != nil {
		return err
	}

	h.SecretKey = cfg.Section("Interface").Key("PrivateKey").String()
	h.Address = cfg.Section("Interface").Key("Address").String()
	if port, err := strconv.Atoi(strings.Split(cfg.Section("Peer").Key("Endpoint").String(), ":")[1]); err == nil {
		h.Port = port
	}
	h.DNS = cfg.Section("Interface").Key("DNS").Strings(",")
	h.MTU = cfg.Section("Interface").Key("MTU").MustInt(1420)
	h.PublicKey = cfg.Section("Peer").Key("PublicKey").String()
	h.AllowedIPs = cfg.Section("Peer").Key("AllowedIPs").Strings(",")
	h.Endpoint = cfg.Section("Peer").Key("Endpoint").String()

	return nil
}

func (h *ParserWireguard) GetAddr() string {
	return h.Endpoint
}

func (h *ParserWireguard) Show() {
	fmt.Printf("addr: %s, pubkey: %s\n",
		h.Endpoint,
		h.PublicKey)
}

/*
https://xtls.github.io/config/outbounds/wireguard.html#outboundconfigurationobject

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

var XrayWireguard string = `{
	"secretKey": "PRIVATE_KEY",
	"address": ["10.0.0.1"],
	"peers": [
		{
			"endpoint": "ENDPOINT_ADDR",
			"publicKey": "PUBLIC_KEY"
		}
	],
	"noKernelTun": false,
	"mtu": 1420,
	"workers": 2,
	"domainStrategy": "ForceIP"
}`

type WireguardOut struct {
	RawUri   string
	Parser   *ParserWireguard
	outbound string
}

func (h *WireguardOut) Parse(rawUri string) {
	h.Parser = &ParserWireguard{}
	h.Parser.Parse(rawUri)
}

func (h *WireguardOut) Addr() string {
	if h.Parser == nil {
		return ""
	}
	return h.Parser.GetAddr()
}

func (h *WireguardOut) Port() int {
	return h.Parser.Port
}

func (h *WireguardOut) Scheme() string {
	return SchemaWireguard
}

func (h *WireguardOut) GetRawUri() string {
	return h.RawUri
}

func (h *WireguardOut) getSettings() string {
	j := gjson.New(XrayWireguard)

	j.Set("secretKey", h.Parser.SecretKey)
	j.Set("peers.0.endpoint", h.Parser.Endpoint)
	j.Set("peers.0.publicKey", h.Parser.PublicKey)
	j.Set("peers.0.allowedIPs", h.Parser.AllowedIPs)
	j.Set("dns.servers", h.Parser.DNS)
	j.Set("mtu", h.Parser.MTU)

	return j.MustToJsonString()
}

func (h *WireguardOut) setProtocolAndTag(outStr string) string {
	j := gjson.New(outStr)

	j.Set("protocol", "wireguard")
	j.Set("tag", utils.OutboundTag)

	return j.MustToJsonString()
}

func (h *WireguardOut) GetOutboundStr() string {
	if h.Parser.Endpoint == "" {
		return ""
	}

	if h.outbound == "" {
		settings := h.getSettings()
		outStr := fmt.Sprintf(xray.XrayOut, settings, "{}")
		h.outbound = h.setProtocolAndTag(outStr)
	}

	return h.outbound
}

func TestWireguard() {
	rawUri := "wireguard://W0ludGVyZmFjZV0KUHJpdmF0ZUtleSA9IFNObk5ON0l4YzN0emxYS2FJNGY4NnEyOFYzbnhGS2YxcmNoYWt4bWdBbHM9CkFkZHJlc3MgPSAxMC4wLjAuMi8zMgpETlMgPSAxLjEuMS4xLCAxLjAuMC4xCk1UVSA9IDE0MjAKCiMgLTEKW1BlZXJdClB1YmxpY0tleSA9IHk2MTdkQ2dNM1g2bEtEanBkdDVhR2NBWmROWW5OT0FwMFMyanFUbGpmZzA9CkFsbG93ZWRJUHMgPSAwLjAuMC4wLzAsIDo6LzAKRW5kcG9pbnQgPSAxLjIuMy40OjI3Nzg5"
	vo := &WireguardOut{}
	vo.Parse(rawUri)
	o := vo.GetOutboundStr()
	fmt.Println(o)
}
