package xray

/*
"log": {
	"access": "path",
	"error": "path",
	"loglevel": "warning",
	"dnsLog": false,
	"maskAddress": ""
}
*/
// skip! go:generate gonstructor --type=LogConfig --constructorTypes=allArgs,builder --output=log_gen.go
type LogConfig struct {
	Access      string `json:"access,omitempty"`
	Error       string `json:"error,omitempty"`
	Loglevel    string `json:"loglevel,omitempty"`
	DnsLog      bool   `json:"dnsLog,omitempty"`
	MaskAddress string `json:"maskAddress,omitempty"`
}
