package sampling

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

// Handler processes sampling/createMessage requests.
type Handler struct {
	model  model.ToolCallingChatModel
	logger *zap.Logger
}

// NewHandler builds a sampling handler using SubAgent configuration.
func NewHandler(ctx context.Context, config domain.SubAgentConfig, logger *zap.Logger) (*Handler, error) {
	chatModel, err := initializeModel(ctx, config)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		model:  chatModel,
		logger: logger.Named("sampling"),
	}, nil
}

// CreateMessage generates a sampling response from the configured model.
func (h *Handler) CreateMessage(ctx context.Context, params *domain.SamplingRequest) (*domain.SamplingResult, error) {
	if params == nil {
		return nil, fmt.Errorf("sampling params are required")
	}
	messages, err := toMessages(params)
	if err != nil {
		return nil, err
	}
	response, err := h.model.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("sampling generate: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("sampling response is nil")
	}
	result := &domain.SamplingResult{
		Role: "assistant",
		Content: domain.SamplingContent{
			Type: "text",
			Text: response.Content,
		},
	}
	if response.ResponseMeta != nil {
		result.StopReason = response.ResponseMeta.FinishReason
	}
	return result, nil
}

func toMessages(params *domain.SamplingRequest) ([]*schema.Message, error) {
	messages := make([]*schema.Message, 0, len(params.Messages)+1)
	if strings.TrimSpace(params.SystemPrompt) != "" {
		messages = append(messages, schema.SystemMessage(params.SystemPrompt))
	}
	for _, msg := range params.Messages {
		contentType := strings.TrimSpace(msg.Content.Type)
		if contentType == "" {
			contentType = "text"
		}
		if contentType != "text" {
			return nil, fmt.Errorf("unsupported content type: %s", contentType)
		}
		text := msg.Content.Text
		switch strings.TrimSpace(msg.Role) {
		case "user":
			messages = append(messages, schema.UserMessage(text))
		case "assistant":
			messages = append(messages, schema.AssistantMessage(text, nil))
		case "system":
			messages = append(messages, schema.SystemMessage(text))
		default:
			return nil, fmt.Errorf("unsupported role: %s", msg.Role)
		}
	}
	return messages, nil
}

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
