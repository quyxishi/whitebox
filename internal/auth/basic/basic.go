package basic

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/quyxishi/whitebox/internal/auth"
)

// BasicAuthStrategy implements the standard HTTP Basic Authentication scheme (RFC 7617).
// It combines a username and password into a single, base64-encoded string and injects
// it into the HTTP request's Authorization header.
//
// While Basic Auth is conceptually simple, it transmits credentials in an easily
// decodable format. Therefore, it should only be used over encrypted connections (HTTPS/TLS)
// to prevent credential interception.
type BasicAuthStrategy struct {
	// ID is the username/identity used for authentication.
	ID string `yaml:"id,omitempty"`

	// Password is the secret/password used for authentication.
	Password string `yaml:"password,omitempty"`
}

// Name returns the unique identifier for this authentication strategy.
func (_ *BasicAuthStrategy) Name() string {
	return "Basic"
}

// Init initializes the authentication strategy.
// For the basic auth strategy, this is a no-op.
func (h *BasicAuthStrategy) Init() error {
	// no-op
	return nil
}

// Issue formats the ID and Password, encodes them in base64, and injects the
// resulting string into the provided HTTP request Authorization header.
func (h *BasicAuthStrategy) Issue(req *http.Request) error {
	req.Header.Set(
		"authorization",
		"Basic "+base64.StdEncoding.EncodeToString(
			fmt.Appendf(nil, "%s:%s", h.ID, h.Password),
		),
	)
	return nil
}

// Compile-time assertion to ensure that strategy satisfies interface
var _ auth.HttpAuthStrategy = (*BasicAuthStrategy)(nil)
