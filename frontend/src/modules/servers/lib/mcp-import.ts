export type ImportServerDraft = {
  id: string
  name: string
  transport: 'stdio' | 'streamable_http'
  cmd: string[]
  env: Record<string, string>
  cwd: string
  http?: StreamableHTTPDraft
  source?: 'mcpServers' | 'command' | 'url' | 'httpJson'
}

export type StreamableHTTPDraft = {
  endpoint: string
  headers: Record<string, string>
  maxRetries: number
  proxy?: ProxyDraft
}

export type ProxyDraft = {
  mode: string
  url: string
  noProxy: string
}

type McpServerEntry = {
  command?: unknown
  args?: unknown
  env?: unknown
  cwd?: unknown
  transport?: unknown
  type?: unknown
  endpoint?: unknown
  url?: unknown
  headers?: unknown
  maxRetries?: unknown
  proxy?: unknown
}

type ParseResult = {
  servers: ImportServerDraft[]
  errors: string[]
}

/**
 * 自动检测输入是 JSON 还是命令行，并调用相应的解析器
 */
export function parseMcpServersJson(input: string): ParseResult {
  const trimmed = input.trim()
  if (!trimmed) {
    return { servers: [], errors: ['Paste JSON, a streamable HTTP endpoint, or a command line to continue.'] }
  }

  if (looksLikeUrl(trimmed)) {
    return parseUrlInput(trimmed)
  }

  // 尝试检测是否为 JSON
  if (isJsonInput(trimmed)) {
    return parseJsonPayload(trimmed)
  }

  // 否则尝试作为命令行解析
  return parseCommandLine(trimmed)
}

/**
 * 检测输入是否看起来像 JSON
 */
function isJsonInput(input: string): boolean {
  const trimmed = input.trim()
  // JSON 必须以 { 或 [ 开头
  return trimmed.startsWith('{') || trimmed.startsWith('[')
}

/**
 * 解析 JSON 格式的 mcpServers 配置
 */
function parseJsonPayload(input: string): ParseResult {
  let payload: unknown
  try {
    payload = JSON.parse(input)
  }
  catch {
    return { servers: [], errors: ['Invalid JSON format.'] }
  }

  if (!isRecord(payload)) {
    return { servers: [], errors: ['JSON must be an object with mcpServers or an endpoint.'] }
  }

  const rawServers = payload.mcpServers
  if (!isRecord(rawServers)) {
    const singleHttp = parseSingleHttpPayload(payload)
    if (singleHttp) {
      return singleHttp
    }
    return { servers: [], errors: ['mcpServers must be an object map.'] }
  }

  const errors: string[] = []
  const servers: ImportServerDraft[] = []

  Object.entries(rawServers).forEach(([name, raw], index) => {
    const prefix = name ? `mcpServers.${name}` : `mcpServers[${index}]`
    if (!name.trim()) {
      errors.push(`${prefix}: server name is required.`)
      return
    }
    if (!isRecord(raw)) {
      errors.push(`${prefix}: entry must be an object.`)
      return
    }

    const entry = raw as McpServerEntry
    const transportRaw = entry.transport ?? entry.type
    const transport = transportRaw === undefined
      ? (hasStreamableHttpFields(entry) ? 'streamable_http' : 'stdio')
      : parseTransport(transportRaw, prefix, errors)
    if (!transport) {
      return
    }

    if (transport === 'streamable_http') {
      const http = parseStreamableHTTP(entry, prefix, errors)
      if (!http) {
        return
      }
      servers.push({
        id: `${index}-${name}`,
        name,
        transport,
        cmd: [],
        env: {},
        cwd: '',
        http,
        source: 'mcpServers',
      })
      return
    }

    const { command } = entry
    if (typeof command !== 'string' || command.trim() === '') {
      errors.push(`${prefix}: command is required.`)
      return
    }

    const args = parseArgs(entry.args, prefix, errors)
    if (args === null) {
      return
    }

    const env = parseEnv(entry.env, prefix, errors)
    if (env === null) {
      return
    }

    const cwd = parseCwd(entry.cwd, prefix, errors)
    if (cwd === null) {
      return
    }

    servers.push({
      id: `${index}-${name}`,
      name,
      transport,
      cmd: [command, ...args],
      env,
      cwd,
      source: 'mcpServers',
    })
  })

  if (servers.length === 0 && errors.length === 0) {
    errors.push('No servers found in mcpServers.')
  }

  if (errors.length > 0) {
    return { servers: [], errors }
  }

  return { servers, errors: [] }
}

function parseTransport(
  raw: unknown,
  prefix: string,
  errors: string[],
): 'stdio' | 'streamable_http' | null {
  if (raw === undefined) {
    return 'stdio'
  }
  if (typeof raw !== 'string') {
    errors.push(`${prefix}: transport must be a string.`)
    return null
  }
  const normalized = raw.trim().toLowerCase()
  if (!normalized) {
    return 'stdio'
  }
  if (normalized === 'stdio') {
    return 'stdio'
  }
  if (normalized === 'streamable_http' || normalized === 'streamable-http' || normalized === 'streamablehttp') {
    return 'streamable_http'
  }
  errors.push(`${prefix}: transport must be stdio or streamable_http.`)
  return null
}

function parseStreamableHTTP(
  entry: McpServerEntry,
  prefix: string,
  errors: string[],
): StreamableHTTPDraft | null {
  if (entry.command || entry.args || entry.cwd || entry.env) {
    errors.push(`${prefix}: streamable_http transport does not support command/args/env/cwd.`)
    return null
  }

  const rawEndpoint = entry.endpoint ?? entry.url
  if (typeof rawEndpoint !== 'string' || rawEndpoint.trim() === '') {
    errors.push(`${prefix}: endpoint is required for streamable_http transport.`)
    return null
  }

  const headers = parseHeaders(entry.headers, prefix, errors)
  if (headers === null) {
    return null
  }

  const maxRetries = parseMaxRetries(entry.maxRetries, prefix, errors)
  if (maxRetries === null) {
    return null
  }

  const proxy = parseProxy(entry.proxy, prefix, errors)
  if (proxy === null) {
    return null
  }

  return {
    endpoint: rawEndpoint.trim(),
    headers,
    maxRetries,
    ...(proxy ? { proxy } : {}),
  }
}

function parseArgs(
  raw: unknown,
  prefix: string,
  errors: string[],
): string[] | null {
  if (raw === undefined) {
    return []
  }
  if (!Array.isArray(raw)) {
    errors.push(`${prefix}: args must be an array of strings.`)
    return null
  }
  const args: string[] = []
  raw.forEach((item, index) => {
    if (typeof item !== 'string') {
      errors.push(`${prefix}: args[${index}] must be a string.`)
      return
    }
    args.push(item)
  })
  return args
}

function parseEnv(
  raw: unknown,
  prefix: string,
  errors: string[],
): Record<string, string> | null {
  if (raw === undefined) {
    return {}
  }
  if (!isRecord(raw)) {
    errors.push(`${prefix}: env must be an object.`)
    return null
  }
  const env: Record<string, string> = {}
  Object.entries(raw).forEach(([key, value]) => {
    if (typeof value !== 'string') {
      errors.push(`${prefix}: env.${key} must be a string.`)
      return
    }
    env[key] = value
  })
  return env
}

function parseHeaders(
  raw: unknown,
  prefix: string,
  errors: string[],
): Record<string, string> | null {
  if (raw === undefined) {
    return {}
  }
  if (!isRecord(raw)) {
    errors.push(`${prefix}: headers must be an object.`)
    return null
  }
  const headers: Record<string, string> = {}
  Object.entries(raw).forEach(([key, value]) => {
    if (typeof value !== 'string') {
      errors.push(`${prefix}: headers.${key} must be a string.`)
      return
    }
    headers[key] = value
  })
  return headers
}

function parseMaxRetries(
  raw: unknown,
  prefix: string,
  errors: string[],
): number | null {
  if (raw === undefined) {
    return 0
  }
  if (typeof raw !== 'number' || Number.isNaN(raw)) {
    errors.push(`${prefix}: maxRetries must be a number.`)
    return null
  }
  return raw
}

function parseProxy(
  raw: unknown,
  prefix: string,
  errors: string[],
): ProxyDraft | null | undefined {
  if (raw === undefined) {
    return undefined
  }
  if (!isRecord(raw)) {
    errors.push(`${prefix}: proxy must be an object.`)
    return null
  }

  const modeRaw = typeof raw.mode === 'string' ? raw.mode.trim().toLowerCase() : ''
  const url = typeof raw.url === 'string' ? raw.url.trim() : ''
  const noProxy = typeof raw.noProxy === 'string' ? raw.noProxy.trim() : ''

  let mode = modeRaw
  if (!mode) {
    mode = url ? 'custom' : 'inherit'
  }

  if (!['inherit', 'custom', 'disabled', 'system'].includes(mode)) {
    errors.push(`${prefix}: proxy.mode must be inherit, custom, disabled, or system.`)
    return null
  }
  if (mode === 'custom' && !url) {
    errors.push(`${prefix}: proxy.url is required when proxy.mode is custom.`)
    return null
  }

  if (!mode && !url && !noProxy) {
    return undefined
  }

  return { mode, url, noProxy }
}

function parseCwd(
  raw: unknown,
  prefix: string,
  errors: string[],
): string | null {
  if (raw === undefined) {
    return ''
  }
  if (typeof raw !== 'string') {
    errors.push(`${prefix}: cwd must be a string.`)
    return null
  }
  return raw
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function looksLikeUrl(input: string): boolean {
  return /^https?:\/\//i.test(input)
}

function hasStreamableHttpFields(entry: McpServerEntry): boolean {
  return typeof entry.endpoint === 'string' || typeof entry.url === 'string'
}

function parseSingleHttpPayload(payload: Record<string, unknown>): ParseResult | null {
  const entry = payload as McpServerEntry
  if (!hasStreamableHttpFields(entry)) {
    return null
  }
  const errors: string[] = []
  const http = parseStreamableHTTP(entry, 'payload', errors)
  if (!http || errors.length > 0) {
    return { servers: [], errors }
  }

  const name = inferServerNameFromUrl(http.endpoint)
  const server: ImportServerDraft = {
    id: `http-${Date.now()}`,
    name,
    transport: 'streamable_http',
    cmd: [],
    env: {},
    cwd: '',
    http,
    source: 'httpJson',
  }
  return { servers: [server], errors: [] }
}

/**
 * 解析命令行格式:
 * npx -y @upstash/context7-mcp --api-key YOUR_API_KEY
 *
 * 生成单个 server，名称从命令推断
 */
function parseCommandLine(input: string): ParseResult {
  const errors: string[] = []

  // 简单的 shell 参数解析
  const tokens = shellParse(input)
  if (tokens.length === 0) {
    errors.push('Command line is empty.')
    return { servers: [], errors }
  }

  // 第一个 token 是命令，其余是参数
  const [command, ...args] = tokens

  // 从命令推断 server 名称
  let name = inferServerName(command, args)
  if (!name) {
    name = command.split(/[/\\]/).pop() || 'unnamed'
  }

  // 规范化名称
  name = normalizeServerName(name)

  const server: ImportServerDraft = {
    id: `cmd-${Date.now()}`,
    name,
    transport: 'stdio',
    cmd: [command, ...args],
    env: {},
    cwd: '',
    source: 'command',
  }

  return { servers: [server], errors: [] }
}

function parseUrlInput(input: string): ParseResult {
  let endpoint: string
  try {
    endpoint = new URL(input).toString()
  }
  catch {
    return { servers: [], errors: ['Invalid URL format.'] }
  }

  const name = inferServerNameFromUrl(endpoint)
  const server: ImportServerDraft = {
    id: `url-${Date.now()}`,
    name,
    transport: 'streamable_http',
    cmd: [],
    env: {},
    cwd: '',
    http: {
      endpoint,
      headers: {},
      maxRetries: 0,
    },
    source: 'url',
  }
  return { servers: [server], errors: [] }
}

/**
 * 简单的 shell 风格参数解析
 * 支持双引号和单引号
 */
function shellParse(input: string): string[] {
  const tokens: string[] = []
  let current = ''
  let inDouble = false
  let inSingle = false

  for (let i = 0; i < input.length; i++) {
    const char = input[i]
    const prev = i > 0 ? input[i - 1] : ''

    if (char === '"' && prev !== '\\' && !inSingle) {
      inDouble = !inDouble
      continue
    }
    if (char === "'" && prev !== '\\' && !inDouble) {
      inSingle = !inSingle
      continue
    }
    if ((char === ' ' || char === '\t') && !inDouble && !inSingle) {
      if (current) {
        tokens.push(current)
        current = ''
      }
      continue
    }

    current += char
  }

  if (current) {
    tokens.push(current)
  }

  return tokens.filter(t => t.length > 0)
}

/**
 * 从命令和参数推断 server 名称
 * 尝试从类似包名的参数中提取
 */
function inferServerName(command: string, args: string[]): string | null {
  // 对于 npx，查找 @scope/package-name 或 package-name 形式的参数
  if (command === 'npx' || command.endsWith('/npx')) {
    for (const arg of args) {
      // 跳过 flag 类参数 (-y, --api-key 等)
      if (arg.startsWith('-')) {
        continue
      }
      // 查找看起来像包名的参数（包含 / 或 - 的字符串）
      if (arg.includes('/') || arg.includes('-') || arg.includes('@')) {
        return arg
      }
    }
  }

  // 对于其他命令，使用命令名本身
  return command.split(/[/\\]/).pop() || null
}

/**
 * 清理并规范化 server 名称
 * 移除特殊字符，保留字母数字和连字符下划线，转换为小写
 */
function normalizeServerName(name: string): string {
  // 移除前导 @ 符号（用于 npm 作用域包）
  let cleaned = name.startsWith('@') ? name.slice(1) : name

  // 将 / 替换为 -（scoped packages）
  cleaned = cleaned.replaceAll('/', '-')

  // 移除其他特殊字符，保留字母数字、连字符、下划线
  cleaned = cleaned.replaceAll(/[^\w-]/g, '-')

  // 合并连续的连字符
  cleaned = cleaned.replaceAll(/-+/g, '-')

  // 转换为小写
  cleaned = cleaned.toLowerCase()

  // 移除开头和结尾的连字符或下划线
  cleaned = cleaned.replaceAll(/^[-_]+|[-_]+$/g, '')

  return cleaned || 'imported-server'
}

function inferServerNameFromUrl(input: string): string {
  try {
    const url = new URL(input)
    const hostname = url.hostname.replace(/^www\./, '')
    const candidate = hostname.split('.')[0] || hostname
    return normalizeServerName(candidate || 'http-server')
  }
  catch {
    return normalizeServerName(input)
  }
}
