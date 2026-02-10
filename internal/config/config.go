package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

const DefaultScopeName string = "default"

type FailIfModule string

const (
	// Drop `Val` in that scenario
	FailIf_SSL FailIfModule = "ssl"

	FailIf_BodyMatchesRegexp  FailIfModule = "body_matches_regexp"
	FailIf_BodyJsonMatchesCEL FailIfModule = "body_json_matches_cel"

	// Expecting `name:value_regexp` as `Val`
	FailIf_HeaderMatchesRegexp FailIfModule = "header_matches_regexp"

	// Expecting a string of response status codes separated by comma as `Val` (e.g., "200,301-399,500")
	FailIf_StatusCodeMatches FailIfModule = "status_code_matches"
)

type WhiteboxConfig struct {
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

type ScopeRecord struct {
	// Fallbacks to 5s by default
	Timeout time.Duration `yaml:"timeout,omitempty"`
	Http    HttpRecord    `yaml:"http,omitempty"`
}

func NewScopeRecord() ScopeRecord {
	return ScopeRecord{
		Timeout: 5 * time.Second,
		Http:    NewHttpRecord(),
	}
}

type HttpRecord struct {
	// Fallbacks to 5 by default
	MaxRedirects int `yaml:"max_redirects,omitempty"`

	// Fallbacks to GET by default
	Method   string            `yaml:"method,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	Body     string            `yaml:"body,omitempty"`
	BodyFile string            `yaml:"body_file,omitempty"`

	// Response validation constraints
	FailIf []FailIfRecord `yaml:"fail_if,omitempty"`
}

func NewHttpRecord() HttpRecord {
	return HttpRecord{
		MaxRedirects: 5,
		Method:       "GET",
		Headers:      make(map[string]string),
	}
}

// Validate ensures the http configuration semantic correctness
func (h *HttpRecord) Validate() error {
	for i, rule := range h.FailIf {
		switch rule.Mod {
		case FailIf_SSL, FailIf_BodyMatchesRegexp, FailIf_BodyJsonMatchesCEL, FailIf_HeaderMatchesRegexp, FailIf_StatusCodeMatches:
			// Valid
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

type FailIfRecord struct {
	// Predicate, see FailIf_* constants modules
	Mod FailIfModule `yaml:"mod,omitempty"`

	// Value
	Val string `yaml:"val,omitempty"`

	// Invert predicate
	Inv bool `yaml:"inv,omitempty"`
}
