package serial_test

import (
	"bytes"
	"testing"

	"github.com/quyxishi/whitebox/internal/serial"
	"github.com/xtls/xray-core/core"
)

const (
	URI_VMESS_RAW_TLS   string = "vmess://ewogICJ2IjogIjIiLAogICJwcyI6ICJvMm9qeG8yeiIsCiAgImFkZCI6ICIxLjIuMy40IiwKICAicG9ydCI6IDQ0MywKICAiaWQiOiAiYjc1MjJjODktMTgxMC00N2U2LWExMDMtYjQyNTI4NmRhM2JjIiwKICAic2N5IjogImF1dG8iLAogICJuZXQiOiAidGNwIiwKICAidGxzIjogInRscyIsCiAgInR5cGUiOiAibm9uZSIsCiAgInNuaSI6ICJnb29nbGUuY29tIiwKICAiZnAiOiAiY2hyb21lIiwKICAiYWxwbiI6ICJoMixodHRwLzEuMSIKfQ=="
	URI_VMESS_XHTTP_TLS string = "vmess://ewogICJ2IjogIjIiLAogICJwcyI6ICJvMm9qeG8yeiIsCiAgImFkZCI6ICIxLjIuMy40IiwKICAicG9ydCI6IDQ0MywKICAiaWQiOiAiYjc1MjJjODktMTgxMC00N2U2LWExMDMtYjQyNTI4NmRhM2JjIiwKICAic2N5IjogImF1dG8iLAogICJuZXQiOiAieGh0dHAiLAogICJ0bHMiOiAidGxzIiwKICAicGF0aCI6ICIvIiwKICAiaG9zdCI6ICJoIiwKICAidHlwZSI6ICJhdXRvIiwKICAic25pIjogImdvb2dsZS5jb20iLAogICJmcCI6ICJjaHJvbWUiLAogICJhbHBuIjogImgyLGh0dHAvMS4xIgp9"

	URI_VLESS_RAW_REALITY     string = "vless://c9f5228c-8870-47bd-a92f-9b38c7c02b08@1.2.3.4:443?type=tcp&encryption=none&security=reality&pbk=DF-3KL1W4RuNB2HsgGDwLqHLvvTTN4_QfwUCUn8Uhy0&fp=firefox&sni=google.com&sid=dc8cc0b47450f9&spx=%2F&flow=xtls-rprx-vision#ring0-raii-idx0"
	URI_VLESS_MKCP            string = "vless://0b0f23d5-b18b-4d4f-9fe6-0ca218fb04dc@1.2.3.4:443?type=kcp&encryption=none&headerType=dtls&seed=FnItITnyKO&security=none#j0kc18zb"
	URI_VLESS_WEBSOCKET_TLS   string = "vless://bb5729ac-2f0f-4f2c-812f-86cfcc76c095@1.2.3.4:443?type=ws&encryption=none&path=%2F&host=h&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&sni=google.com&ech=AF3%2BDQBZAAAgACD1OUhamXCNu1WOWNKywNPTujyCiv3QN1cFLN%2BE0hRKfwAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D#89e22ges"
	URI_VLESS_GRPC_REALITY    string = "vless://6883ff2d-d3c1-4597-a541-65c16e41065e@1.2.3.4:443?type=grpc&encryption=none&serviceName=sn&authority=au&security=reality&pbk=WjuE_vMYhy6_CXH1lwtPbllnbQZYHk9nuXubS9YicRU&fp=chrome&sni=google.com&sid=03cbc01665&spx=%2F#9yfgrep5"
	URI_VLESS_HTTPUPGRADE_TLS string = "vless://84b478e1-6010-4560-83c4-b15748f6590d@1.2.3.4:443?type=httpupgrade&encryption=none&path=%2F&host=h&security=tls&fp=chrome&alpn=h2%2Chttp%2F1.1&sni=google.com&ech=AF3%2BDQBZAAAgACD6CLXeT10x7ZYlrBSiwjbKiMKsX40IaoXAsbzhQ5xDdgAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAApnb29nbGUuY29tAAA%3D#l7clt36p"
	URI_VLESS_XHTTP_REALITY   string = "vless://4bdc409f-173c-4f3e-9b38-814af2cff886@1.2.3.4:443?type=xhttp&encryption=none&path=%2F&host=google.com&mode=packet-up&security=reality&pbk=bkjtnsLGV5l4lPp1hN9LwbMK5hIHW_tjqZVdKakxlnY&fp=randomizednoalpn&sni=google.com&sid=f07b3894&spx=%2F#ring0-raii-xhttp"
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
