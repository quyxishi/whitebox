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

	// ...
}

// BasicAuthRecord defines the credentials for HTTP Basic Authentication.
type BasicAuthRecord struct {
	// ID is the username/identity for basic auth.
	ID string `yaml:"id,omitempty"`

	// Password is the secret/password for basic auth.
	Password string `yaml:"password,omitempty"`
}
