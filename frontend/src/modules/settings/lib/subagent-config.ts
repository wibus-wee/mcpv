import type { SubAgentConfigDetail } from '@bindings/mcpv/internal/ui/types'

export const SUBAGENT_PROVIDER_OPTIONS = [
  { value: 'openai', label: 'OpenAI' },
] as const

export const DEFAULT_SUBAGENT_FORM: SubAgentConfigDetail = {
  enabledTags: [],
  model: '',
  provider: 'openai',
  apiKeyEnvVar: '',
  baseURL: '',
  maxToolsPerRequest: 0,
  filterPrompt: '',
}

export const toSubAgentFormState = (config: SubAgentConfigDetail): SubAgentConfigDetail => ({
  enabledTags: config.enabledTags ?? [],
  model: config.model,
  provider: config.provider || 'openai',
  apiKeyEnvVar: config.apiKeyEnvVar,
  baseURL: config.baseURL,
  maxToolsPerRequest: config.maxToolsPerRequest,
  filterPrompt: config.filterPrompt,
})
