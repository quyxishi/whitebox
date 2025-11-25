package stream

/*
"httpupgradeSettings": {
	"path": "/ww",
	"host": "host"
}
*/
// skip! go:generate gonstructor --type=HttpUpgradeSettings --constructorTypes=allArgs,builder --output=http_upgrade_gen.go
type HttpUpgradeSettings struct {
	Path string `json:"path,omitempty"`
	Host string `json:"host,omitempty"`
}
