package outbound

/*
"proxySettings": {
	"tag": "tag"
},
*/
// skip! go:generate gonstructor --type=ProxyConfig --constructorTypes=allArgs,builder --output=proxy_gen.go
type ProxyConfig struct {
	Tag string `json:"tag,omitempty"`
}
