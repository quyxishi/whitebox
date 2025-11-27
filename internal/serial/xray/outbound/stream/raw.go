package stream

/*
{
	"acceptProxyProtocol": false,
	"header": {
		"type": "none"
	}
}
*/
// skip! go:generate gonstructor --type=RawConfig --type=RawHeader --constructorTypes=allArgs,builder --output=reality_gen.go
type RawConfig struct {
	AcceptProxyProtocol bool       `json:"acceptProxyProtocol,omitempty"`
	Header              *RawHeader `json:"header,omitempty"`
}

type RawHeader struct {
	HeaderType string `json:"type,omitempty"`
}
