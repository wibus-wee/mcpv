// Input: SWR key definitions
// Output: Centralized SWR cache keys for consistent data fetching
// Position: Infrastructure layer - provides type-safe SWR key constants

/**
 * Centralized SWR cache keys to ensure consistency across hooks and event handlers.
 * This prevents silent failures when hook keys are modified without updating event listeners.
 *
 * @example
 * // In hooks
 * useSWR(swrKeys.servers, fetcher, config)
 *
 * // In event handlers
 * mutate(swrKeys.runtimeStatus, data, { revalidate: false })
 */
export const swrKeys = {
  // Server-related keys
  servers: 'servers',
  serverGroups: 'server-groups',
  server: 'server',
  serverDetails: 'server-details',
  runtimeStatus: 'runtime-status',
  serverInitStatus: 'server-init-status',

  // Client-related keys
  activeClients: 'active-clients',

  // Config-related keys
  configMode: 'config-mode',

  // Discovery-related keys
  tools: 'tools',
  resources: 'resources',
  prompts: 'prompts',

  // Core-related keys
  appInfo: 'app-info',
  coreState: 'core-state',
  bootstrapProgress: 'bootstrap-progress',
  subAgentConfig: 'subagent-config',

  // Log-related keys
  logs: 'logs',

  // Plugin-related keys
  pluginList: 'plugin-list',
  pluginMetrics: 'plugin-metrics',
} as const

export type SwrKey = typeof swrKeys[keyof typeof swrKeys]
