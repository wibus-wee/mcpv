package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"mcpv/internal/infra/fsutil"
)

// RuntimeUpdate describes a serialized runtime config change.
type RuntimeUpdate struct {
	Path string
	Data []byte
}

// RuntimeConfigUpdate holds full runtime settings for updates.
type RuntimeConfigUpdate struct {
	RouteTimeoutSeconds        int
	PingIntervalSeconds        int
	ToolRefreshSeconds         int
	ToolRefreshConcurrency     int
	ClientCheckSeconds         int
	ClientInactiveSeconds      int
	ServerInitRetryBaseSeconds int
	ServerInitRetryMaxSeconds  int
	ServerInitMaxRetries       int
	ReloadMode                 string
	BootstrapMode              string
	BootstrapConcurrency       int
	BootstrapTimeoutSeconds    int
	DefaultActivationMode      string
	ExposeTools                bool
	ToolNamespaceStrategy      string
}

// SubAgentConfigUpdate holds partial SubAgent configuration updates.
type SubAgentConfigUpdate struct {
	EnabledTags        *[]string
	Model              *string
	Provider           *string
	APIKey             *string
	APIKeyEnvVar       *string
	BaseURL            *string
	MaxToolsPerRequest *int
	FilterPrompt       *string
}

// ResolveRuntimePath returns the runtime config path inside a profile store.
func ResolveRuntimePath(storePath string, allowCreate bool) (string, error) {
	if storePath == "" {
		return "", errors.New("profile store path is required")
	}

	runtimePath := filepath.Join(storePath, runtimeFileName)
	altPath := filepath.Join(storePath, runtimeFileAlt)

	yamlExists, err := fileExists(runtimePath)
	if err != nil {
		return "", err
	}
	ymlExists, err := fileExists(altPath)
	if err != nil {
		return "", err
	}

	if yamlExists && ymlExists {
		return "", fmt.Errorf("runtime config has both %s and %s", runtimeFileName, runtimeFileAlt)
	}
	if yamlExists {
		return runtimePath, nil
	}
	if ymlExists {
		return altPath, nil
	}
	if allowCreate {
		return runtimePath, nil
	}
	return "", fmt.Errorf("runtime config not found in %s", storePath)
}

// UpdateRuntimeConfig overwrites all runtime settings; zero values are treated as explicit values.
func UpdateRuntimeConfig(path string, update RuntimeConfigUpdate) (RuntimeUpdate, error) {
	if path == "" {
		return RuntimeUpdate{}, errors.New("runtime config path is required")
	}

	doc, err := loadRuntimeDocument(path)
	if err != nil {
		return RuntimeUpdate{}, err
	}

	doc["routeTimeoutSeconds"] = update.RouteTimeoutSeconds
	doc["pingIntervalSeconds"] = update.PingIntervalSeconds
	doc["toolRefreshSeconds"] = update.ToolRefreshSeconds
	doc["toolRefreshConcurrency"] = update.ToolRefreshConcurrency
	doc["clientCheckSeconds"] = update.ClientCheckSeconds
	doc["clientInactiveSeconds"] = update.ClientInactiveSeconds
	doc["serverInitRetryBaseSeconds"] = update.ServerInitRetryBaseSeconds
	doc["serverInitRetryMaxSeconds"] = update.ServerInitRetryMaxSeconds
	doc["serverInitMaxRetries"] = update.ServerInitMaxRetries
	doc["reloadMode"] = strings.TrimSpace(update.ReloadMode)
	doc["bootstrapMode"] = strings.TrimSpace(update.BootstrapMode)
	doc["bootstrapConcurrency"] = update.BootstrapConcurrency
	doc["bootstrapTimeoutSeconds"] = update.BootstrapTimeoutSeconds
	doc["defaultActivationMode"] = strings.TrimSpace(update.DefaultActivationMode)
	doc["exposeTools"] = update.ExposeTools
	doc["toolNamespaceStrategy"] = strings.TrimSpace(update.ToolNamespaceStrategy)

	merged, err := yaml.Marshal(doc)
	if err != nil {
		return RuntimeUpdate{}, fmt.Errorf("render runtime config: %w", err)
	}

	return RuntimeUpdate{Path: path, Data: merged}, nil
}

// UpdateSubAgentConfig applies SubAgent-specific updates to the runtime config.
func UpdateSubAgentConfig(path string, update SubAgentConfigUpdate) (RuntimeUpdate, error) {
	if path == "" {
		return RuntimeUpdate{}, errors.New("runtime config path is required")
	}

	doc, err := loadRuntimeDocument(path)
	if err != nil {
		return RuntimeUpdate{}, err
	}

	subAgent, ok := doc["subAgent"].(map[string]any)
	if !ok || subAgent == nil {
		subAgent = make(map[string]any)
	}

	if update.Model != nil {
		subAgent["model"] = strings.TrimSpace(*update.Model)
	}
	if update.Provider != nil {
		subAgent["provider"] = strings.TrimSpace(*update.Provider)
	}
	if update.EnabledTags != nil {
		subAgent["enabledTags"] = append([]string(nil), (*update.EnabledTags)...)
	}
	if update.APIKey != nil {
		subAgent["apiKey"] = strings.TrimSpace(*update.APIKey)
	}
	if update.APIKeyEnvVar != nil {
		subAgent["apiKeyEnvVar"] = strings.TrimSpace(*update.APIKeyEnvVar)
	}
	if update.BaseURL != nil {
		subAgent["baseURL"] = strings.TrimSpace(*update.BaseURL)
	}
	if update.MaxToolsPerRequest != nil {
		subAgent["maxToolsPerRequest"] = *update.MaxToolsPerRequest
	}
	if update.FilterPrompt != nil {
		subAgent["filterPrompt"] = strings.TrimSpace(*update.FilterPrompt)
	}

	doc["subAgent"] = subAgent

	merged, err := yaml.Marshal(doc)
	if err != nil {
		return RuntimeUpdate{}, fmt.Errorf("render runtime config: %w", err)
	}

	return RuntimeUpdate{Path: path, Data: merged}, nil
}

func loadRuntimeDocument(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("read runtime config: %w", err)
	}

	doc := make(map[string]any)
	if len(data) == 0 {
		return doc, nil
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse runtime config: %w", err)
	}
	return doc, nil
}

func writeRuntimeUpdate(update RuntimeUpdate) error {
	if err := os.MkdirAll(filepath.Dir(update.Path), fsutil.DefaultDirMode); err != nil {
		return fmt.Errorf("create runtime config directory: %w", err)
	}
	if err := os.WriteFile(update.Path, update.Data, fsutil.DefaultFileMode); err != nil {
		return fmt.Errorf("write runtime config: %w", err)
	}
	return nil
}
