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
  httpProxyMode: 'server-http-proxy-mode',
  httpProxyUrl: 'server-http-proxy-url',
  httpProxyNoProxy: 'server-http-proxy-no-proxy',
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
    httpProxyUrl: 'socks5://127.0.0.1:1080',
    httpProxyNoProxy: 'localhost,127.0.0.1',
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
    httpProxyMode: 'How this server resolves proxies',
    httpProxyUrl: 'Proxy URL used when mode is custom',
    httpProxyNoProxy: 'Comma-separated hosts to bypass the proxy',
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
    httpProxyMode: 'Select proxy mode',
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
  httpProxyMode: [
    { value: 'inherit', label: 'Inherit' },
    { value: 'system', label: 'System' },
    { value: 'custom', label: 'Custom' },
    { value: 'disabled', label: 'Disabled' },
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
  proxyUrlRequired: 'Proxy URL is required when proxy mode is custom.',
  minZero: 'Value must be 0 or greater.',
  minOne: 'Value must be 1 or greater.',
}

export const SERVER_FIELD_HELP: Record<string, FieldHelpContent> = {
  name: {
    id: 'name',
    title: 'Server name',
    summary: 'Unique identifier used in routing and tool names.',
    details: 'Used in logs and as the server namespace when tools are prefixed. Names must be unique and cannot be changed after creation.',
    tips: [
      'Use short, stable, lowercase names.',
    ],
  },
  transport: {
    id: 'transport',
    title: 'Transport type',
    summary: 'Controls how mcpv connects to the server runtime.',
    details: 'stdio launches a local process and speaks MCP over stdin/stdout. streamable_http connects to an external HTTP endpoint and does not manage its lifecycle.',
    tips: [
      'stdio for local binaries.',
      'streamable_http for hosted services.',
    ],
  },
  cmd: {
    id: 'cmd',
    title: 'Command',
    summary: 'Executable used for stdio servers.',
    details: 'Required for stdio. This becomes the first entry in the command array; arguments are configured separately.',
    tips: [
      'Prefer absolute paths for reliability.',
    ],
  },
  args: {
    id: 'args',
    title: 'Arguments',
    summary: 'Optional command arguments, comma separated.',
    details: 'Comma-separated list appended to cmd. Whitespace is trimmed.',
    tips: [
      'Do not include arguments in the Command field.',
    ],
  },
  cwd: {
    id: 'cwd',
    title: 'Working directory',
    summary: 'Directory used as the process working directory.',
    details: 'Only applies to stdio. Leave empty to use the mcpv working directory.',
    tips: [
      'Use an absolute path to avoid ambiguity.',
    ],
  },
  env: {
    id: 'env',
    title: 'Environment variables',
    summary: 'Key/value pairs injected into the process environment.',
    details: 'One KEY=value per line. Empty lines are ignored and later keys overwrite earlier ones.',
  },
  endpoint: {
    id: 'endpoint',
    title: 'Endpoint URL',
    summary: 'Streamable HTTP endpoint for the MCP server.',
    details: 'Must be a valid http(s) URL. The remote server must already be running and support MCP Streamable HTTP.',
  },
  httpMaxRetries: {
    id: 'httpMaxRetries',
    title: 'Max retries',
    summary: 'How many times to retry failed HTTP requests.',
    details: '0 uses the default retry policy (5). Higher values help with flaky networks.',
    tips: [
      'Use 1 to fail fast when upstreams are down.',
    ],
  },
  httpHeaders: {
    id: 'httpHeaders',
    title: 'HTTP headers',
    summary: 'Headers included on every HTTP request.',
    details: 'One Header-Name=value per line. Reserved headers such as Content-Type, Accept, and MCP-Protocol-Version cannot be overridden.',
    tips: [
      'Use Authorization for bearer tokens.',
    ],
  },
  httpProxyMode: {
    id: 'httpProxyMode',
    title: 'Proxy mode',
    summary: 'Controls how this server resolves proxy settings.',
    tips: [
      'Inherit: use the global runtime proxy.',
      'System: use environment variables (HTTP_PROXY/HTTPS_PROXY/NO_PROXY).',
      'Custom: use the proxy URL below for this server.',
      'Disabled: bypass proxies entirely.',
    ],
  },
  httpProxyUrl: {
    id: 'httpProxyUrl',
    title: 'Proxy URL',
    summary: 'Explicit proxy URL for this server.',
    details: 'Supports http, https, socks5, and socks5h URLs. Required when mode is custom.',
  },
  httpProxyNoProxy: {
    id: 'httpProxyNoProxy',
    title: 'No proxy list',
    summary: 'Hosts that should bypass the proxy.',
    details: 'Comma-separated list of hosts, IPs, or CIDRs.',
  },
  tags: {
    id: 'tags',
    title: 'Tags',
    summary: 'Tags control visibility and client routing.',
    details: 'Clients only see servers that share at least one tag. If either side has no tags, the server is visible to all.',
    tips: [
      'Tags are normalized to lowercase and de-duplicated.',
    ],
  },
  activationMode: {
    id: 'activationMode',
    title: 'Activation mode',
    summary: 'On-demand starts when traffic arrives. Always-on keeps instances warm.',
    details: 'On-demand only runs when clients are active. Always-on keeps a warm pool based on minReady (at least one instance).',
    tips: [
      'Use always-on for latency-sensitive or background services.',
    ],
  },
  strategy: {
    id: 'strategy',
    title: 'Strategy',
    summary: 'Stateless routes to any instance. Stateful keeps sticky sessions.',
    details: 'Stateful binds the same routing key to one instance until the session TTL expires.',
    tips: [
      'Use stateful when tools maintain per-client state.',
    ],
  },
  drainTimeoutSeconds: {
    id: 'drainTimeoutSeconds',
    title: 'Drain timeout',
    summary: 'Grace period to finish in-flight work before shutdown.',
    details: 'Applied during stop or reload. 0 uses the default (30s).',
    tips: [
      'Increase for long-running tool calls.',
    ],
  },
  idleSeconds: {
    id: 'idleSeconds',
    title: 'Idle timeout',
    summary: 'Time before idle instances are shut down.',
    details: 'Instances above minReady are drained after this period of inactivity. When minReady is 0, idle instances are drained immediately.',
    tips: [
      'Higher values reduce cold starts but keep resources allocated.',
    ],
  },
  maxConcurrent: {
    id: 'maxConcurrent',
    title: 'Max concurrency',
    summary: 'Maximum in-flight requests per instance.',
    details: 'Stateless servers can start new instances when this limit is reached. Stateful servers may return busy until a slot frees.',
  },
  minReady: {
    id: 'minReady',
    title: 'Min ready',
    summary: 'Minimum ready instances kept while active.',
    details: 'On-demand with 0 scales to zero when idle. Always-on keeps at least one instance warm even if minReady is 0.',
    tips: [
      'Increase to reduce cold starts under bursty traffic.',
    ],
  },
  sessionTTLSeconds: {
    id: 'sessionTTLSeconds',
    title: 'Session TTL',
    summary: 'How long stateful bindings remain valid.',
    details: 'Only used for stateful strategy. 0 keeps bindings forever; positive values allow rebinding after expiry.',
    tips: [
      'Set a TTL if clients can reconnect to different instances.',
    ],
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
    message: 'Idle timeout is set to 0. Instances above minReady drain immediately after becoming idle.',
    when: values => values.idleSeconds === 0,
  },
  {
    id: 'always-on-min-ready',
    severity: 'warning',
    message: 'Always-on keeps at least one instance warm. Increase minReady for more warm capacity.',
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
    message: 'Drain timeout is 0. The default (30s) will be used.',
    when: values => values.drainTimeoutSeconds === 0,
  },
]
