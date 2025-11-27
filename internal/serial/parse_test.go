package serial_test

import (
	"bytes"
	"testing"

	"github.com/quyxishi/whitebox/internal/serial"
	"github.com/xtls/xray-core/core"
)

const (
	URI_VMESS_RAW_TLS         string = "vmess://ewogICJ2IjogIjIiLAogICJwcyI6ICJvMm9qeG8yeiIsCiAgImFkZCI6ICIxLjIuMy40IiwKICAicG9ydCI6IDQ0MywKICAiaWQiOiAiYjc1MjJjODktMTgxMC00N2U2LWExMDMtYjQyNTI4NmRhM2JjIiwKICAic2N5IjogImF1dG8iLAogICJuZXQiOiAidGNwIiwKICAidGxzIjogInRscyIsCiAgInR5cGUiOiAibm9uZSIsCiAgInNuaSI6ICJnb29nbGUuY29tIiwKICAiZnAiOiAiY2hyb21lIiwKICAiYWxwbiI6ICJoMixodHRwLzEuMSIKfQ=="
	URI_VMESS_MKCP            string = "vmess://ewogICJ2IjogIjIiLAogICJwcyI6ICJ0Z2huaWxpdiIsCiAgImFkZCI6ICIxLjIuMy40IiwKICAicG9ydCI6IDQ0MywKICAiaWQiOiAiZGU3Yjk0YzAtMTk5Ny00YzZiLWI4MGQtZWJmNjgzZTg3OWMzIiwKICAic2N5IjogImF1dG8iLAogICJuZXQiOiAia2NwIiwKICAidGxzIjogIm5vbmUiLAogICJ0eXBlIjogImR0bHMiLAogICJwYXRoIjogInphRTg0eGE2dTgiCn0="
	URI_VMESS_WEBSOCKET_TLS   string = "vmess://ewogICJ2IjogIjIiLAogICJwcyI6ICJ0Z2huaWxpdiIsCiAgImFkZCI6ICIxLjIuMy40IiwKICAicG9ydCI6IDQ0MywKICAiaWQiOiAiZGU3Yjk0YzAtMTk5Ny00YzZiLWI4MGQtZWJmNjgzZTg3OWMzIiwKICAic2N5IjogImF1dG8iLAogICJuZXQiOiAid3MiLAogICJ0bHMiOiAidGxzIiwKICAicGF0aCI6ICIvcCIsCiAgImhvc3QiOiAiaCIsCiAgInNuaSI6ICJnb29nbGUuY29tIiwKICAiZnAiOiAiY2hyb21lIiwKICAiYWxwbiI6ICJoMixodHRwLzEuMSIKfQ=="
	URI_VMESS_GRPC_TLS        string = "vmess://ewogICJ2IjogIjIiLAogICJwcyI6ICJ0Z2huaWxpdiIsCiAgImFkZCI6ICIxLjIuMy40IiwKICAicG9ydCI6IDQ0MywKICAiaWQiOiAiZGU3Yjk0YzAtMTk5Ny00YzZiLWI4MGQtZWJmNjgzZTg3OWMzIiwKICAic2N5IjogImF1dG8iLAogICJuZXQiOiAiZ3JwYyIsCiAgInRscyI6ICJ0bHMiLAogICJwYXRoIjogInNuIiwKICAiYXV0aG9yaXR5IjogImF1IiwKICAic25pIjogImdvb2dsZS5jb20iLAogICJmcCI6ICJjaHJvbWUiLAogICJhbHBuIjogImgyLGh0dHAvMS4xIgp9"
	URI_VMESS_HTTPUPGRADE_TLS string = "vmess://ewogICJ2IjogIjIiLAogICJwcyI6ICJ0Z2huaWxpdiIsCiAgImFkZCI6ICIxLjIuMy40IiwKICAicG9ydCI6IDQ0MywKICAiaWQiOiAiZGU3Yjk0YzAtMTk5Ny00YzZiLWI4MGQtZWJmNjgzZTg3OWMzIiwKICAic2N5IjogImF1dG8iLAogICJuZXQiOiAiaHR0cHVwZ3JhZGUiLAogICJ0bHMiOiAidGxzIiwKICAicGF0aCI6ICIvcCIsCiAgImhvc3QiOiAiaCIsCiAgInNuaSI6ICJnb29nbGUuY29tIiwKICAiZnAiOiAiY2hyb21lIiwKICAiYWxwbiI6ICJoMixodHRwLzEuMSIKfQ=="
	URI_VMESS_XHTTP_TLS       string = "vmess://ewogICJ2IjogIjIiLAogICJwcyI6ICJvMm9qeG8yeiIsCiAgImFkZCI6ICIxLjIuMy40IiwKICAicG9ydCI6IDQ0MywKICAiaWQiOiAiYjc1MjJjODktMTgxMC00N2U2LWExMDMtYjQyNTI4NmRhM2JjIiwKICAic2N5IjogImF1dG8iLAogICJuZXQiOiAieGh0dHAiLAogICJ0bHMiOiAidGxzIiwKICAicGF0aCI6ICIvIiwKICAiaG9zdCI6ICJoIiwKICAidHlwZSI6ICJhdXRvIiwKICAic25pIjogImdvb2dsZS5jb20iLAogICJmcCI6ICJjaHJvbWUiLAogICJhbHBuIjogImgyLGh0dHAvMS4xIgp9"

	URI_VLESS_RAW_REALITY     string = "vless://c9f5228c-8870-47bd-a92f-9b38c7c02b08@1.2.3.4:443?type=tcp&encryption=none&security=reality&pbk=DF-3KL1W4RuNB2HsgGDwLqHLvvTTN4_QfwUCUn8Uhy0&fp=firefox&sni=google.com&sid=dc8cc0b47450f9&spx=%2F&flow=xtls-rprx-vision#ring0-raii-idx0"
	URI_VLESS_MKCP            string = "vless://0b0f23d5-b18b-4d4f-9fe6-0ca218fb04dc@1.2.3.4:443?type=kcp&encryption=none&headerType=dtls&seed=FnItITnyKO&security=none#j0kc18zb"
	URI_VLESS_WEBSOCKET_TLS   string = "vless://bb5729ac-2f0f-4f2c-812f-86cfcc76c095@1.2.3.4:443?type=ws&encryption=none&path=%2F&host=h&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&sni=google.com&ech=AF3%2BDQBZAAAgACD1OUhamXCNu1WOWNKywNPTujyCiv3QN1cFLN%2BE0hRKfwAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D#89e22ges"
	URI_VLESS_GRPC_REALITY    string = "vless://6883ff2d-d3c1-4597-a541-65c16e41065e@1.2.3.4:443?type=grpc&encryption=none&serviceName=sn&authority=au&security=reality&pbk=WjuE_vMYhy6_CXH1lwtPbllnbQZYHk9nuXubS9YicRU&fp=chrome&sni=google.com&sid=03cbc01665&spx=%2F#9yfgrep5"
	URI_VLESS_HTTPUPGRADE_TLS string = "vless://84b478e1-6010-4560-83c4-b15748f6590d@1.2.3.4:443?type=httpupgrade&encryption=none&path=%2F&host=h&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&sni=google.com&ech=AF3%2BDQBZAAAgACD6CLXeT10x7ZYlrBSiwjbKiMKsX40IaoXAsbzhQ5xDdgAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D#l7clt36p"
	URI_VLESS_XHTTP_REALITY   string = "vless://4bdc409f-173c-4f3e-9b38-814af2cff886@1.2.3.4:443?type=xhttp&encryption=none&path=%2F&host=google.com&mode=packet-up&security=reality&pbk=bkjtnsLGV5l4lPp1hN9LwbMK5hIHW_tjqZVdKakxlnY&fp=randomizednoalpn&sni=google.com&sid=f07b3894&spx=%2F#ring0-raii-xhttp"

	URI_TROJAN_RAW_REALITY     string = "trojan://Vtvxlvq2ku@1.2.3.4:443?type=tcp&security=reality&pbk=rh_ToroMlTyQIYIQcH41RmIiaHr5FKtnByRUYA82i3o&fp=chrome&sni=google.com&sid=9675d1&spx=%2F#42j1zdc0"
	URI_TROJAN_MKCP            string = "trojan://Vtvxlvq2ku@1.2.3.4:443?type=kcp&headerType=dtls&seed=8omUFe2xgl&security=none#42j1zdc0"
	URI_TROJAN_WEBSOCKET_TLS   string = "trojan://Vtvxlvq2ku@1.2.3.4:443?type=ws&path=%2F&host=&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&ech=AF3%2BDQBZAAAgACAGdEfh%2FUGI%2By7XzHs1FPgnkmxw0Ryv09Jm%2B19RCBMvEgAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D&sni=google.com#42j1zdc0"
	URI_TROJAN_GRPC_TLS        string = "trojan://Vtvxlvq2ku@1.2.3.4:443?type=grpc&serviceName=sn&authority=au&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&ech=AF3%2BDQBZAAAgACAGdEfh%2FUGI%2By7XzHs1FPgnkmxw0Ryv09Jm%2B19RCBMvEgAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D&sni=google.com#42j1zdc0"
	URI_TROJAN_HTTPUPGRADE_TLS string = "trojan://Vtvxlvq2ku@1.2.3.4:443?type=httpupgrade&path=%2Fp&host=h&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&ech=AF3%2BDQBZAAAgACAGdEfh%2FUGI%2By7XzHs1FPgnkmxw0Ryv09Jm%2B19RCBMvEgAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D&sni=google.com#42j1zdc0"
	URI_TROJAN_XHTTP_REALITY   string = "trojan://Vtvxlvq2ku@1.2.3.4:443?type=xhttp&path=%2F&host=&mode=auto&security=reality&pbk=TYPtWkMZ2VTWKYJnrmpPR8KM8zq9lLps-FtU9GMEWFM&fp=chrome&sni=google.com&sid=52&spx=%2F#42j1zdc0"

	URI_SHADOWSOCKS_RAW_TLS         string = "ss://MjAyMi1ibGFrZTMtY2hhY2hhMjAtcG9seTEzMDU6SFJvWEFQRWh1M1BLQWk1bCtvWnR0MmxReDA3NUVVUktRWnpLa1BVeWlqWT0@1.2.3.4:443?type=tcp&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&ech=AF3%2BDQBZAAAgACA%2BQLsnaagAGGUJVcHx5XuKje8dAGIus54wdz11d1dbFAAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D&sni=google.com"
	URI_SHADOWSOCKS_MKCP            string = "ss://MjAyMi1ibGFrZTMtY2hhY2hhMjAtcG9seTEzMDU6SFJvWEFQRWh1M1BLQWk1bCtvWnR0MmxReDA3NUVVUktRWnpLa1BVeWlqWT0@1.2.3.4:443?type=kcp&headerType=dtls&seed=5y65LuZh6j"
	URI_SHADOWSOCKS_WEBSOCKET_TLS   string = "ss://MjAyMi1ibGFrZTMtY2hhY2hhMjAtcG9seTEzMDU6SFJvWEFQRWh1M1BLQWk1bCtvWnR0MmxReDA3NUVVUktRWnpLa1BVeWlqWT0@1.2.3.4:443?type=ws&path=%2Fp&host=h&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&ech=AF3%2BDQBZAAAgACD21HO7PNeZMft4mWQiZguw0MzGRrK2oNZjXe8gbTbfGQAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D&sni=google.com"
	URI_SHADOWSOCKS_GRPC_TLS        string = "ss://MjAyMi1ibGFrZTMtY2hhY2hhMjAtcG9seTEzMDU6SFJvWEFQRWh1M1BLQWk1bCtvWnR0MmxReDA3NUVVUktRWnpLa1BVeWlqWT0@1.2.3.4:443?type=grpc&serviceName=sn&authority=au&mode=multi&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&ech=AF3%2BDQBZAAAgACD21HO7PNeZMft4mWQiZguw0MzGRrK2oNZjXe8gbTbfGQAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D&sni=google.com"
	URI_SHADOWSOCKS_HTTPUPGRADE_TLS string = "ss://MjAyMi1ibGFrZTMtY2hhY2hhMjAtcG9seTEzMDU6SFJvWEFQRWh1M1BLQWk1bCtvWnR0MmxReDA3NUVVUktRWnpLa1BVeWlqWT0@1.2.3.4:443?type=httpupgrade&path=%2Fp&host=h&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&ech=AF3%2BDQBZAAAgACD21HO7PNeZMft4mWQiZguw0MzGRrK2oNZjXe8gbTbfGQAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D&sni=google.com"
	URI_SHADOWSOCKS_XHTTP_TLS       string = "ss://MjAyMi1ibGFrZTMtY2hhY2hhMjAtcG9seTEzMDU6SFJvWEFQRWh1M1BLQWk1bCtvWnR0MmxReDA3NUVVUktRWnpLa1BVeWlqWT0@1.2.3.4:443?type=xhttp&path=%2Fp&host=h&mode=auto&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&ech=AF3%2BDQBZAAAgACD21HO7PNeZMft4mWQiZguw0MzGRrK2oNZjXe8gbTbfGQAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D&sni=google.com"

	URI_WIREGUARD string = "wireguard://W0ludGVyZmFjZV0KUHJpdmF0ZUtleSA9IFNObk5ON0l4YzN0emxYS2FJNGY4NnEyOFYzbnhGS2YxcmNoYWt4bWdBbHM9CkFkZHJlc3MgPSAxMC4wLjAuMi8zMgpETlMgPSAxLjEuMS4xLCAxLjAuMC4xCk1UVSA9IDE0MjAKCiMgLTEKW1BlZXJdClB1YmxpY0tleSA9IHk2MTdkQ2dNM1g2bEtEanBkdDVhR2NBWmROWW5OT0FwMFMyanFUbGpmZzA9CkFsbG93ZWRJUHMgPSAwLjAuMC4wLzAsIDo6LzAKRW5kcG9pbnQgPSAxLjIuMy40OjI3Nzg5"

	// URI_SUBJSON_VLESS string = "http://1.2.3.4:2096/json/l08vryrtn0gb07s4"
)

func xrayParseAndLoad(t *testing.T, uri string) {
	cfg, err := serial.ParseURI(serial.CONFIG_BACKEND_XRAYCORE, uri, &serial.ParseParams{EnableDebug: true})
	if err != nil {
		t.Errorf("error occurred during parsing: %v", err)
		return
	}

	t.Log(cfg)

	_, err = core.LoadConfig("json", bytes.NewReader([]byte(cfg)))
	if err != nil {
		t.Errorf("unable to load xray config: %s", err.Error())
		return
	}

	t.Log("successfully loaded uri-based parsed config into xray-core")
}

// -- VMESS

func TestParseURI_VmessRawTls(t *testing.T) {
	xrayParseAndLoad(t, URI_VMESS_RAW_TLS)
}

func TestParseURI_VmessMkcp(t *testing.T) {
	xrayParseAndLoad(t, URI_VMESS_MKCP)
}

func TestParseURI_VmessWebsocketTls(t *testing.T) {
	xrayParseAndLoad(t, URI_VMESS_WEBSOCKET_TLS)
}

func TestParseURI_VmessGrpcTls(t *testing.T) {
	xrayParseAndLoad(t, URI_VMESS_GRPC_TLS)
}

func TestParseURI_VmessHttpupgradeTls(t *testing.T) {
	xrayParseAndLoad(t, URI_VMESS_HTTPUPGRADE_TLS)
}

func TestParseURI_VmessXhttpTls(t *testing.T) {
	xrayParseAndLoad(t, URI_VMESS_XHTTP_TLS)
}

// -- VLESS

func TestParseURI_VlessRawReality(t *testing.T) {
	xrayParseAndLoad(t, URI_VLESS_RAW_REALITY)
}

func TestParseURI_VlessMkcp(t *testing.T) {
	xrayParseAndLoad(t, URI_VLESS_MKCP)
}

func TestParseURI_VlessWebsocketTls(t *testing.T) {
	xrayParseAndLoad(t, URI_VLESS_WEBSOCKET_TLS)
}

func TestParseURI_VlessGrpcReality(t *testing.T) {
	xrayParseAndLoad(t, URI_VLESS_GRPC_REALITY)
}

func TestParseURI_VlessHttpupgradeTls(t *testing.T) {
	xrayParseAndLoad(t, URI_VLESS_HTTPUPGRADE_TLS)
}

func TestParseURI_VlessXhttpReality(t *testing.T) {
	xrayParseAndLoad(t, URI_VLESS_XHTTP_REALITY)
}

// -- TROJAN

func TestParseURI_TrojanRawReality(t *testing.T) {
	xrayParseAndLoad(t, URI_TROJAN_RAW_REALITY)
}

func TestParseURI_TrojanMkcp(t *testing.T) {
	xrayParseAndLoad(t, URI_TROJAN_MKCP)
}

func TestParseURI_TrojanWebsocketTls(t *testing.T) {
	xrayParseAndLoad(t, URI_TROJAN_WEBSOCKET_TLS)
}

func TestParseURI_TrojanGrpcTls(t *testing.T) {
	xrayParseAndLoad(t, URI_TROJAN_GRPC_TLS)
}

func TestParseURI_TrojanHttpupgradeTls(t *testing.T) {
	xrayParseAndLoad(t, URI_TROJAN_HTTPUPGRADE_TLS)
}

func TestParseURI_TrojanXhttpReality(t *testing.T) {
	xrayParseAndLoad(t, URI_TROJAN_XHTTP_REALITY)
}

// -- SHADOWSOCKS

func TestParseURI_ShadowsocksRawTls(t *testing.T) {
	xrayParseAndLoad(t, URI_SHADOWSOCKS_RAW_TLS)
}

func TestParseURI_ShadowsocksMkcp(t *testing.T) {
	xrayParseAndLoad(t, URI_SHADOWSOCKS_MKCP)
}

func TestParseURI_ShadowsocksWebsocketTls(t *testing.T) {
	xrayParseAndLoad(t, URI_SHADOWSOCKS_WEBSOCKET_TLS)
}

func TestParseURI_ShadowsocksGrpcTls(t *testing.T) {
	xrayParseAndLoad(t, URI_SHADOWSOCKS_GRPC_TLS)
}

func TestParseURI_ShadowsocksHttpupgradeTls(t *testing.T) {
	xrayParseAndLoad(t, URI_SHADOWSOCKS_HTTPUPGRADE_TLS)
}

func TestParseURI_ShadowsocksXhttpTls(t *testing.T) {
	xrayParseAndLoad(t, URI_SHADOWSOCKS_XHTTP_TLS)
}

// -- WIREGUARD

func TestParseURI_Wireguard(t *testing.T) {
	xrayParseAndLoad(t, URI_WIREGUARD)
}

// -- SUBJSON

// func TestParseSubscriptionURI_Vless(t *testing.T) {
// 	cfg, err := serial.ParseSubscriptionURI(URI_SUBJSON_VLESS, &serial.ParseSubParams{EnableDebug: true})
// 	if err != nil {
// 		t.Errorf("error occurred during parsing: %v", err)
// 		return
// 	}

// 	t.Log(cfg)

// 	_, err = core.LoadConfig("json", bytes.NewReader([]byte(cfg)))
// 	if err != nil {
// 		t.Errorf("unable to load xray config: %s", err.Error())
// 		return
// 	}

// 	t.Log("successfully loaded uri-based parsed config into xray-core")
// }
