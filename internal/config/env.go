package config

import (
	"os"
	"regexp"
)

// ${VAR} or ${VAR:-default}
var envVarRegex = regexp.MustCompile(`\${([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?}`)

// Replaces ${VAR} and ${VAR:-default} with actual environment values.
func ExpandEnvironment(data []byte) []byte {
	return envVarRegex.ReplaceAllFunc(data, func(match []byte) []byte {
		s := string(match)

		// ReplaceAllFunc gives the whole match, not subgroups directly
		submatches := envVarRegex.FindStringSubmatch(s)
		if len(submatches) < 2 {
			return match
		}

		var defaultValue string
		if len(submatches) > 2 {
			defaultValue = submatches[2]
		}

		// Lookup environment variable
		val, exists := os.LookupEnv(submatches[1])
		if !exists {
			return []byte(defaultValue)
		}

		return []byte(val)
	})
}
