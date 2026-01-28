package subagent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"mcpd/internal/domain"
)

// initializeModel creates the chat model based on configuration.
func initializeModel(ctx context.Context, config domain.SubAgentConfig) (model.ToolCallingChatModel, error) {
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		envVar := strings.TrimSpace(config.APIKeyEnvVar)
		if envVar == "" {
			return nil, fmt.Errorf("API key is required: set subAgent.apiKey or subAgent.apiKeyEnvVar")
		}
		apiKey = os.Getenv(envVar)
		if apiKey == "" {
			return nil, fmt.Errorf("API key not found in env var %s", envVar)
		}
	}

	switch config.Provider {
	case "openai", "":
		cfg := &openai.ChatModelConfig{
			Model:  config.Model,
			APIKey: apiKey,
		}
		if config.BaseURL != "" {
			cfg.BaseURL = config.BaseURL
		}
		return openai.NewChatModel(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}
}
