package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

// DefaultScopeName is the reserved identifier for the fallback configuration scope.
//
// This constant is implicitly selected when the `scope` query parameter is
// omitted or empty in an incoming request.
// It ensures the application resolves to a valid execution context even
// when no specific target is explicitly defined.
const DefaultScopeName string = "default"

// FailIfModule defines the strategy used to inspect the HTTP response.
type FailIfModule string

const (
	// FailIf_SSL triggers a failure if the SSL/TLS handshake is performed in the final trace.
	// The `Val` field is ignored.
	FailIf_SSL FailIfModule = "ssl"

	// FailIf_BodyMatchesRegexp triggers a failure if the response body
	// matches the regular expression provided in `Val`.
	FailIf_BodyMatchesRegexp FailIfModule = "body_matches_regexp"

	// FailIf_BodyJsonMatchesCEL triggers a failure if the response body
	// (parsed as JSON) satisfies the CEL (Common Expression Language)
	// predicate provided in `Val`.
	FailIf_BodyJsonMatchesCEL FailIfModule = "body_json_matches_cel"

	// FailIf_HeaderMatchesRegexp triggers a failure if a specific header
	// matches a pattern.
	// Format expected in `Val`: "Header-Name:Regexp"
	FailIf_HeaderMatchesRegexp FailIfModule = "header_matches_regexp"

	// FailIf_StatusCodeMatches triggers a failure if the response status code
	// matches the provided list or range.
	// Format expected in `Val`: comma-separated values (e.g., "404,500-599").
	FailIf_StatusCodeMatches FailIfModule = "status_code_matches"
)

// BearerAuthType designates the strategy used to construct or sign the
// Bearer token for the Authorization header.
type BearerAuthType string

const (
	// BearerAuth_RAW indicates that the token is a static literal.
	// The value is transmitted exactly as provided.
	BearerAuth_RAW BearerAuthType = "raw"

	// --- HMAC-SHA Family (Symmetric) ---
	// These algorithms employ a shared secret key to sign the JWT.

	// BearerAuth_JWT_HS256 uses HMAC with SHA-256.
	BearerAuth_JWT_HS256 BearerAuthType = "jwt_hs256"
	// BearerAuth_JWT_HS384 uses HMAC with SHA-384.
	BearerAuth_JWT_HS384 BearerAuthType = "jwt_hs384"
	// BearerAuth_JWT_HS512 uses HMAC with SHA-512.
	BearerAuth_JWT_HS512 BearerAuthType = "jwt_hs512"

	// --- RSA Family (Asymmetric) ---
	// These algorithms use the RSA PKCS#1 v1.5 signature scheme.
	// They require a private key for signing and a public key for verification.

	// BearerAuth_JWT_RS256 uses RSA with SHA-256.
	BearerAuth_JWT_RS256 BearerAuthType = "jwt_rs256"
	// BearerAuth_JWT_RS384 uses RSA with SHA-384.
	BearerAuth_JWT_RS384 BearerAuthType = "jwt_rs384"
	// BearerAuth_JWT_RS512 uses RSA with SHA-512.
	BearerAuth_JWT_RS512 BearerAuthType = "jwt_rs512"

	// --- ECDSA Family (Asymmetric) ---
	// These algorithms use Elliptic Curve Digital Signature Algorithm.
	// They generally offer better performance and smaller key sizes than RSA.

	// BearerAuth_JWT_ES256 uses ECDSA with P-256 and SHA-256.
	BearerAuth_JWT_ES256 BearerAuthType = "jwt_es256"
	// BearerAuth_JWT_ES384 uses ECDSA with P-384 and SHA-384.
	BearerAuth_JWT_ES384 BearerAuthType = "jwt_es384"
	// BearerAuth_JWT_ES512 uses ECDSA with P-521 and SHA-512.
	BearerAuth_JWT_ES512 BearerAuthType = "jwt_es512"

	// --- RSASSA-PSS Family (Asymmetric) ---
	// These algorithms use RSA Probabilistic Signature Scheme.
	// PSS is cryptographically superior to PKCS#1 v1.5 and is recommended
	// for modern implementations.

	// BearerAuth_JWT_PS256 uses RSASSA-PSS with SHA-256.
	BearerAuth_JWT_PS256 BearerAuthType = "jwt_ps256"
	// BearerAuth_JWT_PS384 uses RSASSA-PSS with SHA-384.
	BearerAuth_JWT_PS384 BearerAuthType = "jwt_ps384"
	// BearerAuth_JWT_PS512 uses RSASSA-PSS with SHA-512.
	BearerAuth_JWT_PS512 BearerAuthType = "jwt_ps512"
)

// WhiteboxConfig represents the root configuration structure for the monitoring application.
// It acts as a container for named scopes, where each scope represents a distinct
// testing scenario or target environment.
type WhiteboxConfig struct {
	// Scopes is a map where the key is a unique identifier for the test scope
	// and the value is the configuration for that specific scenario.
	Scopes map[string]ScopeRecord `yaml:"scopes,omitempty"`
}

func NewWhiteboxConfig() WhiteboxConfig {
	return WhiteboxConfig{
		Scopes: map[string]ScopeRecord{DefaultScopeName: NewScopeRecord()},
	}
}

// Load reads a YAML file from the given path and returns the parsed configuration
func Load(path string) (*WhiteboxConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("unable to read whitebox config file", "err", err)
		return nil, err
	}

	expandedData := ExpandEnvironment(data)

	var config WhiteboxConfig
	if err := yaml.Unmarshal(expandedData, &config); err != nil {
		slog.Error("unable to parse whitebox config file", "err", err)
		return nil, err
	}

	for name, scope := range config.Scopes {
		if err := scope.Http.Validate(); err != nil {
			slog.Error("whitebox scope configuration is invalid", "name", name, "err", err)
			return nil, fmt.Errorf("invalid scope configuration: %v", err)
		}
	}

	if _, ok := config.Scopes[DefaultScopeName]; !ok {
		config.Scopes[DefaultScopeName] = NewScopeRecord()
	}

	return &config, nil
}

// ScopeRecord defines the execution parameters for a single testing scope.
// A scope encapsulates the timing constraints and the HTTP definition
// required to perform a check.
type ScopeRecord struct {
	// Timeout specifies the maximum duration allowed for the entire scope execution
	// before it is cancelled. (default: 5s)
	Timeout time.Duration `yaml:"timeout,omitempty"`

	// Http contains the specific HTTP request definition and validation rules
	// for this scope.
	Http HttpRecord `yaml:"http,omitempty"`
}

func NewScopeRecord() ScopeRecord {
	return ScopeRecord{
		Timeout: 5 * time.Second,
		Http:    NewHttpRecord(),
	}
}

// HttpRecord details the HTTP request to be sent and the criteria used
// to validate the response.
type HttpRecord struct {
	// MaxRedirects is the maximum number of HTTP redirects (3xx)
	// the client will follow. (default: 5)
	MaxRedirects int `yaml:"max_redirects,omitempty"`

	// Method is the HTTP verb to use for the request. (default: GET)
	Method string `yaml:"method,omitempty"`

	// Headers is a map of HTTP non-canonical header names to their values
	// to be included in the request.
	Headers map[string]string `yaml:"headers,omitempty"`

	// Body provides the raw string content to use as the request body.
	Body string `yaml:"body,omitempty"`

	// BodyFile is the file path from which to read the request body content.
	// If Body is also set, this field takes precedence.
	BodyFile string `yaml:"body_file,omitempty"`

	// FailIf is a list of response validation constraints.
	// See example configuration for further details.
	FailIf []FailIfRecord `yaml:"fail_if,omitempty"`

	// Auth contains credentials for authentication strategies.
	Auth AuthRecord `yaml:"auth,omitempty"`
}

func NewHttpRecord() HttpRecord {
	return HttpRecord{
		MaxRedirects: 5,
		Method:       "GET",
		Headers:      make(map[string]string),
	}
}

// Validate ensures the HTTP configuration semantic correctness
func (h *HttpRecord) Validate() error {
	for i, rule := range h.FailIf {
		switch rule.Mod {
		case FailIf_SSL, FailIf_BodyMatchesRegexp, FailIf_BodyJsonMatchesCEL, FailIf_HeaderMatchesRegexp, FailIf_StatusCodeMatches:
			// Valid
			// todo! regexp & cel validation
		case "":
			return fmt.Errorf("http.fail_if[%d]: mod is required", i)
		default:
			return fmt.Errorf("http.fail_if[%d]: unknown module '%s'", i, rule.Mod)
		}

		if rule.Val == "" && rule.Mod != FailIf_SSL {
			return fmt.Errorf("http.fail_if[%d]: val (pattern/expression) cannot be empty", i)
		}
	}
	return nil
}

// FailIfRecord represents a single assertion rule applied to the HTTP response.
// It follows a "fail-on-match" logic unless inverted.
type FailIfRecord struct {
	// Predicate, see `FailIf_*` constants modules
	Mod FailIfModule `yaml:"mod,omitempty"`

	// Val is the argument for the module. Its format depends on the `Mod` selected
	// (e.g., a regex string, a CEL expression, or a status code range).
	Val string `yaml:"val,omitempty"`

	// Inv inverts the result of the check.
	// - If Inv is false (default): Fail if the condition matches.
	// - If Inv is true: Fail if the condition does NOT match.
	Inv bool `yaml:"inv,omitempty"`
}

// AuthRecord holds configuration for various authentication methods.
// Note: Only one authentication method should be configured per scope.
type AuthRecord struct {
	// Basic configures HTTP Basic Authentication (RFC 7617).
	Basic BasicAuthRecord `yaml:"basic,omitempty"`

	// Bearer configures HTTP Bearer Authentication (RFC 6750).
	Bearer BearerAuthRecord `yaml:"bearer,omitempty"`

	// ...
}

// BasicAuthRecord defines the credentials for HTTP Basic Authentication (RFC 7617).
type BasicAuthRecord struct {
	// ID is the username/identity for basic auth.
	ID string `yaml:"id,omitempty"`

	// Password is the secret/password for basic auth.
	Password string `yaml:"password,omitempty"`
}

// BearerAuthRecord defines the configuration for HTTP Bearer Authentication (RFC 6750).
// It supports both static tokens and dynamic JWT generation signed with various algorithms.
type BearerAuthRecord struct {
	// Kind specifies the type of Bearer token to use.
	// It determines whether the token is a static string ("raw") or a dynamically
	// generated JWT signed with a specific algorithm (see `BearerAuth_JWT_*` constants algorithms).
	Kind BearerAuthType `yaml:"kind,omitempty"`

	// Credentials holds the static token string.
	// This field is only used when Kind is set to "raw".
	Credentials string `yaml:"credentials,omitempty"`

	// Headers is a map of custom claims to include in the JWT header.
	// Standard claims like `alg` (signature or encryption algorithm) and `typ`
	// (type of token) are handled automatically.
	Headers map[string]any `yaml:"headers,omitempty"`

	// Claims is a map of custom claims to include in the JWT payload.
	// Standard claims like `iat` (issued at) and `exp` (expiration) are handled
	// automatically if Refresh is enabled.
	Claims map[string]any `yaml:"claims,omitempty"`

	// Refresh controls whether the JWT's temporal claims (`iat`, `exp`)
	// are automatically updated.
	// If true (default), new tokens are generated periodically based on the Interval.
	Refresh bool `yaml:"refresh,omitempty"`

	// Interval specifies the duration between token regenerations when Refresh is true.
	// It effectively sets the validity period of the JWT. (default: 15m)
	Interval time.Duration `yaml:"interval,omitempty"`

	// Key provides the secret or private key used to sign the JWT.
	// - For HMAC algorithms (HS*), this is the shared secret.
	// - For RSA/ECDSA algorithms (RS*, ES*, PS*), this is the PEM-encoded private key.
	Key string `yaml:"key,omitempty"`

	// KeyFile is the path to a file containing the signing key.
	// If specified, the content of this file takes precedence over the Key field.
	KeyFile string `yaml:"key_file,omitempty"`
}
