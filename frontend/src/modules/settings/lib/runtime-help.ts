// Input: FieldHelpContent type
// Output: Runtime settings field help content map
// Position: Guidance copy for runtime settings UI

import type { FieldHelpContent } from '@/components/common/field-help'

export const RUNTIME_FIELD_HELP: Record<string, FieldHelpContent> = {
  bootstrapMode: {
    id: 'bootstrapMode',
    title: 'Bootstrap mode',
    summary: 'Controls whether metadata is collected during startup.',
    details: 'Metadata mode temporarily starts servers to fetch tools/resources/prompts and cache them. It does not keep instances warm unless activation policy requires it.',
    tips: [
      'Disable to shorten startup when servers are slow or unreachable.',
      'Use activation mode and minReady to keep instances warm.',
    ],
  },
  defaultActivationMode: {
    id: 'defaultActivationMode',
    title: 'Default activation mode',
    summary: 'Applied when a server does not specify activationMode.',
    details: 'On-demand only runs servers when clients are active. Always-on keeps a warm pool even without clients.',
    tips: [
      'Override per server when behavior needs to differ.',
      'Always-on keeps at least one instance warm even if minReady is 0.',
    ],
  },
  routeTimeoutSeconds: {
    id: 'routeTimeoutSeconds',
    title: 'Route timeout',
    summary: 'Maximum time to wait for a single routed request.',
    details: 'Applies to tool calls, resource reads, and prompt requests. Must be greater than 0.',
    tips: [
      'Increase for long-running tools.',
      'Shorter timeouts surface hung servers sooner.',
    ],
  },
  pingIntervalSeconds: {
    id: 'pingIntervalSeconds',
    title: 'Ping interval',
    summary: 'Interval for server health checks.',
    details: 'Set to 0 to disable pings. Smaller values detect failures faster but add overhead.',
  },
  toolRefreshSeconds: {
    id: 'toolRefreshSeconds',
    title: 'Tool refresh interval',
    summary: 'How often to refresh tool lists from servers.',
    details: 'Set to 0 to disable periodic refresh. List-change notifications still trigger refresh when supported.',
    tips: [
      'Increase for stable servers to reduce load.',
    ],
  },
  bootstrapConcurrency: {
    id: 'bootstrapConcurrency',
    title: 'Bootstrap concurrency',
    summary: 'How many servers can initialize in parallel.',
    details: '0 uses the default (3). Higher values speed startup but can spike CPU and memory.',
  },
  bootstrapTimeoutSeconds: {
    id: 'bootstrapTimeoutSeconds',
    title: 'Bootstrap timeout',
    summary: 'Maximum time for server initialization during bootstrap.',
    details: '0 uses the default (30s). This only affects metadata bootstrap, not routing.',
  },
  toolRefreshConcurrency: {
    id: 'toolRefreshConcurrency',
    title: 'Tool refresh concurrency',
    summary: 'Parallel tool refresh operations limit.',
    details: '0 uses the default (4). Higher values increase concurrent HTTP or stdio load.',
  },
  clientCheckSeconds: {
    id: 'clientCheckSeconds',
    title: 'Client check interval',
    summary: 'How often to check for inactive clients.',
    details: 'Must be greater than 0. Used to detect dead clients and release their activations.',
  },
  clientInactiveSeconds: {
    id: 'clientInactiveSeconds',
    title: 'Client inactive threshold',
    summary: 'Time before marking a client as inactive.',
    details: 'Must be greater than 0 and typically larger than the client check interval.',
  },
  serverInitRetryBaseSeconds: {
    id: 'serverInitRetryBaseSeconds',
    title: 'Init retry base delay',
    summary: 'Initial delay for server initialization retry.',
    details: 'Must be greater than 0. Retries use exponential backoff starting at this delay.',
  },
  serverInitRetryMaxSeconds: {
    id: 'serverInitRetryMaxSeconds',
    title: 'Init retry max delay',
    summary: 'Maximum delay for server initialization retry.',
    details: 'Must be greater than or equal to the base delay. Caps the backoff.',
  },
  serverInitMaxRetries: {
    id: 'serverInitMaxRetries',
    title: 'Init max retries',
    summary: 'Maximum retry attempts for server initialization.',
    details: '0 means no retry cap (retry indefinitely). Values above 0 suspend after the limit.',
  },
  exposeTools: {
    id: 'exposeTools',
    title: 'Expose tools',
    summary: 'Controls whether tools are visible to external clients.',
    details: 'When disabled, tools/resources/prompts are hidden and refresh work is paused.',
  },
  toolNamespaceStrategy: {
    id: 'toolNamespaceStrategy',
    title: 'Tool namespace strategy',
    summary: 'Controls how tool names are namespaced across servers.',
    details: 'Prefix uses serverName.toolName to avoid collisions. Flat exposes raw tool names.',
    tips: [
      'Use prefix when multiple servers expose similarly named tools.',
    ],
  },
  observabilityListenAddress: {
    id: 'observabilityListenAddress',
    title: 'Observability listen address',
    summary: 'Bind address for metrics and health endpoints.',
    details: 'Applies to /metrics and /healthz. Use host:port, for example 0.0.0.0:9090.',
    tips: [
      'Use 127.0.0.1 for local-only access.',
      'Ensure the port is free before enabling endpoints.',
    ],
  },
  observabilityMetricsEnabled: {
    id: 'observabilityMetricsEnabled',
    title: 'Metrics endpoint',
    summary: 'Expose Prometheus metrics at /metrics.',
    details: 'Enable this to export runtime metrics to Prometheus and Grafana.',
  },
  observabilityHealthzEnabled: {
    id: 'observabilityHealthzEnabled',
    title: 'Healthz endpoint',
    summary: 'Expose health checks at /healthz.',
    details: 'Enable this to monitor core loop health and receive a JSON report.',
  },
}
