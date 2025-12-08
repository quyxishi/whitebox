package extra

import (
	"net/url"

	"gopkg.in/ini.v1"
)

type ConnectionExtra struct {
	URL            *url.URL
	Query          url.Values
	VmessInner     *map[string]any
	WireguardInner *ini.File
	AmneziaWGInner *ini.File
}
