package extra

import "net/url"

type ConnectionExtra struct {
	URL        *url.URL
	Query      url.Values
	VmessInner *map[string]any
}
