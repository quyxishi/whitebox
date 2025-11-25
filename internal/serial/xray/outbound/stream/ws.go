package stream

/*
{
  "acceptProxyProtocol": false,
  "path": "/",
  "host": "xray.com",
  "headers": {
    "key": "value"
  },
  "heartbeatPeriod": 10
}
*/
// skip! go:generate gonstructor --type=WsConfig --constructorTypes=allArgs,builder --output=ws_gen.go
type WsConfig struct {
	AcceptProxyProtocol bool              `json:"acceptProxyProtocol,omitempty"`
	Path                string            `json:"path,omitempty"`
	Host                string            `json:"host,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	HeartbeatPeriod     int               `json:"heartbeatPeriod,omitempty"`
}
