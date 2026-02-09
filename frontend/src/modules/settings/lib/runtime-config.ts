import type { RuntimeConfigDetail } from '@bindings/mcpv/internal/ui/types'

export const BOOTSTRAP_MODE_OPTIONS = [
  { value: 'metadata', label: 'Metadata' },
  { value: 'disabled', label: 'Disabled' },
] as const

export const ACTIVATION_MODE_OPTIONS = [
  { value: 'on-demand', label: 'On-demand' },
  { value: 'always-on', label: 'Always-on' },
] as const

export const NAMESPACE_STRATEGY_OPTIONS = [
  { value: 'prefix', label: 'Prefix' },
  { value: 'flat', label: 'Flat' },
] as const

export const BOOTSTRAP_MODE_LABELS: Record<string, string> = {
  metadata: 'Metadata',
  disabled: 'Disabled',
}

export const ACTIVATION_MODE_LABELS: Record<string, string> = {
  'on-demand': 'On-demand',
  'always-on': 'Always-on',
}

export const NAMESPACE_STRATEGY_LABELS: Record<string, string> = {
  prefix: 'Prefix',
  flat: 'Flat',
}

export type RuntimeFormState = {
  routeTimeoutSeconds: number
  pingIntervalSeconds: number
  toolRefreshSeconds: number
  toolRefreshConcurrency: number
  clientCheckSeconds: number
  clientInactiveSeconds: number
  serverInitRetryBaseSeconds: number
  serverInitRetryMaxSeconds: number
  serverInitMaxRetries: number
  reloadMode: string
  bootstrapMode: string
  bootstrapConcurrency: number
  bootstrapTimeoutSeconds: number
  defaultActivationMode: string
  exposeTools: boolean
  toolNamespaceStrategy: string
}

export const DEFAULT_RUNTIME_FORM: RuntimeFormState = {
  routeTimeoutSeconds: 0,
  pingIntervalSeconds: 0,
  toolRefreshSeconds: 0,
  toolRefreshConcurrency: 0,
  clientCheckSeconds: 0,
  clientInactiveSeconds: 0,
  serverInitRetryBaseSeconds: 0,
  serverInitRetryMaxSeconds: 0,
  serverInitMaxRetries: 0,
  reloadMode: 'lenient',
  bootstrapMode: 'metadata',
  bootstrapConcurrency: 0,
  bootstrapTimeoutSeconds: 0,
  defaultActivationMode: 'on-demand',
  exposeTools: false,
  toolNamespaceStrategy: 'prefix',
}

export const toRuntimeFormState = (runtime: RuntimeConfigDetail): RuntimeFormState => ({
  routeTimeoutSeconds: runtime.routeTimeoutSeconds,
  pingIntervalSeconds: runtime.pingIntervalSeconds,
  toolRefreshSeconds: runtime.toolRefreshSeconds,
  toolRefreshConcurrency: runtime.toolRefreshConcurrency,
  clientCheckSeconds: runtime.clientCheckSeconds,
  clientInactiveSeconds: runtime.clientInactiveSeconds,
  serverInitRetryBaseSeconds: runtime.serverInitRetryBaseSeconds,
  serverInitRetryMaxSeconds: runtime.serverInitRetryMaxSeconds,
  serverInitMaxRetries: runtime.serverInitMaxRetries,
  reloadMode: runtime.reloadMode || 'hot',
  bootstrapMode: runtime.bootstrapMode || 'metadata',
  bootstrapConcurrency: runtime.bootstrapConcurrency,
  bootstrapTimeoutSeconds: runtime.bootstrapTimeoutSeconds,
  defaultActivationMode: runtime.defaultActivationMode || 'on-demand',
  exposeTools: runtime.exposeTools,
  toolNamespaceStrategy: runtime.toolNamespaceStrategy || 'prefix',
})
