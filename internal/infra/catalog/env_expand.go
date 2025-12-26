package catalog

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func expandConfigEnv(raw []byte) (string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return "", fmt.Errorf("parse config: %w", err)
	}

	expandNode(&root)

	expanded, err := yaml.Marshal(&root)
	if err != nil {
		return "", fmt.Errorf("encode expanded config: %w", err)
	}
	return string(expanded), nil
}

func expandNode(node *yaml.Node) {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			expandNode(child)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			expandNode(node.Content[i+1])
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			expandNode(child)
		}
	case yaml.ScalarNode:
		expandScalar(node)
	}
}

func expandScalar(node *yaml.Node) {
	if node.Tag != "" && node.Tag != "!!str" {
		return
	}
	if !strings.Contains(node.Value, "$") {
		return
	}

	expanded := os.ExpandEnv(node.Value)
	if expanded == node.Value {
		return
	}

	if node.Style != 0 {
		node.Tag = "!!str"
		node.Value = expanded
		return
	}

	tag, value := coerceExpandedScalar(expanded)
	node.Tag = tag
	node.Value = value
}

func coerceExpandedScalar(value string) (string, string) {
	if strings.TrimSpace(value) == "" {
		return "!!str", value
	}

	var parsed any
	if err := yaml.Unmarshal([]byte(value), &parsed); err != nil {
		return "!!str", value
	}

	switch v := parsed.(type) {
	case nil:
		return "!!null", "null"
	case bool:
		return "!!bool", strconv.FormatBool(v)
	case int:
		return "!!int", strconv.Itoa(v)
	case int64:
		return "!!int", strconv.FormatInt(v, 10)
	case uint64:
		return "!!int", strconv.FormatUint(v, 10)
	case float64:
		return "!!float", strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return "!!str", value
	default:
		return "!!str", value
	}
}
