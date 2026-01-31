package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PluginCategory defines the governance category for a plugin.
type PluginCategory string

const (
	PluginCategoryObservability  PluginCategory = "observability"
	PluginCategoryAuthentication PluginCategory = "authentication"
	PluginCategoryAuthorization  PluginCategory = "authorization"
	PluginCategoryRateLimiting   PluginCategory = "rate_limiting"
	PluginCategoryValidation     PluginCategory = "validation"
	PluginCategoryContent        PluginCategory = "content"
	PluginCategoryAudit          PluginCategory = "audit"
)

// PluginFlow indicates whether a plugin participates in request or response flow.
type PluginFlow string

const (
	PluginFlowRequest  PluginFlow = "request"
	PluginFlowResponse PluginFlow = "response"
)

// PluginSpec defines a governance plugin process.
type PluginSpec struct {
	Name       string            `json:"name"`
	Category   PluginCategory    `json:"category"`
	Required   bool              `json:"required"`
	Cmd        []string          `json:"cmd"`
	Env        map[string]string `json:"env,omitempty"`
	Cwd        string            `json:"cwd,omitempty"`
	CommitHash string            `json:"commitHash,omitempty"`
	TimeoutMs  int               `json:"timeoutMs"`
	ConfigJSON json.RawMessage   `json:"configJson,omitempty"`
	Flows      []PluginFlow      `json:"flows,omitempty"`
}

// GovernanceRequest describes a single MCP request/response in the pipeline.
type GovernanceRequest struct {
	Flow         PluginFlow
	Method       string
	Caller       string
	Server       string
	ToolName     string
	ResourceURI  string
	PromptName   string
	RoutingKey   string
	RequestJSON  json.RawMessage
	ResponseJSON json.RawMessage
	Metadata     map[string]string
}

// GovernanceDecision captures plugin decisions and optional mutations.
type GovernanceDecision struct {
	Category      PluginCategory
	Plugin        string
	Continue      bool
	RequestJSON   json.RawMessage
	ResponseJSON  json.RawMessage
	RejectCode    string
	RejectMessage string
}

// GovernanceRejection represents a plugin rejection with MCP-facing details.
type GovernanceRejection struct {
	Category PluginCategory
	Plugin   string
	Code     string
	Message  string
}

func (g GovernanceRejection) Error() string {
	msg := strings.TrimSpace(g.Message)
	if msg == "" {
		msg = "request rejected"
	}
	if g.Plugin == "" {
		return fmt.Sprintf("%s: %s", g.Category, msg)
	}
	return fmt.Sprintf("%s/%s: %s", g.Category, g.Plugin, msg)
}

// NormalizePluginCategory ensures a category is valid and normalized.
func NormalizePluginCategory(raw string) (PluginCategory, bool) {
	value := PluginCategory(strings.ToLower(strings.TrimSpace(raw)))
	switch value {
	case PluginCategoryObservability,
		PluginCategoryAuthentication,
		PluginCategoryAuthorization,
		PluginCategoryRateLimiting,
		PluginCategoryValidation,
		PluginCategoryContent,
		PluginCategoryAudit:
		return value, true
	default:
		return "", false
	}
}

// NormalizePluginFlows normalizes plugin flows; empty means both request and response.
func NormalizePluginFlows(raw []string) ([]PluginFlow, bool) {
	if len(raw) == 0 {
		return []PluginFlow{PluginFlowRequest, PluginFlowResponse}, true
	}
	seen := map[PluginFlow]struct{}{}
	out := make([]PluginFlow, 0, len(raw))
	for _, entry := range raw {
		flow := PluginFlow(strings.ToLower(strings.TrimSpace(entry)))
		if flow != PluginFlowRequest && flow != PluginFlowResponse {
			return nil, false
		}
		if _, ok := seen[flow]; ok {
			continue
		}
		seen[flow] = struct{}{}
		out = append(out, flow)
	}
	if len(out) == 0 {
		return []PluginFlow{PluginFlowRequest, PluginFlowResponse}, true
	}
	return out, true
}
