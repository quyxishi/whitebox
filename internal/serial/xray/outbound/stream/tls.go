package stream

/*
"tlsSettings": {
	"allowInsecure": false,
	"serverName": "google.com",
	"alpn": [
		"h2",
		"http/1.1"
	],
	"fingerprint": "chrome"
},
*/
// skip! go:generate gonstructor --type=TlsConfig --constructorTypes=allArgs,builder --output=tls_gen.go
type TlsConfig struct {
	AllowInsecure bool     `json:"allowInsecure,omitempty"`
	SNI           string   `json:"serverName,omitempty"`
	Alpn          []string `json:"alpn,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
}
