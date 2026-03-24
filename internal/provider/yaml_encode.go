package provider

import (
	"fmt"

	goyaml "github.com/goccy/go-yaml"
)

// yamlEncode marshals a native Go value to a YAML string using github.com/goccy/go-yaml.
// It produces block-style YAML with 2-space indentation and indented sequences.
func yamlEncode(v any) (string, error) {
	out, err := goyaml.MarshalWithOptions(v, goyaml.IndentSequence(true))
	if err != nil {
		return "", fmt.Errorf("error encoding value to YAML: %w", err)
	}
	return string(out), nil
}
