package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"mcpd/internal/domain"
)

type ProfileUpdate struct {
	Path string
	Data []byte
}

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

func ResolveProfilePath(storePath string, profileName string) (string, error) {
	if storePath == "" {
		return "", errors.New("profile store path is required")
	}
	if profileName == "" {
		return "", errors.New("profile name is required")
	}

	profilesDir := filepath.Join(storePath, profilesDirName)
	candidateYAML := filepath.Join(profilesDir, profileName+".yaml")
	candidateYML := filepath.Join(profilesDir, profileName+".yml")

	yamlExists, err := fileExists(candidateYAML)
	if err != nil {
		return "", err
	}
	ymlExists, err := fileExists(candidateYML)
	if err != nil {
		return "", err
	}

	if yamlExists && ymlExists {
		return "", fmt.Errorf("profile %q has both .yaml and .yml files", profileName)
	}
	if yamlExists {
		return candidateYAML, nil
	}
	if ymlExists {
		return candidateYML, nil
	}

	return "", fmt.Errorf("profile %q not found in %s", profileName, profilesDir)
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
		return ProfileUpdate{}, fmt.Errorf("server %q not found in profile", serverName)
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
		return ProfileUpdate{}, fmt.Errorf("server %q not found in profile", serverName)
	}

	doc["servers"] = updated
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
		Cmd:                 spec.Cmd,
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

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return !info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
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
