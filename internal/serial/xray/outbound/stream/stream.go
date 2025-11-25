package stream

/* https://xtls.github.io/config/transport.html#streamsettingsobject */
// skip! go:generate gonstructor --type=StreamConfig --constructorTypes=allArgs,builder --output=stream_gen.go
type StreamConfig struct {
	Network  string `json:"network"`
	Security string `json:"security"`

	RawSettings         *RawConfig           `json:"rawSettings,omitempty"`
	TlsSettings         *TlsSettings         `json:"tlsSettings,omitempty"`
	XhttpSettings       *XhttpConfig         `json:"xhttpSettings,omitempty"`
	RealitySettings     *RealityConfig       `json:"realitySettings,omitempty"`
	GrpcSettings        *GrpcConfig          `json:"grpcSettings,omitempty"`
	WsSettings          *WsConfig            `json:"wsSettings,omitempty"`
	HttpupgradeSettings *HttpUpgradeSettings `json:"httpupgradeSettings,omitempty"`
	KcpSettings         *KcpConfig           `json:"kcpSettings,omitempty"`

	// todo! sockopt
}
