package transfer

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"mcpv/internal/domain"
)

// ResolvePath returns the config file path for the given source.
func ResolvePath(source Source) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	switch source {
	case SourceClaude:
		return filepath.Join(home, ".claude.json"), nil
	case SourceCodex:
		return filepath.Join(home, ".codex", "config.toml"), nil
	case SourceGemini:
		return filepath.Join(home, ".gemini", "settings.json"), nil
	default:
		return "", ErrUnknownSource
	}
}

// ReadSource reads and parses MCP servers from the source config.
func ReadSource(source Source) (Result, error) {
	path, err := ResolvePath(source)
	if err != nil {
		return Result{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{Source: source, Path: path}, ErrNotFound
		}
		return Result{}, fmt.Errorf("read source: %w", err)
	}
	switch source {
	case SourceClaude, SourceGemini:
		return readJSONSource(source, path, data)
	case SourceCodex:
		return readCodexSource(path, data)
	default:
		return Result{}, ErrUnknownSource
	}
}

func readJSONSource(source Source, path string, data []byte) (Result, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return Result{}, fmt.Errorf("parse json: %w", err)
	}
	raw, ok := payload["mcpServers"].(map[string]any)
	if !ok {
		return Result{}, errors.New("mcpServers must be an object map")
	}
	result := Result{
		Source:  source,
		Path:    path,
		Servers: make([]domain.ServerSpec, 0, len(raw)),
	}
	for name, entry := range raw {
		table, ok := entry.(map[string]any)
		if !ok {
			result.Issues = append(result.Issues, Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "entry must be an object",
			})
			continue
		}
		spec, issue, ok := parseServerSpec(name, table)
		if !ok {
			if issue != nil {
				result.Issues = append(result.Issues, *issue)
			}
			continue
		}
		result.Servers = append(result.Servers, spec)
	}
	return result, nil
}

func readCodexSource(path string, data []byte) (Result, error) {
	var payload map[string]any
	if err := toml.Unmarshal(data, &payload); err != nil {
		return Result{}, fmt.Errorf("parse toml: %w", err)
	}
	result := Result{
		Source: SourceCodex,
		Path:   path,
	}
	primary := readTomlTable(payload, "mcp_servers")
	legacy := readTomlTable(payload, "mcp", "servers")

	seen := make(map[string]struct{})
	result.Servers = make([]domain.ServerSpec, 0, len(primary))

	for name, entry := range primary {
		table, ok := entry.(map[string]any)
		if !ok {
			result.Issues = append(result.Issues, Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "entry must be an object",
			})
			continue
		}
		spec, issue, ok := parseServerSpec(name, table)
		if !ok {
			if issue != nil {
				result.Issues = append(result.Issues, *issue)
			}
			continue
		}
		seen[spec.Name] = struct{}{}
		result.Servers = append(result.Servers, spec)
	}

	for name, entry := range legacy {
		if _, exists := seen[name]; exists {
			result.Issues = append(result.Issues, Issue{
				Name:    name,
				Kind:    IssueDuplicate,
				Message: "legacy mcp.servers entry ignored because mcp_servers already defines it",
			})
			continue
		}
		table, ok := entry.(map[string]any)
		if !ok {
			result.Issues = append(result.Issues, Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "entry must be an object",
			})
			continue
		}
		spec, issue, ok := parseServerSpec(name, table)
		if !ok {
			if issue != nil {
				result.Issues = append(result.Issues, *issue)
			}
			continue
		}
		seen[spec.Name] = struct{}{}
		result.Servers = append(result.Servers, spec)
	}

	return result, nil
}

func readTomlTable(payload map[string]any, path ...string) map[string]any {
	current := payload
	for i, key := range path {
		value, ok := current[key]
		if !ok {
			return nil
		}
		if i == len(path)-1 {
			table, ok := value.(map[string]any)
			if !ok {
				return nil
			}
			return table
		}
		next, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		current = next
	}
	return nil
}

func parseServerSpec(name string, entry map[string]any) (domain.ServerSpec, *Issue, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.ServerSpec{}, &Issue{
			Kind:    IssueInvalid,
			Message: "server name is required",
		}, false
	}
	endpoint, ok := readOptionalString(entry, "endpoint")
	if !ok {
		return domain.ServerSpec{}, &Issue{
			Name:    name,
			Kind:    IssueInvalid,
			Message: "endpoint must be a string",
		}, false
	}
	if endpoint == "" {
		endpoint, ok = readOptionalString(entry, "url")
		if !ok {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "url must be a string",
			}, false
		}
	}
	transportRaw, ok := readOptionalString(entry, "transport")
	if !ok {
		return domain.ServerSpec{}, &Issue{
			Name:    name,
			Kind:    IssueInvalid,
			Message: "transport must be a string",
		}, false
	}
	if transportRaw == "" {
		transportRaw, ok = readOptionalString(entry, "type")
		if !ok {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "type must be a string",
			}, false
		}
	}
	transport, ok := normalizeTransport(transportRaw, endpoint != "")
	if !ok {
		return domain.ServerSpec{}, &Issue{
			Name:    name,
			Kind:    IssueInvalid,
			Message: "unsupported transport type",
		}, false
	}

	spec := domain.ServerSpec{
		Name:      name,
		Transport: transport,
	}

	switch transport {
	case domain.TransportStdio:
		command, ok := readRequiredString(entry, "command")
		if !ok {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "command is required for stdio transport",
			}, false
		}
		args, ok := readOptionalStringSlice(entry, "args")
		if !ok {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "args must be an array of strings",
			}, false
		}
		env, ok := readOptionalStringMap(entry, "env")
		if !ok {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "env must be a map of strings",
			}, false
		}
		cwd, ok := readOptionalString(entry, "cwd")
		if !ok {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "cwd must be a string",
			}, false
		}
		spec.Cmd = append([]string{command}, args...)
		spec.Env = env
		spec.Cwd = cwd
	case domain.TransportStreamableHTTP:
		if endpoint == "" {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "endpoint is required for streamable_http transport",
			}, false
		}
		headers, ok := readHTTPHeaders(entry)
		if !ok {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "headers must be a map of strings",
			}, false
		}
		maxRetries, ok := readOptionalInt(entry, "maxRetries", "max_retry_attempts", "retry_count")
		if !ok {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "maxRetries must be a number",
			}, false
		}
		spec.HTTP = &domain.StreamableHTTPConfig{
			Endpoint:   endpoint,
			Headers:    headers,
			MaxRetries: maxRetries,
		}
	default:
		return domain.ServerSpec{}, &Issue{
			Name:    name,
			Kind:    IssueInvalid,
			Message: "unsupported transport type",
		}, false
	}

	protocolVersion, ok := readOptionalString(entry, "protocolVersion")
	if !ok {
		return domain.ServerSpec{}, &Issue{
			Name:    name,
			Kind:    IssueInvalid,
			Message: "protocolVersion must be a string",
		}, false
	}
	if protocolVersion == "" {
		protocolVersion, ok = readOptionalString(entry, "protocol_version")
		if !ok {
			return domain.ServerSpec{}, &Issue{
				Name:    name,
				Kind:    IssueInvalid,
				Message: "protocol_version must be a string",
			}, false
		}
	}
	spec.ProtocolVersion = protocolVersion

	return spec, nil, true
}

func normalizeTransport(raw string, hasEndpoint bool) (domain.TransportKind, bool) {
	if raw == "" {
		if hasEndpoint {
			return domain.TransportStreamableHTTP, true
		}
		return domain.TransportStdio, true
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "stdio":
		return domain.TransportStdio, true
	case "streamable_http", "streamable-http", "streamablehttp", "http", "sse":
		return domain.TransportStreamableHTTP, true
	default:
		return "", false
	}
}

func readRequiredString(entry map[string]any, key string) (string, bool) {
	value, ok := entry[key]
	if !ok {
		return "", false
	}
	s, ok := value.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func readOptionalString(entry map[string]any, key string) (string, bool) {
	value, ok := entry[key]
	if !ok {
		return "", true
	}
	s, ok := value.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(s), true
}

func readOptionalStringSlice(entry map[string]any, key string) ([]string, bool) {
	value, ok := entry[key]
	if !ok {
		return nil, true
	}
	switch raw := value.(type) {
	case []string:
		return append([]string(nil), raw...), true
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func readOptionalStringMap(entry map[string]any, key string) (map[string]string, bool) {
	value, ok := entry[key]
	if !ok {
		return nil, true
	}
	return toStringMap(value)
}

func readHTTPHeaders(entry map[string]any) (map[string]string, bool) {
	if _, exists := entry["http_headers"]; exists {
		return readOptionalStringMap(entry, "http_headers")
	}
	return readOptionalStringMap(entry, "headers")
}

func readOptionalInt(entry map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := entry[key]
		if !ok {
			continue
		}
		return toInt(value)
	}
	return 0, true
}

func toInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		if math.Trunc(float64(v)) != float64(v) {
			return 0, false
		}
		return int(v), true
	case float64:
		if math.Trunc(v) != v {
			return 0, false
		}
		return int(v), true
	default:
		return 0, false
	}
}

func toStringMap(value any) (map[string]string, bool) {
	switch raw := value.(type) {
	case map[string]string:
		out := make(map[string]string, len(raw))
		for k, v := range raw {
			out[k] = v
		}
		return out, true
	case map[string]any:
		out := make(map[string]string, len(raw))
		for k, v := range raw {
			s, ok := v.(string)
			if !ok {
				return nil, false
			}
			out[k] = s
		}
		return out, true
	default:
		return nil, false
	}
}
