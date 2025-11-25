package xray

import (
	"github.com/quyxishi/whitebox/internal/serial/xray/outbound"
)

const (
	SchemaVless = "vless://"
)

/*
{
  "version": {},
  "log": {},
  "api": {},
  "dns": {},
  "routing": {},
  "policy": {},
  "inbounds": [],
  "outbounds": [],
  "transport": {},
  "stats": {},
  "reverse": {},
  "fakedns": {},
  "metrics": {},
  "observatory": {},
  "burstObservatory": {}
}
*/
// skip! go:generate gonstructor --type=XrayConfig --constructorTypes=allArgs,builder --output=config_gen.go
type XrayConfig struct {
	Log       *LogConfig
	Outbounds []*outbound.OutboundConfig
}
