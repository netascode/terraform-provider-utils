package provider

import (
	"fmt"
	"math"
	"strconv"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// yamlDecode parses a YAML string to a native Go value, preserving unknown tags
// as literal strings (e.g., "!env ABC" → "!env ABC").
func yamlDecode(input string) (any, error) {
	file, err := parser.ParseBytes([]byte(input), 0)
	if err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}

	if len(file.Docs) == 0 {
		return nil, nil
	}
	if len(file.Docs) > 1 {
		return nil, fmt.Errorf("multiple YAML documents are not supported (expected 1, got %d)", len(file.Docs))
	}

	doc := file.Docs[0]
	if doc.Body == nil {
		return nil, nil
	}

	decoder := &yamlDecoder{
		anchors: make(map[string]any),
	}
	return decoder.traverseNode(doc.Body)
}

// yamlDecoder holds state during AST traversal (e.g., anchor resolution).
type yamlDecoder struct {
	anchors map[string]any
}

// traverseNode recursively walks the AST and returns a native Go value.
func (d *yamlDecoder) traverseNode(node ast.Node) (any, error) {
	if node == nil {
		return nil, nil
	}

	switch n := node.(type) {
	case *ast.TagNode:
		return d.handleTag(n)

	case *ast.NullNode:
		return nil, nil

	case *ast.StringNode:
		return n.Value, nil

	case *ast.IntegerNode:
		return d.handleInteger(n)

	case *ast.FloatNode:
		return n.Value, nil

	case *ast.BoolNode:
		return n.Value, nil

	case *ast.LiteralNode:
		return n.Value.Value, nil

	case *ast.InfinityNode:
		return n.Value, nil

	case *ast.NanNode:
		return math.NaN(), nil

	case *ast.MappingNode:
		return d.handleMapping(n)

	case *ast.MappingValueNode:
		// A single key-value pair treated as a one-element map
		return d.handleMappingValue(n)

	case *ast.SequenceNode:
		return d.handleSequence(n)

	case *ast.AnchorNode:
		return d.handleAnchor(n)

	case *ast.AliasNode:
		return d.handleAlias(n)

	default:
		return nil, fmt.Errorf("unsupported YAML node type: %T", node)
	}
}

// handleTag processes tagged YAML values. Standard tags are handled normally;
// unknown tags on scalar values are preserved as literal strings.
func (d *yamlDecoder) handleTag(n *ast.TagNode) (any, error) {
	tag := n.Start.Value

	if isStandardTag(tag) {
		return d.handleStandardTag(tag, n.Value)
	}

	// Unknown tag — only allowed on scalar values, preserved as "!tag value"
	_, isScalar := n.Value.(ast.ScalarNode)
	if !isScalar {
		return nil, fmt.Errorf("unsupported tag %q on non-scalar value", tag)
	}

	value, err := d.traverseNode(n.Value)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("%s %v", tag, value), nil
}

// isStandardTag checks whether a YAML tag is a standard YAML 1.2 tag.
func isStandardTag(tag string) bool {
	switch tag {
	case "!!str", "!!int", "!!float", "!!bool", "!!null",
		"!!map", "!!seq", "!!timestamp", "!!binary":
		return true
	}
	return false
}

// handleStandardTag processes standard YAML tags.
func (d *yamlDecoder) handleStandardTag(tag string, value ast.Node) (any, error) {
	switch tag {
	case "!!str":
		v, err := d.traverseNode(value)
		if err != nil {
			return nil, err
		}
		return fmt.Sprintf("%v", v), nil

	case "!!int":
		v, err := d.traverseNode(value)
		if err != nil {
			return nil, err
		}
		switch val := v.(type) {
		case int:
			return val, nil
		case int64:
			return int(val), nil
		case uint64:
			return int(val), nil
		case string:
			i, err := strconv.ParseInt(val, 0, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %q to integer: %w", val, err)
			}
			return int(i), nil
		default:
			return nil, fmt.Errorf("cannot convert %T to integer", v)
		}

	case "!!float":
		v, err := d.traverseNode(value)
		if err != nil {
			return nil, err
		}
		switch val := v.(type) {
		case float64:
			return val, nil
		case int:
			return float64(val), nil
		case string:
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %q to float: %w", val, err)
			}
			return f, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to float", v)
		}

	case "!!bool":
		v, err := d.traverseNode(value)
		if err != nil {
			return nil, err
		}
		switch val := v.(type) {
		case bool:
			return val, nil
		case string:
			b, err := strconv.ParseBool(val)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %q to bool: %w", val, err)
			}
			return b, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to bool", v)
		}

	case "!!null":
		return nil, nil

	case "!!map":
		return d.traverseNode(value)

	case "!!seq":
		return d.traverseNode(value)

	case "!!timestamp":
		// Preserve timestamp as its original string representation
		if scalar, ok := value.(ast.ScalarNode); ok {
			return fmt.Sprintf("%v", scalar.GetValue()), nil
		}
		return value.String(), nil

	case "!!binary":
		// Preserve binary data as its base64 string representation
		if scalar, ok := value.(ast.ScalarNode); ok {
			return fmt.Sprintf("%v", scalar.GetValue()), nil
		}
		return value.String(), nil

	default:
		return d.traverseNode(value)
	}
}

// handleInteger converts an IntegerNode value to a Go int.
func (d *yamlDecoder) handleInteger(n *ast.IntegerNode) (any, error) {
	switch v := n.Value.(type) {
	case int64:
		return int(v), nil
	case uint64:
		return int(v), nil
	default:
		return nil, fmt.Errorf("unexpected integer type: %T", n.Value)
	}
}

// handleMapping converts a MappingNode to a map[string]any.
func (d *yamlDecoder) handleMapping(n *ast.MappingNode) (any, error) {
	result := make(map[string]any, len(n.Values))
	for _, mv := range n.Values {
		key, err := d.extractMapKey(mv.Key)
		if err != nil {
			return nil, err
		}

		if mv.Key.IsMergeKey() {
			// Handle merge key (<<) — merge the referenced map into the result
			mergedVal, err := d.traverseNode(mv.Value)
			if err != nil {
				return nil, err
			}
			switch merged := mergedVal.(type) {
			case map[string]any:
				for k, v := range merged {
					if _, exists := result[k]; !exists {
						result[k] = v
					}
				}
			case []any:
				// Merge key with sequence of maps
				for _, item := range merged {
					if m, ok := item.(map[string]any); ok {
						for k, v := range m {
							if _, exists := result[k]; !exists {
								result[k] = v
							}
						}
					}
				}
			default:
				return nil, fmt.Errorf("merge key (<<) value must be a map or list of maps")
			}
			continue
		}

		value, err := d.traverseNode(mv.Value)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

// handleMappingValue handles a standalone MappingValueNode (single key-value pair).
func (d *yamlDecoder) handleMappingValue(n *ast.MappingValueNode) (any, error) {
	key, err := d.extractMapKey(n.Key)
	if err != nil {
		return nil, err
	}
	value, err := d.traverseNode(n.Value)
	if err != nil {
		return nil, err
	}
	return map[string]any{key: value}, nil
}

// extractMapKey extracts a string key from a MapKeyNode.
func (d *yamlDecoder) extractMapKey(keyNode ast.MapKeyNode) (string, error) {
	switch k := keyNode.(type) {
	case *ast.MappingKeyNode:
		val, err := d.traverseNode(k.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%v", val), nil
	default:
		// For simple scalar keys used directly as MapKeyNode
		if scalar, ok := keyNode.(ast.ScalarNode); ok {
			return fmt.Sprintf("%v", scalar.GetValue()), nil
		}
		return "", fmt.Errorf("unsupported map key type: %T", keyNode)
	}
}

// handleSequence converts a SequenceNode to a []any.
func (d *yamlDecoder) handleSequence(n *ast.SequenceNode) (any, error) {
	result := make([]any, 0, len(n.Values))
	for _, val := range n.Values {
		value, err := d.traverseNode(val)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

// handleAnchor processes an anchor node, storing its value for later alias resolution.
func (d *yamlDecoder) handleAnchor(n *ast.AnchorNode) (any, error) {
	value, err := d.traverseNode(n.Value)
	if err != nil {
		return nil, err
	}

	anchorName := n.Name.GetToken().Value
	d.anchors[anchorName] = value
	return value, nil
}

// handleAlias resolves an alias reference to a previously defined anchor.
func (d *yamlDecoder) handleAlias(n *ast.AliasNode) (any, error) {
	aliasName := n.Value.GetToken().Value
	value, ok := d.anchors[aliasName]
	if !ok {
		return nil, fmt.Errorf("alias *%s references undefined anchor", aliasName)
	}
	return value, nil
}
