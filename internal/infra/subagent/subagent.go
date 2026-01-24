package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mcpcodec"
)

const (
	defaultSessionTTL     = 30 * time.Minute
	defaultMaxCacheSize   = 10000
	defaultMaxToolsReturn = 50
)

// EinoSubAgent filters and proxies tool calls using an LLM.
type EinoSubAgent struct {
	config       domain.SubAgentConfig
	model        model.ToolCallingChatModel
	cache        *domain.SessionCache
	controlPlane controlPlaneProvider
	metrics      domain.Metrics
	logger       *zap.Logger
}

// controlPlaneProvider provides access to client-scoped tool snapshots.
type controlPlaneProvider interface {
	GetToolSnapshotForClient(client string) (domain.ToolSnapshot, error)
}

// NewEinoSubAgent creates a new SubAgent instance.
func NewEinoSubAgent(
	ctx context.Context,
	config domain.SubAgentConfig,
	controlPlane controlPlaneProvider,
	metrics domain.Metrics,
	logger *zap.Logger,
) (*EinoSubAgent, error) {
	// Initialize LLM model based on config
	chatModel, err := initializeModel(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("initialize model: %w", err)
	}

	return &EinoSubAgent{
		config:       config,
		model:        chatModel,
		cache:        domain.NewSessionCache(defaultSessionTTL, defaultMaxCacheSize),
		controlPlane: controlPlane,
		metrics:      metrics,
		logger:       logger.Named("subagent"),
	}, nil
}

// SelectToolsForClient filters tools based on the query using LLM reasoning
// and applies deduplication based on session cache.
func (s *EinoSubAgent) SelectToolsForClient(
	ctx context.Context,
	clientID string,
	params domain.AutomaticMCPParams,
) (domain.AutomaticMCPResult, error) {
	snapshot, err := s.controlPlane.GetToolSnapshotForClient(clientID)
	if err != nil {
		return domain.AutomaticMCPResult{}, fmt.Errorf("get tool snapshot: %w", err)
	}

	if len(snapshot.Tools) == 0 {
		return domain.AutomaticMCPResult{ETag: snapshot.ETag}, nil
	}

	// Build tool summaries and hash map
	summaries, hashMap := s.buildToolSummaries(snapshot.Tools)

	// Use LLM to select relevant tools (if query provided)
	var selectedNames []string
	if params.Query != "" {
		selectedNames, err = s.filterWithLLM(ctx, params.Query, summaries)
		if err != nil {
			// Fallback: return all tools if LLM fails
			s.logger.Warn("LLM filtering failed, returning all tools", zap.Error(err))
			selectedNames = allToolNames(summaries)
		}
	} else {
		// No query - return all tools
		selectedNames = allToolNames(summaries)
	}

	// Apply max tools limit
	maxTools := s.config.MaxToolsPerRequest
	if maxTools <= 0 {
		maxTools = defaultMaxToolsReturn
	}
	if len(selectedNames) > maxTools {
		selectedNames = selectedNames[:maxTools]
	}

	sessionKey := domain.AutomaticMCPSessionKey(clientID, params.SessionID)

	shouldSend := make(map[string]bool, len(selectedNames))
	for _, name := range selectedNames {
		hash, ok := hashMap[name]
		if !ok {
			shouldSend[name] = true
			continue
		}
		shouldSend[name] = params.ForceRefresh || s.cache.NeedsFull(sessionKey, name, hash)
	}

	toolsToSend, sentSchemas := s.buildToolPayloads(snapshot.Tools, selectedNames, hashMap, shouldSend)
	s.cache.Update(sessionKey, sentSchemas)
	s.observeFilterPrecision(selectedNames, toolsToSend)

	return domain.AutomaticMCPResult{
		ETag:           snapshot.ETag,
		Tools:          toolsToSend,
		TotalAvailable: len(snapshot.Tools),
		Filtered:       len(toolsToSend),
	}, nil
}

// InvalidateSession clears the session cache for a client.
func (s *EinoSubAgent) InvalidateSession(clientID string) {
	s.cache.Invalidate(clientID)
}

// Close shuts down the SubAgent and releases resources.
func (s *EinoSubAgent) Close() error {
	// Clean up any resources
	return nil
}

// toolSummary contains minimal info for LLM filtering.
type toolSummary struct {
	Name        string
	Description string
	ParamCount  int
}

// buildToolSummaries creates summaries for LLM and computes schema hashes.
func (s *EinoSubAgent) buildToolSummaries(tools []domain.ToolDefinition) ([]toolSummary, map[string]string) {
	summaries := make([]toolSummary, 0, len(tools))
	hashMap := make(map[string]string, len(tools))

	for _, t := range tools {
		summaries = append(summaries, toolSummary{
			Name:        t.Name,
			Description: t.Description,
			ParamCount:  countSchemaProperties(t.InputSchema),
		})

		hash, err := mcpcodec.HashToolDefinition(t)
		if err != nil {
			s.logger.Warn("tool hash failed", zap.String("tool", t.Name), zap.Error(err))
			continue
		}
		hashMap[t.Name] = hash
	}

	return summaries, hashMap
}

// filterWithLLM uses the LLM to select relevant tools for the query.
func (s *EinoSubAgent) filterWithLLM(
	ctx context.Context,
	query string,
	summaries []toolSummary,
) ([]string, error) {
	if len(summaries) == 0 {
		return nil, nil
	}

	// Build prompt for tool selection
	prompt := s.buildFilterPrompt(query, summaries)

	messages := []*schema.Message{
		schema.SystemMessage(defaultFilterSystemPrompt),
		schema.UserMessage(prompt),
	}

	started := time.Now()
	response, err := s.model.Generate(ctx, messages)
	s.metrics.ObserveSubAgentLatency(s.config.Provider, s.config.Model, time.Since(started))
	if err != nil {
		return nil, fmt.Errorf("LLM generate: %w", err)
	}
	s.observeTokenUsage(response)

	// Parse response to extract tool names
	return s.parseSelectedTools(response.Content, summaries)
}

// buildFilterPrompt creates the prompt for tool selection.
func (s *EinoSubAgent) buildFilterPrompt(query string, summaries []toolSummary) string {
	var sb strings.Builder
	sb.WriteString("User task: ")
	sb.WriteString(query)
	sb.WriteString("\n\nAvailable tools:\n")

	for _, t := range summaries {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
	}

	sb.WriteString("\nSelect only the tools that are directly relevant to completing this task.\n")
	sb.WriteString("Return only a JSON array of tool names. Do not include any other text.")
	return sb.String()
}

// parseSelectedTools extracts tool names from LLM response.
func (s *EinoSubAgent) parseSelectedTools(response string, summaries []toolSummary) ([]string, error) {
	// Build a map of valid tool names for validation
	validNames := make(map[string]bool, len(summaries))
	for _, t := range summaries {
		validNames[t.Name] = true
	}

	var jsonNames []string
	if err := json.Unmarshal([]byte(response), &jsonNames); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	result := make([]string, 0, len(jsonNames))
	invalid := make([]string, 0)
	for _, name := range jsonNames {
		if validNames[name] {
			result = append(result, name)
			continue
		}
		invalid = append(invalid, name)
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf("invalid tool names: %s", strings.Join(invalid, ", "))
	}

	return result, nil
}

func (s *EinoSubAgent) observeTokenUsage(response *schema.Message) {
	if response == nil || response.ResponseMeta == nil || response.ResponseMeta.Usage == nil {
		return
	}
	tokens := response.ResponseMeta.Usage.TotalTokens
	if tokens <= 0 {
		return
	}
	s.metrics.ObserveSubAgentTokens(s.config.Provider, s.config.Model, tokens)
}

func (s *EinoSubAgent) observeFilterPrecision(selectedNames []string, toolsToSend []domain.ToolDefinition) {
	precision := 0.0
	if len(selectedNames) > 0 {
		precision = float64(len(toolsToSend)) / float64(len(selectedNames))
	}
	s.metrics.ObserveSubAgentFilterPrecision(s.config.Provider, s.config.Model, precision)
}

// buildToolPayloads builds the tool payload list with deduplication applied.
func (s *EinoSubAgent) buildToolPayloads(
	tools []domain.ToolDefinition,
	selectedNames []string,
	hashMap map[string]string,
	shouldSend map[string]bool,
) ([]domain.ToolDefinition, map[string]string) {
	// Build lookup for tools
	toolMap := make(map[string]domain.ToolDefinition, len(tools))
	for _, t := range tools {
		toolMap[t.Name] = t
	}

	result := make([]domain.ToolDefinition, 0, len(selectedNames))
	sentSchemas := make(map[string]string)
	for _, name := range selectedNames {
		t, ok := toolMap[name]
		if !ok {
			continue
		}
		if !shouldSend[name] {
			continue
		}

		result = append(result, domain.CloneToolDefinition(t))
		if hash, ok := hashMap[name]; ok {
			sentSchemas[name] = hash
		}
	}

	return result, sentSchemas
}

func countSchemaProperties(schema any) int {
	obj, ok := schema.(map[string]any)
	if !ok || obj == nil {
		return 0
	}
	props, ok := obj["properties"].(map[string]any)
	if !ok {
		return 0
	}
	return len(props)
}

// allToolNames extracts all tool names from summaries.
func allToolNames(summaries []toolSummary) []string {
	names := make([]string, len(summaries))
	for i, t := range summaries {
		names[i] = t.Name
	}
	return names
}

const defaultFilterSystemPrompt = `You are a tool selection assistant. Given a user task and a list of available tools, select only the tools that are relevant to completing the task.

Output only a JSON array of tool names. Do not include any extra text or formatting.
Example: ["tool1", "tool2"]

Be selective - only include tools that are directly useful for the given task. Do not include tools that are only tangentially related.`
