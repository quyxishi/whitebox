package protocol

import (
	"strconv"

	"github.com/quyxishi/whitebox/internal/serial/xray/outbound/extra"
)

/*
"servers": [
	{
		"address": "1.2.3.4",
		"method": "",
		"ota": false,
		"password": "S9u6hhpqKx",
		"port": 23164,
		"level": 1
	}
]
*/
// skip! go:generate gonstructor --type=TrojanOutbound --constructorTypes=allArgs,builder --output=trojan_gen.go
type TrojanOutbound struct {
	Address  string `json:"address,omitempty"`
	Port     int    `json:"port,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
	Level    int    `json:"level,omitempty"`
}

func ParseTrojanOutbound(con *extra.ConnectionExtra) (out TrojanOutbound, err error) {
	port, err := strconv.Atoi(con.URL.Port())
	if err != nil {
		return out, err
	}

	out = TrojanOutbound{
		Address:  con.URL.Hostname(),
		Port:     port,
		Password: con.URL.User.String(),
		Email:    con.URL.Fragment,
		Level:    1,
	}

	return out, nil
}
