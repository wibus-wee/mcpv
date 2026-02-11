package editor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"mcpv/internal/domain"
)

type ProfileUpdate struct {
	Path string
	Data []byte
}

var ErrServerExists = errors.New("server already exists")
var ErrServerNotFound = errors.New("server not found")
var ErrPluginExists = errors.New("plugin already exists")
var ErrPluginNotFound = errors.New("plugin not found")

type serverSpecYAML struct {
	Name                string              `yaml:"name"`
	Transport           string              `yaml:"transport,omitempty"`
	Cmd                 []string            `yaml:"cmd"`
	Env                 map[string]string   `yaml:"env,omitempty"`
	Cwd                 string              `yaml:"cwd,omitempty"`
	Tags                []string            `yaml:"tags,omitempty"`
	IdleSeconds         int                 `yaml:"idleSeconds"`
	MaxConcurrent       int                 `yaml:"maxConcurrent"`
	Strategy            string              `yaml:"strategy,omitempty"`
	SessionTTLSeconds   int                 `yaml:"sessionTTLSeconds,omitempty"`
	Disabled            bool                `yaml:"disabled,omitempty"`
	MinReady            int                 `yaml:"minReady"`
	ActivationMode      string              `yaml:"activationMode,omitempty"`
	DrainTimeoutSeconds int                 `yaml:"drainTimeoutSeconds"`
	ProtocolVersion     string              `yaml:"protocolVersion"`
	ExposeTools         []string            `yaml:"exposeTools,omitempty"`
	HTTP                *streamableHTTPYAML `yaml:"http,omitempty"`
}

type streamableHTTPYAML struct {
	Endpoint   string            `yaml:"endpoint"`
	Headers    map[string]string `yaml:"headers,omitempty"`
	MaxRetries int               `yaml:"maxRetries,omitempty"`
}

type pluginSpecYAML struct {
	Name               string            `yaml:"name"`
	Category           string            `yaml:"category"`
	Required           bool              `yaml:"required"`
	Disabled           bool              `yaml:"disabled,omitempty"`
	Cmd                []string          `yaml:"cmd"`
	Env                map[string]string `yaml:"env,omitempty"`
	Cwd                string            `yaml:"cwd,omitempty"`
	CommitHash         string            `yaml:"commitHash,omitempty"`
	TimeoutMs          int               `yaml:"timeoutMs,omitempty"`
	HandshakeTimeoutMs int               `yaml:"handshakeTimeoutMs,omitempty"`
	Flows              []string          `yaml:"flows,omitempty"`
	Config             map[string]any    `yaml:"config,omitempty"`
}

func BuildProfileUpdate(path string, servers []domain.ServerSpec) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	existing, err := parseServersFromDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	updated, err := mergeServers(existing, servers)
	if err != nil {
		return ProfileUpdate{}, err
	}

	doc["servers"] = updated

	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{
		Path: path,
		Data: merged,
	}, nil
}

func CreateServer(path string, server domain.ServerSpec) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}
	serverName := strings.TrimSpace(server.Name)
	if serverName == "" {
		return ProfileUpdate{}, errors.New("server name is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	servers, err := parseServersFromDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	for _, existing := range servers {
		if strings.TrimSpace(existing.Name) == serverName {
			return ProfileUpdate{}, fmt.Errorf("%w: %s", ErrServerExists, serverName)
		}
	}

	servers = append(servers, toServerSpecYAML(server))
	doc["servers"] = servers

	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{Path: path, Data: merged}, nil
}

func UpdateServer(path string, server domain.ServerSpec) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}
	serverName := strings.TrimSpace(server.Name)
	if serverName == "" {
		return ProfileUpdate{}, errors.New("server name is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	servers, err := parseServersFromDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	found := false
	for i := range servers {
		if strings.TrimSpace(servers[i].Name) == serverName {
			servers[i] = toServerSpecYAML(server)
			found = true
			break
		}
	}
	if !found {
		return ProfileUpdate{}, fmt.Errorf("%w: %s", ErrServerNotFound, serverName)
	}

	doc["servers"] = servers
	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{Path: path, Data: merged}, nil
}

func SetServerDisabled(path string, serverName string, disabled bool) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return ProfileUpdate{}, errors.New("server name is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	servers, err := parseServersFromDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	found := false
	for i := range servers {
		if strings.TrimSpace(servers[i].Name) == serverName {
			servers[i].Disabled = disabled
			found = true
			break
		}
	}
	if !found {
		return ProfileUpdate{}, fmt.Errorf("%w: %s", ErrServerNotFound, serverName)
	}

	doc["servers"] = servers
	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{Path: path, Data: merged}, nil
}

func DeleteServer(path string, serverName string) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return ProfileUpdate{}, errors.New("server name is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	servers, err := parseServersFromDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	updated := make([]serverSpecYAML, 0, len(servers))
	found := false
	for _, server := range servers {
		if strings.TrimSpace(server.Name) == serverName {
			found = true
			continue
		}
		updated = append(updated, server)
	}
	if !found {
		return ProfileUpdate{}, fmt.Errorf("%w: %s", ErrServerNotFound, serverName)
	}

	doc["servers"] = updated
	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{Path: path, Data: merged}, nil
}

func CreatePlugin(path string, plugin domain.PluginSpec) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}
	pluginName := strings.TrimSpace(plugin.Name)
	if pluginName == "" {
		return ProfileUpdate{}, errors.New("plugin name is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	plugins, err := parsePluginsFromDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	for _, existing := range plugins {
		if strings.TrimSpace(existing.Name) == pluginName {
			return ProfileUpdate{}, fmt.Errorf("%w: %s", ErrPluginExists, pluginName)
		}
	}

	plugins = append(plugins, toPluginSpecYAML(plugin))
	doc["plugins"] = plugins

	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{Path: path, Data: merged}, nil
}

func UpdatePlugin(path string, plugin domain.PluginSpec) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}
	pluginName := strings.TrimSpace(plugin.Name)
	if pluginName == "" {
		return ProfileUpdate{}, errors.New("plugin name is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	plugins, err := parsePluginsFromDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	found := false
	for i := range plugins {
		if strings.TrimSpace(plugins[i].Name) == pluginName {
			plugins[i] = toPluginSpecYAML(plugin)
			found = true
			break
		}
	}
	if !found {
		return ProfileUpdate{}, fmt.Errorf("%w: %s", ErrPluginNotFound, pluginName)
	}

	doc["plugins"] = plugins
	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{Path: path, Data: merged}, nil
}

func SetPluginDisabled(path string, pluginName string, disabled bool) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}
	pluginName = strings.TrimSpace(pluginName)
	if pluginName == "" {
		return ProfileUpdate{}, errors.New("plugin name is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	plugins, err := parsePluginsFromDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	found := false
	for i := range plugins {
		if strings.TrimSpace(plugins[i].Name) == pluginName {
			plugins[i].Disabled = disabled
			found = true
			break
		}
	}
	if !found {
		return ProfileUpdate{}, fmt.Errorf("%w: %s", ErrPluginNotFound, pluginName)
	}

	doc["plugins"] = plugins
	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{Path: path, Data: merged}, nil
}

func DeletePlugin(path string, pluginName string) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}
	pluginName = strings.TrimSpace(pluginName)
	if pluginName == "" {
		return ProfileUpdate{}, errors.New("plugin name is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	plugins, err := parsePluginsFromDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	updated := make([]pluginSpecYAML, 0, len(plugins))
	found := false
	for _, plugin := range plugins {
		if strings.TrimSpace(plugin.Name) == pluginName {
			found = true
			continue
		}
		updated = append(updated, plugin)
	}
	if !found {
		return ProfileUpdate{}, fmt.Errorf("%w: %s", ErrPluginNotFound, pluginName)
	}

	doc["plugins"] = updated
	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{Path: path, Data: merged}, nil
}

func parseServersFromDocument(doc map[string]any) ([]serverSpecYAML, error) {
	if doc == nil {
		return []serverSpecYAML{}, nil
	}

	raw, ok := doc["servers"]
	if !ok || raw == nil {
		return []serverSpecYAML{}, nil
	}

	encoded, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("decode servers: %w", err)
	}

	var servers []serverSpecYAML
	if err := yaml.Unmarshal(encoded, &servers); err != nil {
		return nil, fmt.Errorf("parse servers: %w", err)
	}

	return servers, nil
}

func parsePluginsFromDocument(doc map[string]any) ([]pluginSpecYAML, error) {
	if doc == nil {
		return []pluginSpecYAML{}, nil
	}

	raw, ok := doc["plugins"]
	if !ok || raw == nil {
		return []pluginSpecYAML{}, nil
	}

	encoded, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("decode plugins: %w", err)
	}

	var plugins []pluginSpecYAML
	if err := yaml.Unmarshal(encoded, &plugins); err != nil {
		return nil, fmt.Errorf("parse plugins: %w", err)
	}

	return plugins, nil
}

func mergeServers(existing []serverSpecYAML, incoming []domain.ServerSpec) ([]serverSpecYAML, error) {
	nameIndex := make(map[string]struct{}, len(existing))
	for _, server := range existing {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			continue
		}
		nameIndex[name] = struct{}{}
	}

	for _, server := range incoming {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			return nil, errors.New("import contains empty server name")
		}
		if _, exists := nameIndex[name]; exists {
			return nil, fmt.Errorf("server %q already exists in profile", name)
		}
		nameIndex[name] = struct{}{}
		existing = append(existing, toServerSpecYAML(server))
	}

	return existing, nil
}

func toServerSpecYAML(spec domain.ServerSpec) serverSpecYAML {
	env := spec.Env
	if len(env) == 0 {
		env = nil
	}
	exposeTools := spec.ExposeTools
	if len(exposeTools) == 0 {
		exposeTools = nil
	}
	var httpCfg *streamableHTTPYAML
	if spec.HTTP != nil && domain.NormalizeTransport(spec.Transport) == domain.TransportStreamableHTTP {
		headers := spec.HTTP.Headers
		if len(headers) == 0 {
			headers = nil
		}
		httpCfg = &streamableHTTPYAML{
			Endpoint:   spec.HTTP.Endpoint,
			Headers:    headers,
			MaxRetries: spec.HTTP.MaxRetries,
		}
	}

	return serverSpecYAML{
		Name:                spec.Name,
		Transport:           string(domain.NormalizeTransport(spec.Transport)),
		Cmd:                 append([]string(nil), spec.Cmd...),
		Env:                 env,
		Cwd:                 spec.Cwd,
		Tags:                append([]string(nil), spec.Tags...),
		IdleSeconds:         spec.IdleSeconds,
		MaxConcurrent:       spec.MaxConcurrent,
		Strategy:            string(spec.Strategy),
		SessionTTLSeconds:   spec.SessionTTLSeconds,
		Disabled:            spec.Disabled,
		MinReady:            spec.MinReady,
		ActivationMode:      string(spec.ActivationMode),
		DrainTimeoutSeconds: spec.DrainTimeoutSeconds,
		ProtocolVersion:     spec.ProtocolVersion,
		ExposeTools:         exposeTools,
		HTTP:                httpCfg,
	}
}

func toPluginSpecYAML(spec domain.PluginSpec) pluginSpecYAML {
	env := spec.Env
	if len(env) == 0 {
		env = nil
	}

	flows := make([]string, 0, len(spec.Flows))
	for _, flow := range spec.Flows {
		flows = append(flows, string(flow))
	}
	if len(flows) == 0 {
		flows = nil
	}

	var config map[string]any
	if len(spec.ConfigJSON) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(spec.ConfigJSON, &parsed); err == nil {
			config = parsed
			if config == nil {
				config = map[string]any{}
			}
		}
	}

	return pluginSpecYAML{
		Name:               spec.Name,
		Category:           string(spec.Category),
		Required:           spec.Required,
		Disabled:           spec.Disabled,
		Cmd:                append([]string(nil), spec.Cmd...),
		Env:                env,
		Cwd:                spec.Cwd,
		CommitHash:         spec.CommitHash,
		TimeoutMs:          spec.TimeoutMs,
		HandshakeTimeoutMs: spec.HandshakeTimeoutMs,
		Flows:              flows,
		Config:             config,
	}
}

func loadProfileDocument(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile file: %w", err)
	}

	doc := make(map[string]any)
	if len(data) == 0 {
		return doc, nil
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse profile file: %w", err)
	}
	return doc, nil
}

func marshalProfileDocument(doc map[string]any) ([]byte, error) {
	merged, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("render profile file: %w", err)
	}
	return merged, nil
}

// SetProfileSubAgentEnabled updates the per-profile SubAgent enabled state.
func SetProfileSubAgentEnabled(path string, enabled bool) (ProfileUpdate, error) {
	if path == "" {
		return ProfileUpdate{}, errors.New("profile path is required")
	}

	doc, err := loadProfileDocument(path)
	if err != nil {
		return ProfileUpdate{}, err
	}

	// Get or create subAgent section
	var subAgentConfig map[string]any
	if raw, ok := doc["subAgent"]; ok && raw != nil {
		if cfg, ok := raw.(map[string]any); ok {
			subAgentConfig = cfg
		} else {
			subAgentConfig = make(map[string]any)
		}
	} else {
		subAgentConfig = make(map[string]any)
	}

	subAgentConfig["enabled"] = enabled
	doc["subAgent"] = subAgentConfig

	merged, err := marshalProfileDocument(doc)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{Path: path, Data: merged}, nil
}
