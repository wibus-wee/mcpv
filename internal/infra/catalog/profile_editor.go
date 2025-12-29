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
	Name                string            `yaml:"name"`
	Cmd                 []string          `yaml:"cmd"`
	Env                 map[string]string `yaml:"env,omitempty"`
	Cwd                 string            `yaml:"cwd,omitempty"`
	IdleSeconds         int               `yaml:"idleSeconds"`
	MaxConcurrent       int               `yaml:"maxConcurrent"`
	Sticky              bool              `yaml:"sticky"`
	Persistent          bool              `yaml:"persistent"`
	MinReady            int               `yaml:"minReady"`
	DrainTimeoutSeconds int               `yaml:"drainTimeoutSeconds"`
	ProtocolVersion     string            `yaml:"protocolVersion"`
	ExposeTools         []string          `yaml:"exposeTools,omitempty"`
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

	data, err := os.ReadFile(path)
	if err != nil {
		return ProfileUpdate{}, fmt.Errorf("read profile file: %w", err)
	}

	doc := make(map[string]any)
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return ProfileUpdate{}, fmt.Errorf("parse profile file: %w", err)
		}
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

	merged, err := yaml.Marshal(doc)
	if err != nil {
		return ProfileUpdate{}, fmt.Errorf("render profile file: %w", err)
	}

	return ProfileUpdate{
		Path: path,
		Data: merged,
	}, nil
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

	return serverSpecYAML{
		Name:                spec.Name,
		Cmd:                 spec.Cmd,
		Env:                 env,
		Cwd:                 spec.Cwd,
		IdleSeconds:         spec.IdleSeconds,
		MaxConcurrent:       spec.MaxConcurrent,
		Sticky:              spec.Sticky,
		Persistent:          spec.Persistent,
		MinReady:            spec.MinReady,
		DrainTimeoutSeconds: spec.DrainTimeoutSeconds,
		ProtocolVersion:     spec.ProtocolVersion,
		ExposeTools:         exposeTools,
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
