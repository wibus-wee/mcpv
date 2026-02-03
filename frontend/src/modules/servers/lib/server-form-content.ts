// Input: FieldHelpContent type
// Output: Server form copy, help content, and advice rules
// Position: Domain copy and guidance for server configuration UI

import type { FieldHelpContent } from '@/components/common/field-help'

export const SERVER_FIELD_IDS = {
  name: 'server-name',
  transport: 'server-transport',
  cmd: 'server-cmd',
  args: 'server-args',
  cwd: 'server-cwd',
  env: 'server-env',
  endpoint: 'server-endpoint',
  httpMaxRetries: 'server-http-max-retries',
  httpHeaders: 'server-http-headers',
  tags: 'server-tags',
  activationMode: 'server-activation-mode',
  strategy: 'server-strategy',
  drainTimeoutSeconds: 'server-drain-timeout',
  idleSeconds: 'server-idle-timeout',
  maxConcurrent: 'server-max-concurrent',
  minReady: 'server-min-ready',
  sessionTTLSeconds: 'server-session-ttl',
}

export const SERVER_FORM_TEXT = {
  badges: {
    stdio: 'stdio',
    streamableHttp: 'streamable_http',
  },
  transportSummaries: {
    stdio: 'Configure command execution',
    streamableHttp: 'Configure HTTP endpoint',
  },
  placeholders: {
    name: 'my-server',
    cmd: '/usr/bin/node',
    args: 'server.js, --port, 3000',
    cwd: '/path/to/project',
    env: 'NODE_ENV=production\nAPI_KEY=secret',
    endpoint: 'http://localhost:3000/mcp',
    httpHeaders: 'Authorization=Bearer token\nContent-Type=application/json',
    tags: 'production, api, core',
  },
  descriptions: {
    cmd: 'Executable path or command',
    args: 'Comma-separated command arguments',
    cwd: 'Execution directory',
    env: 'One per line: KEY=value',
    endpoint: 'HTTP endpoint for the MCP server',
    httpMaxRetries: 'Maximum number of retries for HTTP requests',
    httpHeaders: 'One per line: Header-Name=value',
    tags: 'Comma-separated tags for organization',
    drainTimeoutSeconds: 'Drain timeout in seconds',
    idleSeconds: 'Seconds before idle shutdown',
    maxConcurrent: 'Maximum concurrent requests',
    minReady: 'Minimum ready instances',
    sessionTTLSeconds: 'Session time-to-live in seconds',
  },
  selectPlaceholders: {
    transport: 'Select transport',
    activationMode: 'Select mode',
    strategy: 'Select strategy',
  },
  advanced: {
    title: 'Advanced settings',
    description: 'Less common tuning and lifecycle controls.',
    toggleLabel: 'Advanced',
  },
  advice: {
    title: 'Recommendations',
  },
}

export const SERVER_SELECT_OPTIONS = {
  transport: [
    { value: 'stdio', label: 'stdio' },
    { value: 'streamable_http', label: 'streamable_http' },
  ],
  activationMode: [
    { value: 'on-demand', label: 'On Demand' },
    { value: 'always-on', label: 'Always On' },
  ],
  strategy: [
    { value: 'stateless', label: 'Stateless' },
    { value: 'stateful', label: 'Stateful' },
  ],
}

export const SERVER_FORM_VALIDATION = {
  nameRequired: 'Server name is required.',
  cmdRequired: 'Command is required for stdio transport.',
  endpointRequired: 'Endpoint is required for streamable_http transport.',
  minZero: 'Value must be 0 or greater.',
  minOne: 'Value must be 1 or greater.',
}

export const SERVER_FIELD_HELP: Record<string, FieldHelpContent> = {
  name: {
    id: 'name',
    title: 'Server name',
    summary: 'Unique identifier used in routing and tool names.',
  },
  transport: {
    id: 'transport',
    title: 'Transport type',
    summary: 'Controls how mcpv connects to the server runtime.',
    details: 'stdio launches a local process. streamable_http connects to an external HTTP endpoint.',
  },
  cmd: {
    id: 'cmd',
    title: 'Command',
    summary: 'Executable used for stdio servers.',
  },
  args: {
    id: 'args',
    title: 'Arguments',
    summary: 'Optional command arguments, comma separated.',
  },
  cwd: {
    id: 'cwd',
    title: 'Working directory',
    summary: 'Directory used as the process working directory.',
  },
  env: {
    id: 'env',
    title: 'Environment variables',
    summary: 'Key/value pairs injected into the process environment.',
  },
  endpoint: {
    id: 'endpoint',
    title: 'Endpoint URL',
    summary: 'Streamable HTTP endpoint for the MCP server.',
  },
  httpMaxRetries: {
    id: 'httpMaxRetries',
    title: 'Max retries',
    summary: 'How many times to retry failed HTTP requests.',
  },
  httpHeaders: {
    id: 'httpHeaders',
    title: 'HTTP headers',
    summary: 'Headers included on every HTTP request.',
  },
  tags: {
    id: 'tags',
    title: 'Tags',
    summary: 'Tags control visibility and client routing.',
    details: 'Clients only see servers that match their tag set.',
  },
  activationMode: {
    id: 'activationMode',
    title: 'Activation mode',
    summary: 'On demand starts when traffic arrives. Always on keeps instances warm.',
  },
  strategy: {
    id: 'strategy',
    title: 'Strategy',
    summary: 'Stateless routes to any instance. Stateful keeps sticky sessions.',
  },
  drainTimeoutSeconds: {
    id: 'drainTimeoutSeconds',
    title: 'Drain timeout',
    summary: 'Grace period to finish in-flight work before shutdown.',
  },
  idleSeconds: {
    id: 'idleSeconds',
    title: 'Idle timeout',
    summary: 'Time before idle instances are shut down. 0 disables idle shutdown.',
  },
  maxConcurrent: {
    id: 'maxConcurrent',
    title: 'Max concurrency',
    summary: 'Maximum in-flight requests per instance.',
  },
  minReady: {
    id: 'minReady',
    title: 'Min ready',
    summary: 'Minimum ready instances kept while active.',
  },
  sessionTTLSeconds: {
    id: 'sessionTTLSeconds',
    title: 'Session TTL',
    summary: 'How long stateful bindings remain valid.',
  },
}

export type ServerFormValues = {
  transport: 'stdio' | 'streamable_http'
  activationMode: 'on-demand' | 'always-on'
  idleSeconds: number
  maxConcurrent: number
  minReady: number
  strategy: string
  sessionTTLSeconds: number
  drainTimeoutSeconds: number
}

export type ServerAdviceRule = {
  id: string
  severity: 'info' | 'warning'
  message: string
  when: (values: ServerFormValues) => boolean
}

export const SERVER_ADVICE_RULES: ServerAdviceRule[] = [
  {
    id: 'idle-disabled',
    severity: 'info',
    message: 'Idle timeout is set to 0. Instances will stay running after activation.',
    when: values => values.idleSeconds === 0,
  },
  {
    id: 'always-on-min-ready',
    severity: 'warning',
    message: 'Always-on keeps the server active. Consider setting minReady to 1 or more.',
    when: values => values.activationMode === 'always-on' && values.minReady === 0,
  },
  {
    id: 'stateful-no-ttl',
    severity: 'warning',
    message: 'Stateful strategy with session TTL 0 keeps bindings forever.',
    when: values => values.strategy === 'stateful' && values.sessionTTLSeconds === 0,
  },
  {
    id: 'drain-zero',
    severity: 'info',
    message: 'Drain timeout is 0. In-flight work may be terminated immediately on shutdown.',
    when: values => values.drainTimeoutSeconds === 0,
  },
]
