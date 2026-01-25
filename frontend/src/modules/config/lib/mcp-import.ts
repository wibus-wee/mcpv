export type ImportServerDraft = {
  id: string
  name: string
  transport: 'stdio' | 'streamable_http'
  cmd: string[]
  env: Record<string, string>
  cwd: string
  http?: StreamableHTTPDraft
}

export type ImportServerSpec = {
  name: string
  transport?: 'stdio' | 'streamable_http'
  cmd: string[]
  env: Record<string, string>
  cwd: string
  protocolVersion?: string
  http?: StreamableHTTPDraft
}

export type ImportMcpServersRequest = {
  servers: ImportServerSpec[]
}

export type StreamableHTTPDraft = {
  endpoint: string
  headers: Record<string, string>
  maxRetries: number
}

type McpServerEntry = {
  command?: unknown
  args?: unknown
  env?: unknown
  cwd?: unknown
  transport?: unknown
  endpoint?: unknown
  url?: unknown
  headers?: unknown
  maxRetries?: unknown
}

type ParseResult = {
  servers: ImportServerDraft[]
  errors: string[]
}

export function parseMcpServersJson(input: string): ParseResult {
  const trimmed = input.trim()
  if (!trimmed) {
    return { servers: [], errors: ['Paste JSON to continue.'] }
  }

  let payload: unknown
  try {
    payload = JSON.parse(trimmed)
  } catch {
    return { servers: [], errors: ['Invalid JSON format.'] }
  }

  if (!isRecord(payload)) {
    return { servers: [], errors: ['JSON must be an object with mcpServers.'] }
  }

  const rawServers = payload.mcpServers
  if (!isRecord(rawServers)) {
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
    const transport = parseTransport(entry.transport, prefix, errors)
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
      })
      return
    }

    const command = entry.command
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
  if (normalized === 'streamable_http' || normalized === 'streamable-http') {
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

  return {
    endpoint: rawEndpoint.trim(),
    headers,
    maxRetries,
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
