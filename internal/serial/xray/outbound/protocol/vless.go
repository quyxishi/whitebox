package protocol

import (
	"cmp"
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
// skip! go:generate gonstructor --type=VlessOutbound --type=VnextOutbound --type=VlessUsers --constructorTypes=allArgs,builder --output=vless_gen.go
type VlessOutbound struct {
	Vnext []*VnextOutbound `json:"vnext,omitempty"`
}

type VlessUsers struct {
	ID         string `json:"id,omitempty"`
	Email      string `json:"email,omitempty"`
	Security   string `json:"security,omitempty"`
	Encryption string `json:"encryption,omitempty"`
	Flow       string `json:"flow,omitempty"`
}

func ParseVlessOutbound(con *extra.ConnectionExtra) (out VlessOutbound, err error) {
	port, err := strconv.Atoi(con.URL.Port())
	if err != nil {
		return out, err
	}

	out = VlessOutbound{
		Vnext: []*VnextOutbound{
			{
				Address: con.URL.Hostname(),
				Port:    port,
				Users: []any{
					&VlessUsers{
						ID:         con.URL.User.Username(),
						Email:      cmp.Or(con.URL.Fragment, "t@t.tt"),
						Security:   cmp.Or(con.Query.Get("security"), "auto"),
						Encryption: cmp.Or(con.Query.Get("encryption"), "none"),
						Flow:       cmp.Or(con.Query.Get("flow"), "xtls-rprx-vision"),
					},
				},
			},
		},
	}

	return out, nil
}
