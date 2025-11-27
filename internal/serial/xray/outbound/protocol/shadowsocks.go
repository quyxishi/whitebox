package protocol

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/quyxishi/whitebox/internal/serial/xray/outbound/extra"
)

/*
{
  "email": "love@xray.com",
  "address": "1.2.3.4",
  "port": 443,
  "method": "cipher",
  "password": "password",
  "uot": true,
  "UoTVersion": 2,
  "level": 0
}
*/
// skip! go:generate gonstructor --type=ShadowsocksOutbound --constructorTypes=allArgs,builder --output=shadowsocks_gen.go
type ShadowsocksOutbound struct {
	Email      string `json:"email,omitempty"`
	Address    string `json:"address,omitempty"`
	Port       int    `json:"port,omitempty"`
	Method     string `json:"method,omitempty"`
	Password   string `json:"password,omitempty"`
	UOT        bool   `json:"uot,omitempty"`
	UOTVersion int    `json:"UoTVersion,omitempty"`
	Level      int    `json:"level,omitempty"`
}

func ParseShadowsocksOutbound(con *extra.ConnectionExtra) (out ShadowsocksOutbound, err error) {
	port, err := strconv.Atoi(con.URL.Port())
	if err != nil {
		return out, err
	}

	ssExtraRaw, err := base64.RawURLEncoding.DecodeString(con.URL.User.String())
	if err != nil {
		return out, err
	}
	ssExtra := strings.Split(string(ssExtraRaw[:]), ":")

	out = ShadowsocksOutbound{
		Email:      con.URL.Fragment,
		Address:    con.URL.Hostname(),
		Port:       port,
		Method:     ssExtra[0],
		Password:   ssExtra[1],
		UOT:        false,
		UOTVersion: 2,
		Level:      1,
	}

	return out, nil
}
