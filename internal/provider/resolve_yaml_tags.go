package provider

import (
	"fmt"
	"os"
	"strings"
)

// resolveYamlTags recursively walks a native Go value and resolves YAML tag strings.
// Currently supports the "!env VARNAME" tag, which is resolved to the value of the
// corresponding environment variable.
func resolveYamlTags(v any) (any, error) {
	switch val := v.(type) {
	case string:
		return resolveTagString(val)
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			resolved, err := resolveYamlTags(v)
			if err != nil {
				return nil, err
			}
			result[k] = resolved
		}
		return result, nil
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			resolved, err := resolveYamlTags(v)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	default:
		// int, float64, bool, nil — pass through unchanged
		return v, nil
	}
}

// resolveTagString checks if a string contains a known YAML tag prefix and resolves it.
func resolveTagString(s string) (any, error) {
	if strings.HasPrefix(s, "!env ") {
		varName := strings.TrimPrefix(s, "!env ")
		varName = strings.TrimSpace(varName)
		value := os.Getenv(varName)
		if value == "" {
			return nil, fmt.Errorf("environment variable %s not set", varName)
		}
		return value, nil
	}
	return s, nil
}
