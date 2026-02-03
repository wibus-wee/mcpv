// Input: FieldHelpContent type
// Output: SubAgent settings field help content map
// Position: Guidance copy for SubAgent settings UI

import type { FieldHelpContent } from '@/components/common/field-help'

export const SUBAGENT_FIELD_HELP: Record<string, FieldHelpContent> = {
  provider: {
    id: 'provider',
    title: 'Provider',
    summary: 'LLM provider used for tool filtering.',
    details: 'Currently only OpenAI is supported. The provider determines model options and request format.',
  },
  model: {
    id: 'model',
    title: 'Model',
    summary: 'Model identifier used by the SubAgent.',
    details: 'Must support tool calling. Fetch models to populate the list or enter a compatible ID.',
    tips: [
      'Smaller models are faster and cheaper but may filter less accurately.',
    ],
  },
  apiKey: {
    id: 'apiKey',
    title: 'API key',
    summary: 'Inline API key stored in config.',
    details: 'If empty, mcpv reads the key from apiKeyEnvVar at startup.',
    tips: [
      'Prefer env vars to avoid committing secrets.',
    ],
  },
  apiKeyEnvVar: {
    id: 'apiKeyEnvVar',
    title: 'API key env var',
    summary: 'Environment variable name used when no inline key is set.',
    details: 'Required when apiKey is empty. The environment must be set where mcpv runs.',
  },
  baseURL: {
    id: 'baseURL',
    title: 'Base URL',
    summary: 'Optional override for the provider endpoint.',
    details: 'Use for OpenAI-compatible gateways or proxies. Include the full base path (e.g. /v1).',
  },
  enabledTags: {
    id: 'enabledTags',
    title: 'Enabled tags',
    summary: 'Limit SubAgent filtering to specific client tags.',
    details: 'If empty, SubAgent applies to all clients. Tags are normalized to lowercase.',
    tips: [
      'Use tags to enable SubAgent only for selected IDEs or workflows.',
    ],
  },
  maxToolsPerRequest: {
    id: 'maxToolsPerRequest',
    title: 'Max tools per request',
    summary: 'Upper bound on tool candidates per query.',
    details: '0 or negative uses the default cap (50). Lower values reduce token usage.',
  },
  filterPrompt: {
    id: 'filterPrompt',
    title: 'Filter prompt',
    summary: 'Optional prompt override for tool filtering.',
    details: 'Overrides the default system prompt used to select tools for a query.',
    tips: [
      'Keep it short and deterministic, then test with real queries.',
    ],
  },
}
