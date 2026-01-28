package catalog

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func expandConfigEnv(raw []byte) (string, []string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return "", nil, fmt.Errorf("parse config: %w", err)
	}

	missing := make(map[string]struct{})
	expandNode(&root, missing)

	expanded, err := yaml.Marshal(&root)
	if err != nil {
		return "", nil, fmt.Errorf("encode expanded config: %w", err)
	}
	return string(expanded), missingList(missing), nil
}

func expandNode(node *yaml.Node, missing map[string]struct{}) {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			expandNode(child, missing)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			expandNode(node.Content[i+1], missing)
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			expandNode(child, missing)
		}
	case yaml.AliasNode:
		if node.Alias != nil {
			expandNode(node.Alias, missing)
		}
	case yaml.ScalarNode:
		expandScalar(node, missing)
	}
}

func expandScalar(node *yaml.Node, missing map[string]struct{}) {
	if node.Tag != "" && node.Tag != "!!str" {
		return
	}
	if !strings.Contains(node.Value, "$") {
		return
	}

	expanded := expandEnvWithTracking(node.Value, missing)
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

func expandEnvWithTracking(value string, missing map[string]struct{}) string {
	return os.Expand(value, func(key string) string {
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		missing[key] = struct{}{}
		return ""
	})
}

func missingList(missing map[string]struct{}) []string {
	if len(missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
