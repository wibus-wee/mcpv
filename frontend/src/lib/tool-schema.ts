// Shared tool schema parsing utilities
import type { ResourceEntry, ToolEntry } from '@bindings/mcpv/internal/ui/types'

export interface ToolSchema {
  name?: string
  description?: string
  inputSchema?: {
    type?: string
    properties?: Record<string, {
      type?: string
      description?: string
      enum?: string[]
      default?: unknown
    }>
    required?: string[]
  }
}

export interface ResourceSchema {
  uri: string
  name?: string
  description?: string
  mimeType?: string
}

/**
 * Parses tool JSON from ToolEntry, handling both string and object formats
 */
export function parseToolJson(tool: ToolEntry): ToolSchema {
  try {
    const parsed = typeof tool.toolJson === 'string'
      ? JSON.parse(tool.toolJson)
      : tool.toolJson
    return { name: tool.name, ...parsed }
  }
  catch {
    return { name: tool.name }
  }
}

/**
 * Parses resource JSON from ResourceEntry, handling both string and object formats
 */
export function parseResourceJson(resource: ResourceEntry): ResourceSchema {
  try {
    const parsed = typeof resource.resourceJson === 'string'
      ? JSON.parse(resource.resourceJson)
      : resource.resourceJson
    return { uri: resource.uri, ...parsed }
  }
  catch {
    return { uri: resource.uri }
  }
}

/**
 * Extracts description from tool schema
 */
export function parseToolDescription(tool: ToolEntry): string | undefined {
  const schema = parseToolJson(tool)
  return schema.description
}
