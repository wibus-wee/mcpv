package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mcpcodec"
)

// ToolETag returns an ETag for a tool list and logs on failure.
func ToolETag(logger *zap.Logger, tools []domain.ToolDefinition) string {
	return hashWithLogger(logger, "tool", func() (string, error) {
		return mcpcodec.HashToolDefinitions(tools)
	})
}

// ResourceETag returns an ETag for a resource list and logs on failure.
func ResourceETag(logger *zap.Logger, resources []domain.ResourceDefinition) string {
	return hashWithLogger(logger, "resource", func() (string, error) {
		return mcpcodec.HashResourceDefinitions(resources)
	})
}

// PromptETag returns an ETag for a prompt list and logs on failure.
func PromptETag(logger *zap.Logger, prompts []domain.PromptDefinition) string {
	return hashWithLogger(logger, "prompt", func() (string, error) {
		return mcpcodec.HashPromptDefinitions(prompts)
	})
}

// ToolCatalogETag returns an ETag for tool catalog entries.
func ToolCatalogETag(logger *zap.Logger, tools []domain.ToolCatalogEntry) string {
	return hashWithLogger(logger, "tool_catalog", func() (string, error) {
		data, err := json.Marshal(tools)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:]), nil
	})
}

func hashWithLogger(logger *zap.Logger, label string, fn func() (string, error)) string {
	etag, err := fn()
	if err != nil {
		if logger != nil {
			logger.Warn(fmt.Sprintf("%s hash failed", label), zap.Error(err))
		}
		return ""
	}
	return etag
}
