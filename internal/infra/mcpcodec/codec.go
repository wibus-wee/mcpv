package mcpcodec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcpv/internal/domain"
)

// ToolFromMCP converts an MCP tool to a domain definition.
func ToolFromMCP(tool *mcp.Tool) domain.ToolDefinition {
	if tool == nil {
		return domain.ToolDefinition{}
	}
	return domain.ToolDefinition{
		Name:         tool.Name,
		Description:  tool.Description,
		InputSchema:  domain.CloneJSONValue(tool.InputSchema),
		OutputSchema: domain.CloneJSONValue(tool.OutputSchema),
		Title:        tool.Title,
		Annotations:  toolAnnotationsFromMCP(tool.Annotations),
		Meta:         metaFromMCP(tool.Meta),
	}
}

// ResourceFromMCP converts an MCP resource to a domain definition.
func ResourceFromMCP(resource *mcp.Resource) domain.ResourceDefinition {
	if resource == nil {
		return domain.ResourceDefinition{}
	}
	return domain.ResourceDefinition{
		URI:         resource.URI,
		Name:        resource.Name,
		Title:       resource.Title,
		Description: resource.Description,
		MIMEType:    resource.MIMEType,
		Size:        resource.Size,
		Annotations: annotationsFromMCP(resource.Annotations),
		Meta:        metaFromMCP(resource.Meta),
	}
}

// PromptFromMCP converts an MCP prompt to a domain definition.
func PromptFromMCP(prompt *mcp.Prompt) domain.PromptDefinition {
	if prompt == nil {
		return domain.PromptDefinition{}
	}
	return domain.PromptDefinition{
		Name:        prompt.Name,
		Title:       prompt.Title,
		Description: prompt.Description,
		Arguments:   promptArgumentsFromMCP(prompt.Arguments),
		Meta:        metaFromMCP(prompt.Meta),
	}
}

// MarshalToolDefinition encodes a tool definition as MCP JSON.
func MarshalToolDefinition(tool domain.ToolDefinition) ([]byte, error) {
	wire := toolToMCP(tool)
	return json.Marshal(&wire)
}

// MarshalResourceDefinition encodes a resource definition as MCP JSON.
func MarshalResourceDefinition(resource domain.ResourceDefinition) ([]byte, error) {
	wire := resourceToMCP(resource)
	return json.Marshal(&wire)
}

// MarshalPromptDefinition encodes a prompt definition as MCP JSON.
func MarshalPromptDefinition(prompt domain.PromptDefinition) ([]byte, error) {
	wire := promptToMCP(prompt)
	return json.Marshal(&wire)
}

// MustMarshalToolDefinition encodes a tool definition or panics.
func MustMarshalToolDefinition(tool domain.ToolDefinition) []byte {
	raw, err := MarshalToolDefinition(tool)
	if err != nil {
		panic(err)
	}
	return raw
}

// MustMarshalResourceDefinition encodes a resource definition or panics.
func MustMarshalResourceDefinition(resource domain.ResourceDefinition) []byte {
	raw, err := MarshalResourceDefinition(resource)
	if err != nil {
		panic(err)
	}
	return raw
}

// MustMarshalPromptDefinition encodes a prompt definition or panics.
func MustMarshalPromptDefinition(prompt domain.PromptDefinition) []byte {
	raw, err := MarshalPromptDefinition(prompt)
	if err != nil {
		panic(err)
	}
	return raw
}

// HashToolDefinition returns a deterministic hash for a tool definition or an error.
func HashToolDefinition(tool domain.ToolDefinition) (string, error) {
	raw, err := MarshalToolDefinition(tool)
	if err != nil {
		return "", fmt.Errorf("marshal tool definition: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// HashResourceDefinition returns a deterministic hash for a resource definition or an error.
func HashResourceDefinition(resource domain.ResourceDefinition) (string, error) {
	raw, err := MarshalResourceDefinition(resource)
	if err != nil {
		return "", fmt.Errorf("marshal resource definition: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// HashPromptDefinition returns a deterministic hash for a prompt definition or an error.
func HashPromptDefinition(prompt domain.PromptDefinition) (string, error) {
	raw, err := MarshalPromptDefinition(prompt)
	if err != nil {
		return "", fmt.Errorf("marshal prompt definition: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// HashToolDefinitions returns a deterministic hash for a tool list or an error.
func HashToolDefinitions(tools []domain.ToolDefinition) (string, error) {
	hasher := sha256.New()
	for i, tool := range tools {
		raw, err := MarshalToolDefinition(tool)
		if err != nil {
			return "", fmt.Errorf("marshal tool definition %d: %w", i, err)
		}
		_, _ = hasher.Write(raw)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// HashResourceDefinitions returns a deterministic hash for a resource list or an error.
func HashResourceDefinitions(resources []domain.ResourceDefinition) (string, error) {
	hasher := sha256.New()
	for i, resource := range resources {
		raw, err := MarshalResourceDefinition(resource)
		if err != nil {
			return "", fmt.Errorf("marshal resource definition %d: %w", i, err)
		}
		_, _ = hasher.Write(raw)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// HashPromptDefinitions returns a deterministic hash for a prompt list or an error.
func HashPromptDefinitions(prompts []domain.PromptDefinition) (string, error) {
	hasher := sha256.New()
	for i, prompt := range prompts {
		raw, err := MarshalPromptDefinition(prompt)
		if err != nil {
			return "", fmt.Errorf("marshal prompt definition %d: %w", i, err)
		}
		_, _ = hasher.Write(raw)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func toolToMCP(tool domain.ToolDefinition) mcp.Tool {
	return mcp.Tool{
		Meta:         metaToMCP(tool.Meta),
		Annotations:  toolAnnotationsToMCP(tool.Annotations),
		Description:  tool.Description,
		InputSchema:  tool.InputSchema,
		Name:         tool.Name,
		OutputSchema: tool.OutputSchema,
		Title:        tool.Title,
	}
}

func resourceToMCP(resource domain.ResourceDefinition) mcp.Resource {
	return mcp.Resource{
		Meta:        metaToMCP(resource.Meta),
		Annotations: annotationsToMCP(resource.Annotations),
		Description: resource.Description,
		MIMEType:    resource.MIMEType,
		Name:        resource.Name,
		Size:        resource.Size,
		Title:       resource.Title,
		URI:         resource.URI,
	}
}

func promptToMCP(prompt domain.PromptDefinition) mcp.Prompt {
	return mcp.Prompt{
		Meta:        metaToMCP(prompt.Meta),
		Arguments:   promptArgumentsToMCP(prompt.Arguments),
		Description: prompt.Description,
		Name:        prompt.Name,
		Title:       prompt.Title,
	}
}

func metaFromMCP(meta mcp.Meta) domain.Meta {
	if meta == nil {
		return nil
	}
	cloned := domain.CloneJSONValue(map[string]any(meta))
	if typed, ok := cloned.(map[string]any); ok {
		return domain.Meta(typed)
	}
	return nil
}

func metaToMCP(meta domain.Meta) mcp.Meta {
	if meta == nil {
		return nil
	}
	cloned := domain.CloneJSONValue(map[string]any(meta))
	if typed, ok := cloned.(map[string]any); ok {
		return mcp.Meta(typed)
	}
	return nil
}

func annotationsFromMCP(ann *mcp.Annotations) *domain.Annotations {
	if ann == nil {
		return nil
	}
	out := domain.Annotations{
		Audience:     make([]domain.Role, 0, len(ann.Audience)),
		LastModified: ann.LastModified,
		Priority:     ann.Priority,
	}
	for _, role := range ann.Audience {
		out.Audience = append(out.Audience, domain.Role(role))
	}
	return &out
}

func annotationsToMCP(ann *domain.Annotations) *mcp.Annotations {
	if ann == nil {
		return nil
	}
	out := mcp.Annotations{
		Audience:     make([]mcp.Role, 0, len(ann.Audience)),
		LastModified: ann.LastModified,
		Priority:     ann.Priority,
	}
	for _, role := range ann.Audience {
		out.Audience = append(out.Audience, mcp.Role(role))
	}
	return &out
}

func toolAnnotationsFromMCP(ann *mcp.ToolAnnotations) *domain.ToolAnnotations {
	if ann == nil {
		return nil
	}
	out := domain.ToolAnnotations{
		IdempotentHint: ann.IdempotentHint,
		ReadOnlyHint:   ann.ReadOnlyHint,
		Title:          ann.Title,
	}
	if ann.DestructiveHint != nil {
		val := *ann.DestructiveHint
		out.DestructiveHint = &val
	}
	if ann.OpenWorldHint != nil {
		val := *ann.OpenWorldHint
		out.OpenWorldHint = &val
	}
	return &out
}

func toolAnnotationsToMCP(ann *domain.ToolAnnotations) *mcp.ToolAnnotations {
	if ann == nil {
		return nil
	}
	out := mcp.ToolAnnotations{
		IdempotentHint: ann.IdempotentHint,
		ReadOnlyHint:   ann.ReadOnlyHint,
		Title:          ann.Title,
	}
	if ann.DestructiveHint != nil {
		val := *ann.DestructiveHint
		out.DestructiveHint = &val
	}
	if ann.OpenWorldHint != nil {
		val := *ann.OpenWorldHint
		out.OpenWorldHint = &val
	}
	return &out
}

func promptArgumentsFromMCP(args []*mcp.PromptArgument) []domain.PromptArgument {
	if len(args) == 0 {
		return nil
	}
	out := make([]domain.PromptArgument, 0, len(args))
	for _, arg := range args {
		if arg == nil {
			continue
		}
		out = append(out, domain.PromptArgument{
			Name:        arg.Name,
			Title:       arg.Title,
			Description: arg.Description,
			Required:    arg.Required,
		})
	}
	return out
}

func promptArgumentsToMCP(args []domain.PromptArgument) []*mcp.PromptArgument {
	if len(args) == 0 {
		return nil
	}
	out := make([]*mcp.PromptArgument, 0, len(args))
	for _, arg := range args {
		argCopy := arg
		out = append(out, &mcp.PromptArgument{
			Name:        argCopy.Name,
			Title:       argCopy.Title,
			Description: argCopy.Description,
			Required:    argCopy.Required,
		})
	}
	return out
}
