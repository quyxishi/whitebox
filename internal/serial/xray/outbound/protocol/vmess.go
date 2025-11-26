package protocol

import (
	"errors"

	"github.com/quyxishi/whitebox/internal/serial/xray/outbound/extra"
)

/*
"vnext": [
	{
		"address": "1.2.3.4",
		"port": 443,
		"users": [
			{
				"id": "b7522c89-1810-47e6-a103-b425286da3bc",
				"alterId": 0,
				"email": "t@t.tt",
				"security": "auto"
			}
		]
	}
]
*/
// skip! go:generate gonstructor --type=VmessOutbound --type=VnextOutbound --type=VmessUsers --constructorTypes=allArgs,builder --output=vmess_gen.go
type VmessOutbound struct {
	Vnext []*VnextOutbound `json:"vnext,omitempty"`
}

type VnextOutbound struct {
	Address string `json:"address,omitempty"`
	Port    int    `json:"port,omitempty"`
	Users   []any  `json:"users,omitempty"`
}

type VmessUsers struct {
	ID       string `json:"id,omitempty"`
	AlterID  int    `json:"alterId,omitempty"`
	Email    string `json:"email,omitempty"`
	Security string `json:"security,omitempty"`
}

func ParseVmessOutbound(con *extra.ConnectionExtra) (out VmessOutbound, err error) {
	if con.VmessInner == nil {
		return VmessOutbound{}, errors.New("extra.ConnectionExtra.VmessInner is nil")
	}
	inner := *con.VmessInner

	out = VmessOutbound{
		Vnext: []*VnextOutbound{
			{
				Address: inner["add"].(string),
				Port:    int(inner["port"].(float64)),
				Users: []any{
					&VmessUsers{
						ID:       inner["id"].(string),
						AlterID:  0,
						Email:    extra.GetOr(inner, "ps", "t@t.tt"),
						Security: extra.GetOr(inner, "scy", "auto"),
					},
				},
			},
		},
	}

	return out, nil
}
