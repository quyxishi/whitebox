package protocol

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

type VnextOutbound struct {
	Address string        `json:"address,omitempty"`
	Port    int           `json:"port,omitempty"`
	Users   []*VlessUsers `json:"users,omitempty"`
}

type VlessUsers struct {
	ID         string `json:"id,omitempty"`
	Email      string `json:"email,omitempty"`
	Security   string `json:"security,omitempty"`
	Encryption string `json:"encryption,omitempty"`
	Flow       string `json:"flow,omitempty"`
}
