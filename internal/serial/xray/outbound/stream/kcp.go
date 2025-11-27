package stream

/*
"kcpSettings": {
	"mtu": 1350,
	"tti": 50,
	"uplinkCapacity": 12,
	"downlinkCapacity": 100,
	"congestion": false,
	"readBufferSize": 2,
	"writeBufferSize": 2,
	"header": {
		"type": "dtls"
	},
	"seed": "FzxCFgDiim"
}
*/
// skip! go:generate gonstructor --type=KcpConfig --type=KcpHeader --constructorTypes=allArgs,builder --output=kcp_gen.go
type KcpConfig struct {
	MTU              int        `json:"mtu"`
	TTI              int        `json:"tti"`
	UplinkCapacity   int        `json:"uplinkCapacity"`
	DownlinkCapacity int        `json:"downlinkCapacity"`
	Congestion       bool       `json:"congestion"`
	ReadBufferSize   int        `json:"readBufferSize"`
	WriteBufferSize  int        `json:"writeBufferSize"`
	Header           *KcpHeader `json:"header,omitempty"`
	Seed             string     `json:"seed,omitempty"`
}

type KcpHeader struct {
	HeaderType string `json:"type,omitempty"`
}
