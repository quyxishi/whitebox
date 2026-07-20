package stream

/*
{
	"auth": "56fa060c-b509-4151-8d02-1e42de582bf9",
	"version": 2
}
*/
// skip! go:generate gonstructor --type=HysteriaSettings --constructorTypes=allArgs,builder --output=hysteria_gen.go
type HysteriaSettings struct {
	Auth    string `json:"auth,omitempty"`
	Version int    `json:"version,omitempty"`
}
