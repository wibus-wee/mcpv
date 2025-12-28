package ui

import "encoding/json"

// Frontend-friendly types for Wails bindings

// ToolEntry represents a single tool for the frontend
type ToolEntry struct {
	Name     string          `json:"name"`
	ToolJSON json.RawMessage `json:"toolJson"`
}

// ResourceEntry represents a single resource for the frontend
type ResourceEntry struct {
	URI          string          `json:"uri"`
	ResourceJSON json.RawMessage `json:"resourceJson"`
}

// PromptEntry represents a single prompt for the frontend
type PromptEntry struct {
	Name       string          `json:"name"`
	PromptJSON json.RawMessage `json:"promptJson"`
}

// ResourcePage represents a paginated list of resources
type ResourcePage struct {
	Resources  []ResourceEntry `json:"resources"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

// PromptPage represents a paginated list of prompts
type PromptPage struct {
	Prompts    []PromptEntry `json:"prompts"`
	NextCursor string        `json:"nextCursor,omitempty"`
}
