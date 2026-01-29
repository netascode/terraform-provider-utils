package provider

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var tagResolvers = make(map[string]func(*yaml.Node) (*yaml.Node, error))
var tagResolversMutex = &sync.Mutex{}

type CustomTagProcessor struct {
	target interface{}
}

func (i *CustomTagProcessor) UnmarshalYAML(value *yaml.Node) error {
	tagResolversMutex.Lock()
	resolved, err := resolveTags(value)
	tagResolversMutex.Unlock()
	if err != nil {
		return err
	}
	return resolved.Decode(i.target)
}

func resolveTags(node *yaml.Node) (*yaml.Node, error) {
	for tag, fn := range tagResolvers {
		if node.Tag == tag {
			return fn(node)
		}
	}
	if node.Kind == yaml.SequenceNode || node.Kind == yaml.MappingNode {
		var err error
		for i := range node.Content {
			node.Content[i], err = resolveTags(node.Content[i])
			if err != nil {
				return nil, err
			}
		}
	}
	return node, nil
}

func resolveEnv(node *yaml.Node) (*yaml.Node, error) {
	if node.Kind != yaml.ScalarNode {
		return nil, errors.New("!env on a non-scalar node")
	}
	value := os.Getenv(node.Value)
	if value == "" {
		return nil, fmt.Errorf("environment variable %v not set", node.Value)
	}
	node.Value = value
	return node, nil
}

func AddResolvers(tag string, fn func(*yaml.Node) (*yaml.Node, error)) {
	tagResolversMutex.Lock()
	tagResolvers[tag] = fn
	tagResolversMutex.Unlock()
}

func YamlUnmarshal(in []byte, out interface{}) error {
	AddResolvers("!env", resolveEnv)
	err := yaml.Unmarshal(in, &CustomTagProcessor{out})
	return err
}

// needsQuoting determines if a string value needs to be quoted to avoid
// ambiguity when parsed by different YAML implementations (especially Terraform's yamldecode)
func needsQuoting(s string) bool {
	// Empty string needs quoting
	if s == "" {
		return true
	}

	// Check for scientific notation pattern: digits followed by e/E and more digits
	// This pattern matches values that YAML might interpret as scientific notation
	// Examples: 23211e010211, 1e10, 2.5E10
	scientificPattern := regexp.MustCompile(`^[+-]?(\d+\.?\d*|\d*\.?\d+)[eE][+-]?\d+$`)
	if scientificPattern.MatchString(s) {
		return true
	}

	// Check for boolean-like values (case insensitive)
	lower := strings.ToLower(s)
	booleanValues := map[string]bool{
		"true":  true,
		"false": true,
		"yes":   true,
		"no":    true,
		"on":    true,
		"off":   true,
		"y":     true,
		"n":     true,
	}
	if booleanValues[lower] {
		return true
	}

	// Check for null-like values
	if lower == "null" || s == "~" {
		return true
	}

	// Check for pure numeric strings (integers and floats)
	// These might be interpreted as numbers by some YAML parsers
	intPattern := regexp.MustCompile(`^[+-]?\d+$`)
	floatPattern := regexp.MustCompile(`^[+-]?(\d+\.\d*|\d*\.\d+)$`)
	if intPattern.MatchString(s) || floatPattern.MatchString(s) {
		return true
	}

	// Check for octal (0o755), hex (0xFF), or binary (0b1010) patterns
	specialNumPattern := regexp.MustCompile(`^0[xXoObB][0-9a-fA-F]+$`)
	if specialNumPattern.MatchString(s) {
		return true
	}

	// Check for infinity and NaN
	if lower == ".inf" || lower == "-.inf" || lower == "+.inf" || lower == ".nan" {
		return true
	}

	// Check for values starting with special characters that might be interpreted differently
	if len(s) > 0 {
		first := s[0]
		if first == '!' || first == '&' || first == '*' || first == '{' ||
			first == '[' || first == '|' || first == '>' || first == '%' ||
			first == '@' || first == '`' || first == '\'' || first == '"' {
			return true
		}
	}

	// Check for colons followed by space (could be interpreted as key-value)
	if strings.Contains(s, ": ") || strings.HasSuffix(s, ":") {
		return true
	}

	// Check for # which starts comments
	if strings.Contains(s, " #") || strings.HasPrefix(s, "#") {
		return true
	}

	return false
}

// convertToYamlNode converts a Go value to a yaml.Node, applying quoting where needed for strings
func convertToYamlNode(v any) *yaml.Node {
	switch val := v.(type) {
	case string:
		node := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: val,
		}
		if needsQuoting(val) {
			node.Style = yaml.DoubleQuotedStyle
		}
		return node
	case int:
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int"}
		node.Value = fmt.Sprintf("%d", val)
		return node
	case int32:
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int"}
		node.Value = fmt.Sprintf("%d", val)
		return node
	case int64:
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int"}
		node.Value = fmt.Sprintf("%d", val)
		return node
	case float32:
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float"}
		node.Value = fmt.Sprintf("%v", val)
		return node
	case float64:
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float"}
		node.Value = fmt.Sprintf("%v", val)
		return node
	case bool:
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool"}
		node.Value = fmt.Sprintf("%v", val)
		return node
	case []any:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range val {
			node.Content = append(node.Content, convertToYamlNode(item))
		}
		return node
	case map[string]any:
		node := &yaml.Node{Kind: yaml.MappingNode}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
			node.Content = append(node.Content, keyNode, convertToYamlNode(val[k]))
		}
		return node
	default:
		// Fallback for nil and unknown types
		node := &yaml.Node{Kind: yaml.ScalarNode}
		if val == nil {
			node.Tag = "!!null"
			node.Value = "null"
		} else {
			node.Value = fmt.Sprintf("%v", val)
		}
		return node
	}
}

// YamlMarshal marshals a value to YAML with smart quoting to preserve string types
func YamlMarshal(v any) ([]byte, error) {
	node := convertToYamlNode(v)
	return yaml.Marshal(node)
}
