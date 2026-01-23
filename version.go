package whitebox

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var version string

func Version() string {
	return strings.TrimSpace(version)
}
