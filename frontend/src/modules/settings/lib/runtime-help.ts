// Input: FieldHelpContent type
// Output: Runtime settings field help content map
// Position: Guidance copy for runtime settings UI

import type { FieldHelpContent } from '@/components/common/field-help'

export const RUNTIME_FIELD_HELP: Record<string, FieldHelpContent> = {
  bootstrapMode: {
    id: 'bootstrapMode',
    title: 'Bootstrap mode',
    summary: 'Controls how metadata is collected during startup.',
    details: 'Metadata mode loads tools and resources without keeping instances warm.',
  },
  defaultActivationMode: {
    id: 'defaultActivationMode',
    title: 'Default activation mode',
    summary: 'Applied when a server does not specify activationMode.',
  },
  routeTimeoutSeconds: {
    id: 'routeTimeoutSeconds',
    title: 'Route timeout',
    summary: 'Maximum time to wait for routing requests.',
  },
  pingIntervalSeconds: {
    id: 'pingIntervalSeconds',
    title: 'Ping interval',
    summary: 'Interval for server health checks. Set to 0 to disable.',
  },
  toolRefreshSeconds: {
    id: 'toolRefreshSeconds',
    title: 'Tool refresh interval',
    summary: 'How often to refresh tool lists from servers.',
  },
  bootstrapConcurrency: {
    id: 'bootstrapConcurrency',
    title: 'Bootstrap concurrency',
    summary: 'How many servers can initialize in parallel.',
  },
  bootstrapTimeoutSeconds: {
    id: 'bootstrapTimeoutSeconds',
    title: 'Bootstrap timeout',
    summary: 'Maximum time for server initialization during bootstrap.',
  },
  toolRefreshConcurrency: {
    id: 'toolRefreshConcurrency',
    title: 'Tool refresh concurrency',
    summary: 'Parallel tool refresh operations limit.',
  },
  clientCheckSeconds: {
    id: 'clientCheckSeconds',
    title: 'Client check interval',
    summary: 'How often to check for inactive clients.',
  },
  clientInactiveSeconds: {
    id: 'clientInactiveSeconds',
    title: 'Client inactive threshold',
    summary: 'Time before marking a client as inactive.',
  },
  serverInitRetryBaseSeconds: {
    id: 'serverInitRetryBaseSeconds',
    title: 'Init retry base delay',
    summary: 'Initial delay for server initialization retry.',
  },
  serverInitRetryMaxSeconds: {
    id: 'serverInitRetryMaxSeconds',
    title: 'Init retry max delay',
    summary: 'Maximum delay for server initialization retry.',
  },
  serverInitMaxRetries: {
    id: 'serverInitMaxRetries',
    title: 'Init max retries',
    summary: 'Maximum retry attempts for server initialization.',
  },
  exposeTools: {
    id: 'exposeTools',
    title: 'Expose tools',
    summary: 'Controls whether tools are visible to external clients.',
  },
  toolNamespaceStrategy: {
    id: 'toolNamespaceStrategy',
    title: 'Tool namespace strategy',
    summary: 'Controls how tool names are namespaced across servers.',
  },
}
