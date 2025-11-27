package stream

/*
"xhttpSettings": {
	"path": "/",
	"host": "google.com",
	"mode": "stream-up"
},
*/
// skip! go:generate gonstructor --type=XhttpConfig --constructorTypes=allArgs,builder --output=xhttp_gen.go
type XhttpConfig struct {
	Path string `json:"path,omitempty"`
	Host string `json:"host,omitempty"`
	Mode string `json:"mode,omitempty"`
}
