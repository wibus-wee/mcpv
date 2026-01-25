import type { SubAgentConfigDetail } from '@bindings/mcpd/internal/ui'

export const SUBAGENT_PROVIDER_OPTIONS = [
  { value: 'openai', label: 'OpenAI' },
] as const

export type SubAgentFormState = {
  enabledTags: string[]
  model: string
  provider: string
  apiKeyEnvVar: string
  baseURL: string
  maxToolsPerRequest: number
  filterPrompt: string
}

export const DEFAULT_SUBAGENT_FORM: SubAgentFormState = {
  enabledTags: [],
  model: '',
  provider: 'openai',
  apiKeyEnvVar: '',
  baseURL: '',
  maxToolsPerRequest: 0,
  filterPrompt: '',
}

export const toSubAgentFormState = (config: SubAgentConfigDetail): SubAgentFormState => ({
  enabledTags: config.enabledTags ?? [],
  model: config.model,
  provider: config.provider || 'openai',
  apiKeyEnvVar: config.apiKeyEnvVar,
  baseURL: config.baseURL,
  maxToolsPerRequest: config.maxToolsPerRequest,
  filterPrompt: config.filterPrompt,
})
