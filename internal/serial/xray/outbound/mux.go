package outbound

/*
{
  "enabled": true,
  "concurrency": 8,
  "xudpConcurrency": 16,
  "xudpProxyUDP443": "reject"
}
*/
// skip! go:generate gonstructor --type=MuxConfig --constructorTypes=allArgs,builder --output=mux_gen.go
type MuxConfig struct {
	Enabled         bool   `json:"enabled,omitempty"`
	Concurrency     int    `json:"concurrency,omitempty"`
	XudpConcurrency int    `json:"xudpConcurrency,omitempty"`
	XudpProxyUDP443 string `json:"xudpProxyUDP443,omitempty"`
}
