package bearer

import (
	"net/http"

	"github.com/quyxishi/whitebox/internal/auth"
)

// BearerRawAuthStrategy implements a simple Bearer token authentication strategy.
// It directly injects a static, pre-generated token into the HTTP request's
// Authorization header without performing any cryptographic operations or lifecycle management.
//
// This strategy is ideal for scenarios where the application is provided with a long-lived
// API token that does not require programmatic refreshing or signature validation by the client.
type BearerRawAuthStrategy struct {
	// Credentials holds the static token string to be injected into requests.
	// It should be the raw token value, excluding the "Bearer" prefix.
	Credentials string
}

// Name returns the unique identifier for this authentication strategy.
func (_ *BearerRawAuthStrategy) Name() string {
	return "BearerRaw"
}

// Init initializes the authentication strategy.
// For the raw bearer strategy, this is a no-op.
func (h *BearerRawAuthStrategy) Init() error {
	return nil
}

// Issue injects the configured static credentials into the provided HTTP request.
func (h *BearerRawAuthStrategy) Issue(req *http.Request) error {
	req.Header.Set(
		"authorization",
		"Bearer "+h.Credentials,
	)
	return nil
}

// Compile-time assertion to ensure that strategy satisfies interface
var _ auth.HttpAuthStrategy = (*BearerRawAuthStrategy)(nil)
