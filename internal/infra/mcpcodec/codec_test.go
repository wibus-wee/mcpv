package mcpcodec

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcpv/internal/domain"
)

const toolDefinitionSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["name", "inputSchema"],
  "properties": {
    "_meta": { "$ref": "#/$defs/meta" },
    "annotations": { "$ref": "#/$defs/toolAnnotations" },
    "description": { "type": "string" },
    "inputSchema": { "type": "object" },
    "name": { "type": "string" },
    "outputSchema": { "type": "object" },
    "title": { "type": "string" },
    "icons": { "type": "array", "items": { "$ref": "#/$defs/icon" } }
  },
  "additionalProperties": true,
  "$defs": {
    "meta": {
      "type": "object"
    },
    "toolAnnotations": {
      "type": "object",
      "properties": {
        "idempotentHint": { "type": "boolean" },
        "readOnlyHint": { "type": "boolean" },
        "destructiveHint": { "type": ["boolean", "null"] },
        "openWorldHint": { "type": ["boolean", "null"] },
        "title": { "type": "string" }
      },
      "additionalProperties": true
    },
    "icon": {
      "type": "object",
      "required": ["src"],
      "properties": {
        "src": { "type": "string" },
        "mimeType": { "type": "string" },
        "sizes": { "type": "array", "items": { "type": "string" } },
        "theme": { "type": "string" }
      },
      "additionalProperties": true
    }
  }
}`

const resourceDefinitionSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["uri", "name"],
  "properties": {
    "_meta": { "$ref": "#/$defs/meta" },
    "annotations": { "$ref": "#/$defs/annotations" },
    "description": { "type": "string" },
    "mimeType": { "type": "string" },
    "name": { "type": "string" },
    "size": { "type": "integer" },
    "title": { "type": "string" },
    "uri": { "type": "string" },
    "icons": { "type": "array", "items": { "$ref": "#/$defs/icon" } }
  },
  "additionalProperties": true,
  "$defs": {
    "meta": {
      "type": "object"
    },
    "annotations": {
      "type": "object",
      "properties": {
        "audience": { "type": "array", "items": { "type": "string" } },
        "lastModified": { "type": "string" },
        "priority": { "type": "number" }
      },
      "additionalProperties": true
    },
    "icon": {
      "type": "object",
      "required": ["src"],
      "properties": {
        "src": { "type": "string" },
        "mimeType": { "type": "string" },
        "sizes": { "type": "array", "items": { "type": "string" } },
        "theme": { "type": "string" }
      },
      "additionalProperties": true
    }
  }
}`

const promptDefinitionSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["name"],
  "properties": {
    "_meta": { "$ref": "#/$defs/meta" },
    "arguments": { "type": "array", "items": { "$ref": "#/$defs/promptArgument" } },
    "description": { "type": "string" },
    "name": { "type": "string" },
    "title": { "type": "string" },
    "icons": { "type": "array", "items": { "$ref": "#/$defs/icon" } }
  },
  "additionalProperties": true,
  "$defs": {
    "meta": {
      "type": "object"
    },
    "promptArgument": {
      "type": "object",
      "required": ["name"],
      "properties": {
        "name": { "type": "string" },
        "title": { "type": "string" },
        "description": { "type": "string" },
        "required": { "type": "boolean" }
      },
      "additionalProperties": true
    },
    "icon": {
      "type": "object",
      "required": ["src"],
      "properties": {
        "src": { "type": "string" },
        "mimeType": { "type": "string" },
        "sizes": { "type": "array", "items": { "type": "string" } },
        "theme": { "type": "string" }
      },
      "additionalProperties": true
    }
  }
}`

func validateAgainstSchema(t *testing.T, schemaJSON string, payload []byte) {
	t.Helper()

	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), &schema))

	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)

	var decoded any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.NoError(t, resolved.Validate(decoded))
}

// TestHashToolDefinition_Deterministic verifies that hashing is deterministic.
func TestHashToolDefinition_Deterministic(t *testing.T) {
	tests := []struct {
		name     string
		tool1    domain.ToolDefinition
		tool2    domain.ToolDefinition
		sameHash bool
	}{
		{
			name: "identical tools produce same hash",
			tool1: domain.ToolDefinition{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]any{"type": "object"},
			},
			tool2: domain.ToolDefinition{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]any{"type": "object"},
			},
			sameHash: true,
		},
		{
			name: "different names produce different hashes",
			tool1: domain.ToolDefinition{
				Name:        "tool_a",
				Description: "A test tool",
			},
			tool2: domain.ToolDefinition{
				Name:        "tool_b",
				Description: "A test tool",
			},
			sameHash: false,
		},
		{
			name: "different descriptions produce different hashes",
			tool1: domain.ToolDefinition{
				Name:        "test_tool",
				Description: "Description A",
			},
			tool2: domain.ToolDefinition{
				Name:        "test_tool",
				Description: "Description B",
			},
			sameHash: false,
		},
		{
			name: "different schemas produce different hashes",
			tool1: domain.ToolDefinition{
				Name:        "test_tool",
				InputSchema: map[string]any{"type": "object"},
			},
			tool2: domain.ToolDefinition{
				Name:        "test_tool",
				InputSchema: map[string]any{"type": "array"},
			},
			sameHash: false,
		},
		{
			name:     "empty tools produce same hash",
			tool1:    domain.ToolDefinition{},
			tool2:    domain.ToolDefinition{},
			sameHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err1 := HashToolDefinition(tt.tool1)
			hash2, err2 := HashToolDefinition(tt.tool2)

			require.NoError(t, err1)
			require.NoError(t, err2)

			if tt.sameHash {
				assert.Equal(t, hash1, hash2, "Expected identical hashes")
			} else {
				assert.NotEqual(t, hash1, hash2, "Expected different hashes")
			}
		})
	}
}

// TestHashToolDefinition_Concurrent verifies thread-safe hashing.
func TestHashToolDefinition_Concurrent(t *testing.T) {
	tool := domain.ToolDefinition{
		Name:        "concurrent_tool",
		Description: "Test concurrent hashing",
		InputSchema: map[string]any{"type": "object"},
	}

	const goroutines = 100
	hashes := make([]string, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			hash, err := HashToolDefinition(tool)
			hashes[idx] = hash
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "hash error at index %d", i)
	}

	// All hashes should be identical
	expectedHash := hashes[0]
	for i, hash := range hashes {
		assert.Equal(t, expectedHash, hash, "Hash mismatch at index %d", i)
	}
}

// TestHashToolDefinitions_ListHashing verifies list hashing behavior.
func TestHashToolDefinitions_ListHashing(t *testing.T) {
	tool1 := domain.ToolDefinition{Name: "tool1", Description: "First tool"}
	tool2 := domain.ToolDefinition{Name: "tool2", Description: "Second tool"}
	tool3 := domain.ToolDefinition{Name: "tool3", Description: "Third tool"}

	tests := []struct {
		name     string
		list1    []domain.ToolDefinition
		list2    []domain.ToolDefinition
		sameHash bool
	}{
		{
			name:     "identical lists produce same hash",
			list1:    []domain.ToolDefinition{tool1, tool2},
			list2:    []domain.ToolDefinition{tool1, tool2},
			sameHash: true,
		},
		{
			name:     "different order produces different hash",
			list1:    []domain.ToolDefinition{tool1, tool2},
			list2:    []domain.ToolDefinition{tool2, tool1},
			sameHash: false,
		},
		{
			name:     "different length produces different hash",
			list1:    []domain.ToolDefinition{tool1, tool2},
			list2:    []domain.ToolDefinition{tool1, tool2, tool3},
			sameHash: false,
		},
		{
			name:     "empty lists produce same hash",
			list1:    []domain.ToolDefinition{},
			list2:    []domain.ToolDefinition{},
			sameHash: true,
		},
		{
			name:     "nil lists produce same hash",
			list1:    nil,
			list2:    nil,
			sameHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err1 := HashToolDefinitions(tt.list1)
			hash2, err2 := HashToolDefinitions(tt.list2)

			require.NoError(t, err1)
			require.NoError(t, err2)

			if tt.sameHash {
				assert.Equal(t, hash1, hash2, "Expected identical hashes")
			} else {
				assert.NotEqual(t, hash1, hash2, "Expected different hashes")
			}
		})
	}
}

// TestRoundTrip_ToolDefinition verifies encoding preserves all fields.
func TestRoundTrip_ToolDefinition(t *testing.T) {
	boolTrue := true
	boolFalse := false

	original := domain.ToolDefinition{
		Name:        "test_tool",
		Description: "A comprehensive test tool",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param1": map[string]any{"type": "string"},
				"param2": map[string]any{"type": "number"},
			},
			"required": []any{"param1"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"result": map[string]any{"type": "string"},
			},
		},
		Title: "Test Tool",
		Annotations: &domain.ToolAnnotations{
			IdempotentHint:  true,
			ReadOnlyHint:    false,
			DestructiveHint: &boolFalse,
			OpenWorldHint:   &boolTrue,
			Title:           "Annotated Tool",
		},
		Meta: domain.Meta{
			"version": "1.0.0",
			"author":  "test",
		},
	}

	// Marshal to MCP JSON
	data, err := MarshalToolDefinition(original)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	validateAgainstSchema(t, toolDefinitionSchema, data)

	// Unmarshal back from MCP JSON
	var mcpTool mcp.Tool
	err = json.Unmarshal(data, &mcpTool)
	require.NoError(t, err)

	// Convert back to domain
	roundtripped := ToolFromMCP(&mcpTool)

	// Verify all fields preserved
	assert.Equal(t, original.Name, roundtripped.Name)
	assert.Equal(t, original.Description, roundtripped.Description)
	assert.Equal(t, original.Title, roundtripped.Title)

	// Verify schemas preserved (deep comparison)
	originalInputJSON, _ := json.Marshal(original.InputSchema)
	roundtrippedInputJSON, _ := json.Marshal(roundtripped.InputSchema)
	assert.JSONEq(t, string(originalInputJSON), string(roundtrippedInputJSON))

	if original.OutputSchema != nil {
		originalOutputJSON, _ := json.Marshal(original.OutputSchema)
		roundtrippedOutputJSON, _ := json.Marshal(roundtripped.OutputSchema)
		assert.JSONEq(t, string(originalOutputJSON), string(roundtrippedOutputJSON))
	}

	// Verify annotations preserved
	if original.Annotations != nil {
		require.NotNil(t, roundtripped.Annotations)
		assert.Equal(t, original.Annotations.IdempotentHint, roundtripped.Annotations.IdempotentHint)
		assert.Equal(t, original.Annotations.ReadOnlyHint, roundtripped.Annotations.ReadOnlyHint)
		assert.Equal(t, *original.Annotations.DestructiveHint, *roundtripped.Annotations.DestructiveHint)
		assert.Equal(t, *original.Annotations.OpenWorldHint, *roundtripped.Annotations.OpenWorldHint)
		assert.Equal(t, original.Annotations.Title, roundtripped.Annotations.Title)
	}

	// Verify meta preserved
	assert.Equal(t, original.Meta, roundtripped.Meta)

	// Hash should be deterministic
	hash1, err := HashToolDefinition(original)
	require.NoError(t, err)
	hash2, err := HashToolDefinition(roundtripped)
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2, "Hash should be same after round-trip")
}

// TestRoundTrip_ResourceDefinition verifies encoding preserves all fields.
func TestRoundTrip_ResourceDefinition(t *testing.T) {
	original := domain.ResourceDefinition{
		URI:         "file:///test/resource.txt",
		Name:        "test_resource",
		Title:       "Test Resource",
		Description: "A test resource",
		MIMEType:    "text/plain",
		Size:        1024,
		Annotations: &domain.Annotations{
			Audience:     []domain.Role{"user", "assistant"},
			LastModified: "2024-01-01T00:00:00Z",
			Priority:     1.0,
		},
		Meta: domain.Meta{
			"version": "1.0.0",
		},
	}

	// Marshal to MCP JSON
	data, err := MarshalResourceDefinition(original)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	validateAgainstSchema(t, resourceDefinitionSchema, data)

	// Unmarshal back from MCP JSON
	var mcpResource mcp.Resource
	err = json.Unmarshal(data, &mcpResource)
	require.NoError(t, err)

	// Convert back to domain
	roundtripped := ResourceFromMCP(&mcpResource)

	// Verify all fields preserved
	assert.Equal(t, original.URI, roundtripped.URI)
	assert.Equal(t, original.Name, roundtripped.Name)
	assert.Equal(t, original.Title, roundtripped.Title)
	assert.Equal(t, original.Description, roundtripped.Description)
	assert.Equal(t, original.MIMEType, roundtripped.MIMEType)
	assert.Equal(t, original.Size, roundtripped.Size)

	// Verify annotations preserved
	if original.Annotations != nil {
		require.NotNil(t, roundtripped.Annotations)
		assert.Equal(t, original.Annotations.Audience, roundtripped.Annotations.Audience)
		assert.Equal(t, original.Annotations.LastModified, roundtripped.Annotations.LastModified)
		assert.Equal(t, original.Annotations.Priority, roundtripped.Annotations.Priority)
	}

	// Verify meta preserved
	assert.Equal(t, original.Meta, roundtripped.Meta)

	// Hash should be same after round-trip
	hash1, err := HashResourceDefinition(original)
	require.NoError(t, err)
	hash2, err := HashResourceDefinition(roundtripped)
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2, "Hash should be same after round-trip")
}

// TestRoundTrip_PromptDefinition verifies encoding preserves all fields.
func TestRoundTrip_PromptDefinition(t *testing.T) {
	original := domain.PromptDefinition{
		Name:        "test_prompt",
		Title:       "Test Prompt",
		Description: "A test prompt",
		Arguments: []domain.PromptArgument{
			{
				Name:        "arg1",
				Title:       "Argument 1",
				Description: "First argument",
				Required:    true,
			},
			{
				Name:        "arg2",
				Title:       "Argument 2",
				Description: "Second argument",
				Required:    false,
			},
		},
		Meta: domain.Meta{
			"version": "1.0.0",
		},
	}

	// Marshal to MCP JSON
	data, err := MarshalPromptDefinition(original)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	validateAgainstSchema(t, promptDefinitionSchema, data)

	// Unmarshal back from MCP JSON
	var mcpPrompt mcp.Prompt
	err = json.Unmarshal(data, &mcpPrompt)
	require.NoError(t, err)

	// Convert back to domain
	roundtripped := PromptFromMCP(&mcpPrompt)

	// Verify all fields preserved
	assert.Equal(t, original.Name, roundtripped.Name)
	assert.Equal(t, original.Title, roundtripped.Title)
	assert.Equal(t, original.Description, roundtripped.Description)

	// Verify arguments preserved
	require.Equal(t, len(original.Arguments), len(roundtripped.Arguments))
	for i, arg := range original.Arguments {
		assert.Equal(t, arg.Name, roundtripped.Arguments[i].Name)
		assert.Equal(t, arg.Title, roundtripped.Arguments[i].Title)
		assert.Equal(t, arg.Description, roundtripped.Arguments[i].Description)
		assert.Equal(t, arg.Required, roundtripped.Arguments[i].Required)
	}

	// Verify meta preserved
	assert.Equal(t, original.Meta, roundtripped.Meta)

	// Hash should be same after round-trip
	hash1, err := HashPromptDefinition(original)
	require.NoError(t, err)
	hash2, err := HashPromptDefinition(roundtripped)
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2, "Hash should be same after round-trip")
}

// TestHashResourceDefinitions_ListHashing verifies resource list hashing.
func TestHashResourceDefinitions_ListHashing(t *testing.T) {
	resource1 := domain.ResourceDefinition{URI: "file:///a", Name: "resource1"}
	resource2 := domain.ResourceDefinition{URI: "file:///b", Name: "resource2"}

	tests := []struct {
		name     string
		list1    []domain.ResourceDefinition
		list2    []domain.ResourceDefinition
		sameHash bool
	}{
		{
			name:     "identical lists produce same hash",
			list1:    []domain.ResourceDefinition{resource1, resource2},
			list2:    []domain.ResourceDefinition{resource1, resource2},
			sameHash: true,
		},
		{
			name:     "different order produces different hash",
			list1:    []domain.ResourceDefinition{resource1, resource2},
			list2:    []domain.ResourceDefinition{resource2, resource1},
			sameHash: false,
		},
		{
			name:     "empty lists produce same hash",
			list1:    []domain.ResourceDefinition{},
			list2:    []domain.ResourceDefinition{},
			sameHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err1 := HashResourceDefinitions(tt.list1)
			hash2, err2 := HashResourceDefinitions(tt.list2)

			require.NoError(t, err1)
			require.NoError(t, err2)

			if tt.sameHash {
				assert.Equal(t, hash1, hash2)
			} else {
				assert.NotEqual(t, hash1, hash2)
			}
		})
	}
}

// TestHashPromptDefinitions_ListHashing verifies prompt list hashing.
func TestHashPromptDefinitions_ListHashing(t *testing.T) {
	prompt1 := domain.PromptDefinition{Name: "prompt1", Description: "First"}
	prompt2 := domain.PromptDefinition{Name: "prompt2", Description: "Second"}

	tests := []struct {
		name     string
		list1    []domain.PromptDefinition
		list2    []domain.PromptDefinition
		sameHash bool
	}{
		{
			name:     "identical lists produce same hash",
			list1:    []domain.PromptDefinition{prompt1, prompt2},
			list2:    []domain.PromptDefinition{prompt1, prompt2},
			sameHash: true,
		},
		{
			name:     "different order produces different hash",
			list1:    []domain.PromptDefinition{prompt1, prompt2},
			list2:    []domain.PromptDefinition{prompt2, prompt1},
			sameHash: false,
		},
		{
			name:     "empty lists produce same hash",
			list1:    []domain.PromptDefinition{},
			list2:    []domain.PromptDefinition{},
			sameHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err1 := HashPromptDefinitions(tt.list1)
			hash2, err2 := HashPromptDefinitions(tt.list2)

			require.NoError(t, err1)
			require.NoError(t, err2)

			if tt.sameHash {
				assert.Equal(t, hash1, hash2)
			} else {
				assert.NotEqual(t, hash1, hash2)
			}
		})
	}
}

// TestToolFromMCP_Conversion verifies MCP to domain conversion.
func TestToolFromMCP_Conversion(t *testing.T) {
	boolTrue := true
	boolFalse := false

	mcpTool := &mcp.Tool{
		Name:        "test_tool",
		Description: "Test description",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param": map[string]any{"type": "string"},
			},
		},
		OutputSchema: map[string]any{"type": "string"},
		Title:        "Test Tool",
		Annotations: &mcp.ToolAnnotations{
			IdempotentHint:  true,
			ReadOnlyHint:    false,
			DestructiveHint: &boolFalse,
			OpenWorldHint:   &boolTrue,
			Title:           "Annotated",
		},
		Meta: mcp.Meta{
			"version": "1.0",
		},
	}

	result := ToolFromMCP(mcpTool)

	assert.Equal(t, "test_tool", result.Name)
	assert.Equal(t, "Test description", result.Description)
	assert.Equal(t, "Test Tool", result.Title)
	assert.NotNil(t, result.InputSchema)
	assert.NotNil(t, result.OutputSchema)
	assert.NotNil(t, result.Annotations)
	assert.True(t, result.Annotations.IdempotentHint)
	assert.False(t, result.Annotations.ReadOnlyHint)
	assert.NotNil(t, result.Annotations.DestructiveHint)
	assert.False(t, *result.Annotations.DestructiveHint)
	assert.NotNil(t, result.Annotations.OpenWorldHint)
	assert.True(t, *result.Annotations.OpenWorldHint)
	assert.NotNil(t, result.Meta)
}

// TestResourceFromMCP_Conversion verifies MCP to domain conversion.
func TestResourceFromMCP_Conversion(t *testing.T) {
	mcpResource := &mcp.Resource{
		URI:         "file:///test.txt",
		Name:        "test_resource",
		Title:       "Test Resource",
		Description: "Test description",
		MIMEType:    "text/plain",
		Size:        1024,
		Annotations: &mcp.Annotations{
			Audience:     []mcp.Role{"user"},
			LastModified: "2024-01-01T00:00:00Z",
			Priority:     1.0,
		},
		Meta: mcp.Meta{
			"version": "1.0",
		},
	}

	result := ResourceFromMCP(mcpResource)

	assert.Equal(t, "file:///test.txt", result.URI)
	assert.Equal(t, "test_resource", result.Name)
	assert.Equal(t, "Test Resource", result.Title)
	assert.Equal(t, "Test description", result.Description)
	assert.Equal(t, "text/plain", result.MIMEType)
	assert.Equal(t, int64(1024), result.Size)
	assert.NotNil(t, result.Annotations)
	assert.Len(t, result.Annotations.Audience, 1)
	assert.Equal(t, domain.Role("user"), result.Annotations.Audience[0])
	assert.NotNil(t, result.Meta)
}

// TestPromptFromMCP_Conversion verifies MCP to domain conversion.
func TestPromptFromMCP_Conversion(t *testing.T) {
	mcpPrompt := &mcp.Prompt{
		Name:        "test_prompt",
		Title:       "Test Prompt",
		Description: "Test description",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "arg1",
				Title:       "Argument 1",
				Description: "First argument",
				Required:    true,
			},
			{
				Name:        "arg2",
				Title:       "Argument 2",
				Description: "Second argument",
				Required:    false,
			},
		},
		Meta: mcp.Meta{
			"version": "1.0",
		},
	}

	result := PromptFromMCP(mcpPrompt)

	assert.Equal(t, "test_prompt", result.Name)
	assert.Equal(t, "Test Prompt", result.Title)
	assert.Equal(t, "Test description", result.Description)
	assert.Len(t, result.Arguments, 2)
	assert.Equal(t, "arg1", result.Arguments[0].Name)
	assert.True(t, result.Arguments[0].Required)
	assert.Equal(t, "arg2", result.Arguments[1].Name)
	assert.False(t, result.Arguments[1].Required)
	assert.NotNil(t, result.Meta)
}

// TestPromptFromMCP_NilArguments verifies nil argument handling.
func TestPromptFromMCP_NilArguments(t *testing.T) {
	mcpPrompt := &mcp.Prompt{
		Name:      "test_prompt",
		Arguments: []*mcp.PromptArgument{nil, {Name: "valid"}},
	}

	result := PromptFromMCP(mcpPrompt)

	// Nil arguments should be skipped
	assert.Len(t, result.Arguments, 1)
	assert.Equal(t, "valid", result.Arguments[0].Name)
}

// TestIsObjectSchema verifies schema type detection.
func TestIsObjectSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   any
		isObject bool
	}{
		{
			name:     "object schema detected from map",
			schema:   map[string]any{"type": "object"},
			isObject: true,
		},
		{
			name:     "object schema detected case-insensitive",
			schema:   map[string]any{"type": "Object"},
			isObject: true,
		},
		{
			name:     "array schema not detected",
			schema:   map[string]any{"type": "array"},
			isObject: false,
		},
		{
			name:     "string schema not detected",
			schema:   map[string]any{"type": "string"},
			isObject: false,
		},
		{
			name:     "object in type array detected",
			schema:   map[string]any{"type": []any{"string", "object"}},
			isObject: true,
		},
		{
			name:     "object in string array detected",
			schema:   map[string]any{"type": []string{"string", "object"}},
			isObject: true,
		},
		{
			name:     "nil schema returns false",
			schema:   nil,
			isObject: false,
		},
		{
			name:     "empty map returns false",
			schema:   map[string]any{},
			isObject: false,
		},
		{
			name:     "JSON string with object type",
			schema:   `{"type": "object"}`,
			isObject: true,
		},
		{
			name:     "JSON bytes with object type",
			schema:   []byte(`{"type": "object"}`),
			isObject: true,
		},
		{
			name:     "JSON RawMessage with object type",
			schema:   json.RawMessage(`{"type": "object"}`),
			isObject: true,
		},
		{
			name:     "invalid JSON returns false",
			schema:   `{invalid json}`,
			isObject: false,
		},
		{
			name:     "empty JSON bytes returns false",
			schema:   []byte{},
			isObject: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsObjectSchema(tt.schema)
			assert.Equal(t, tt.isObject, result)
		})
	}
}

// TestMetaConversion verifies meta field conversion.
func TestMetaConversion(t *testing.T) {
	t.Run("nil meta returns nil", func(t *testing.T) {
		tool := domain.ToolDefinition{
			Name: "test",
			Meta: nil,
		}
		data, err := MarshalToolDefinition(tool)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		var mcpTool mcp.Tool
		require.NoError(t, json.Unmarshal(data, &mcpTool))
		assert.Nil(t, mcpTool.Meta)

		roundtripped := ToolFromMCP(&mcpTool)
		assert.Nil(t, roundtripped.Meta)
	})

	t.Run("meta with nested objects", func(t *testing.T) {
		expected := domain.Meta{
			"nested": map[string]any{
				"key": "value",
			},
		}
		tool := domain.ToolDefinition{
			Name: "test",
			Meta: expected,
		}
		data, err := MarshalToolDefinition(tool)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		var mcpTool mcp.Tool
		require.NoError(t, json.Unmarshal(data, &mcpTool))

		roundtripped := ToolFromMCP(&mcpTool)
		require.NotNil(t, roundtripped.Meta)
		assert.Equal(t, expected, roundtripped.Meta)
	})
}

// TestAnnotationsConversion verifies annotations conversion.
func TestAnnotationsConversion(t *testing.T) {
	t.Run("nil annotations handled", func(t *testing.T) {
		resource := domain.ResourceDefinition{
			URI:         "file:///test",
			Annotations: nil,
		}
		data, err := MarshalResourceDefinition(resource)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		var mcpResource mcp.Resource
		require.NoError(t, json.Unmarshal(data, &mcpResource))
		assert.Nil(t, mcpResource.Annotations)

		roundtripped := ResourceFromMCP(&mcpResource)
		assert.Nil(t, roundtripped.Annotations)
	})

	t.Run("empty audience handled", func(t *testing.T) {
		resource := domain.ResourceDefinition{
			URI: "file:///test",
			Annotations: &domain.Annotations{
				Audience: []domain.Role{},
			},
		}
		data, err := MarshalResourceDefinition(resource)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		var mcpResource mcp.Resource
		require.NoError(t, json.Unmarshal(data, &mcpResource))
		require.NotNil(t, mcpResource.Annotations)

		roundtripped := ResourceFromMCP(&mcpResource)
		require.NotNil(t, roundtripped.Annotations)
		assert.Empty(t, roundtripped.Annotations.Audience)
	})
}

// TestToolAnnotationsConversion verifies tool annotations conversion.
func TestToolAnnotationsConversion(t *testing.T) {
	t.Run("nil tool annotations handled", func(t *testing.T) {
		tool := domain.ToolDefinition{
			Name:        "test",
			Annotations: nil,
		}
		data, err := MarshalToolDefinition(tool)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		var mcpTool mcp.Tool
		require.NoError(t, json.Unmarshal(data, &mcpTool))
		assert.Nil(t, mcpTool.Annotations)

		roundtripped := ToolFromMCP(&mcpTool)
		assert.Nil(t, roundtripped.Annotations)
	})

	t.Run("tool annotations with nil hints", func(t *testing.T) {
		tool := domain.ToolDefinition{
			Name: "test",
			Annotations: &domain.ToolAnnotations{
				IdempotentHint:  true,
				DestructiveHint: nil,
				OpenWorldHint:   nil,
			},
		}
		data, err := MarshalToolDefinition(tool)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		var mcpTool mcp.Tool
		require.NoError(t, json.Unmarshal(data, &mcpTool))

		roundtripped := ToolFromMCP(&mcpTool)
		require.NotNil(t, roundtripped.Annotations)
		assert.True(t, roundtripped.Annotations.IdempotentHint)
		assert.Nil(t, roundtripped.Annotations.DestructiveHint)
		assert.Nil(t, roundtripped.Annotations.OpenWorldHint)
	})
}

// TestPromptArgumentsConversion verifies prompt arguments conversion.
func TestPromptArgumentsConversion(t *testing.T) {
	t.Run("empty arguments handled", func(t *testing.T) {
		prompt := domain.PromptDefinition{
			Name:      "test",
			Arguments: []domain.PromptArgument{},
		}
		data, err := MarshalPromptDefinition(prompt)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		var mcpPrompt mcp.Prompt
		require.NoError(t, json.Unmarshal(data, &mcpPrompt))
		assert.Nil(t, mcpPrompt.Arguments)

		roundtripped := PromptFromMCP(&mcpPrompt)
		assert.Nil(t, roundtripped.Arguments)
	})

	t.Run("nil arguments handled", func(t *testing.T) {
		prompt := domain.PromptDefinition{
			Name:      "test",
			Arguments: nil,
		}
		data, err := MarshalPromptDefinition(prompt)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		var mcpPrompt mcp.Prompt
		require.NoError(t, json.Unmarshal(data, &mcpPrompt))
		assert.Nil(t, mcpPrompt.Arguments)

		roundtripped := PromptFromMCP(&mcpPrompt)
		assert.Nil(t, roundtripped.Arguments)
	})
}
