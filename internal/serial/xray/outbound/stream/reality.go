package stream

/*
"realitySettings": {
	"serverName": "google.com",
	"fingerprint": "chrome",
	"show": false,
	"publicKey": "vlde4nWVaNSpOhdaw1ZrQgK3Kk114yReazONKdwyawgZ1wU",
	"shortId": "006ad485ab8a",
	"spiderX": "/",
	"mldsa65Verify": ""
},
*/
// skip! go:generate gonstructor --type=RealityConfig --constructorTypes=allArgs,builder --output=reality_gen.go
type RealityConfig struct {
	SNI           string `json:"serverName,omitempty"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	Show          bool   `json:"show,omitempty"`
	PublicKey     string `json:"publicKey,omitempty"`
	ShortId       string `json:"shortId,omitempty"`
	SpiderX       string `json:"spiderX,omitempty"`
	Mldsa65Verify string `json:"mldsa65Verify,omitempty"`
}
