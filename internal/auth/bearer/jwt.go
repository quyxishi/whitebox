package bearer

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/quyxishi/whitebox/internal/auth"
)

// BearerJWTAuthStrategy implements a JWT-based authentication strategy.
// It handles token generation, payload/header construction, automatic key parsing
// for various cryptographic algorithms (HMAC, RSA, ECDSA), and automatic token
// injection into HTTP request Authorization headers.
//
// When Refresh is enabled, the strategy automatically tracks expiration times
// and regenerates new tokens just before the current token expires, making it
// suitable for persistent client sessions.
type BearerJWTAuthStrategy struct {
	// SigningMethod defines the cryptographic algorithm used to sign the JWT
	// (e.g., jwt.SigningMethodHS256, jwt.SigningMethodRS256).
	SigningMethod jwt.SigningMethod

	// Headers is a map of custom claims to include in the JWT header.
	// Standard claims like `alg` (signature or encryption algorithm) and `typ`
	// (type of token) are handled automatically.
	Headers map[string]any

	// Claims is a map of custom claims to include in the JWT payload.
	// Standard claims like `iat` (issued at) and `exp` (expiration) are handled
	// automatically if Refresh is enabled.
	Claims map[string]any

	// Refresh controls whether the JWT's temporal claims (`iat`, `exp`)
	// are automatically updated.
	// If true new tokens are generated periodically based on the Interval.
	Refresh bool

	// Interval specifies the duration between token regenerations when Refresh is true.
	// It effectively sets the validity period of the JWT.
	Interval time.Duration

	// Key provides the secret or private key used to sign the JWT.
	// - For HMAC algorithms (HS*), this is the shared secret.
	// - For RSA/ECDSA algorithms (RS*, ES*, PS*), this is the PEM-encoded private key.
	Key string

	// KeyFile is the path to a file containing the signing key.
	// If specified, the content of this file takes precedence over the Key field.
	KeyFile string

	mu        sync.RWMutex
	inner     string
	key       any
	expiresAt time.Time
}

// Name returns the unique identifier for this authentication strategy.
func (_ *BearerJWTAuthStrategy) Name() string {
	return "BearerJWT"
}

// Init initializes the authentication strategy. It validates that a signing key
// is configured, injects required JWT headers (alg, typ), and sets up the initial
// token claims. Init must be called exactly once before calling Issue.
func (h *BearerJWTAuthStrategy) Init() error {
	if h.SigningMethod == nil {
		return errors.New("signing method must not be nil")
	}

	if h.Headers == nil {
		h.Headers = make(map[string]any)
	}
	if h.Claims == nil {
		h.Claims = make(map[string]any)
	}

	var keyData []byte
	if h.KeyFile != "" {
		data, err := os.ReadFile(h.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to read key file: %w", err)
		}
		keyData = data
	} else if h.Key != "" {
		keyData = []byte(h.Key)
	} else {
		return errors.New("a key (or key_file) is required for JWT signing")
	}

	alg := h.SigningMethod.Alg()

	var key any
	var err error
	switch strings.ToUpper(alg[:2]) {
	case "RS", "PS":
		key, err = jwt.ParseRSAPrivateKeyFromPEM(keyData)
	case "ES":
		key, err = jwt.ParseECPrivateKeyFromPEM(keyData)
	case "ED":
		key, err = jwt.ParseEdPrivateKeyFromPEM(keyData)
	default:
		key = keyData
	}
	if err != nil {
		return fmt.Errorf("failed to parse PK for %s due: %v", alg, err)
	}

	h.key = key
	h.Headers["alg"] = alg
	h.Headers["typ"] = "JWT"

	return h.generateToken(time.Now())
}

// Issue injects a valid JWT into the provided HTTP request's Authorization header.
//
// If Refresh is true and the cached token is expired, Issue will generate a new token.
// Otherwise, it returns the cached token to minimize cryptographic overhead.
func (h *BearerJWTAuthStrategy) Issue(req *http.Request) error {
	// Fast-path: Read lock to check validity
	h.mu.RLock()
	token := h.inner
	needsRefresh := h.Refresh && time.Now().After(h.expiresAt.Add(-time.Second))
	h.mu.RUnlock()

	if token == "" {
		return errors.New("JWT must be initialized before issuing")
	}

	// Slow-path: Token expired, acquire write lock and refresh
	if needsRefresh {
		h.mu.Lock()

		// Double-check pattern (another goroutine might have refreshed it while we waited for the lock)
		if time.Now().After(h.expiresAt.Add(-time.Second)) {
			if err := h.generateToken(time.Now()); err != nil {
				h.mu.Unlock()
				return fmt.Errorf("failed to refresh JWT due: %v", err)
			}
		}

		token = h.inner
		h.mu.Unlock()
	}

	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// generateToken mints a completely new JWT (must be called while holding a write lock).
func (h *BearerJWTAuthStrategy) generateToken(now time.Time) error {
	if h.Refresh {
		h.Claims["iat"] = jwt.NewNumericDate(now)
		h.expiresAt = now.Add(h.Interval)
		h.Claims["exp"] = jwt.NewNumericDate(h.expiresAt)
	}

	// Create a fresh token struct to avoid mutation side-effects
	token := jwt.NewWithClaims(h.SigningMethod, jwt.MapClaims(h.Claims))
	maps.Copy(token.Header, h.Headers)

	signed, err := token.SignedString(h.key)
	if err != nil {
		return err
	}

	h.inner = signed
	return nil
}

// Compile-time assertion to ensure that strategy satisfies interface
var _ auth.HttpAuthStrategy = (*BearerJWTAuthStrategy)(nil)
