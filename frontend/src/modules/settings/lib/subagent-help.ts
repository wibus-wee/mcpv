// Input: FieldHelpContent type
// Output: SubAgent settings field help content map
// Position: Guidance copy for SubAgent settings UI

import type { FieldHelpContent } from '@/components/common/field-help'

export const SUBAGENT_FIELD_HELP: Record<string, FieldHelpContent> = {
  provider: {
    id: 'provider',
    title: 'Provider',
    summary: 'LLM provider used for tool filtering.',
  },
  model: {
    id: 'model',
    title: 'Model',
    summary: 'Model identifier used by the SubAgent.',
    details: 'Fetch available models from the provider to populate the list.',
  },
  apiKey: {
    id: 'apiKey',
    title: 'API key',
    summary: 'Inline API key used by the SubAgent.',
    details: 'Use the env var field if you do not want to store a key in config.',
  },
  apiKeyEnvVar: {
    id: 'apiKeyEnvVar',
    title: 'API key env var',
    summary: 'Environment variable name used when no inline key is set.',
  },
  baseURL: {
    id: 'baseURL',
    title: 'Base URL',
    summary: 'Optional override for the provider endpoint.',
  },
  enabledTags: {
    id: 'enabledTags',
    title: 'Enabled tags',
    summary: 'Limit SubAgent filtering to specific client tags.',
  },
  maxToolsPerRequest: {
    id: 'maxToolsPerRequest',
    title: 'Max tools per request',
    summary: 'Upper bound on tool candidates per query.',
  },
  filterPrompt: {
    id: 'filterPrompt',
    title: 'Filter prompt',
    summary: 'Optional prompt override for tool filtering.',
  },
}
