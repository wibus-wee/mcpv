package domain

// Meta carries extension metadata for MCP entities.
type Meta map[string]any

// Role identifies an MCP audience role.
type Role string

// Annotations captures shared annotation fields.
type Annotations struct {
	Audience     []Role  `json:"audience"`
	LastModified string  `json:"lastModified"`
	Priority     float64 `json:"priority"`
}

// ToolAnnotations captures tool-specific annotation fields.
type ToolAnnotations struct {
	DestructiveHint *bool  `json:"destructiveHint"`
	IdempotentHint  bool   `json:"idempotentHint"`
	OpenWorldHint   *bool  `json:"openWorldHint"`
	ReadOnlyHint    bool   `json:"readOnlyHint"`
	Title           string `json:"title"`
}

// PromptArgument describes a prompt argument.
type PromptArgument struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}
