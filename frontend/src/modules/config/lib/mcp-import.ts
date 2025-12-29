export type ImportServerDraft = {
  id: string
  name: string
  cmd: string[]
  env: Record<string, string>
  cwd: string
}

export type ImportServerSpec = {
  name: string
  cmd: string[]
  env: Record<string, string>
  cwd: string
}

export type ImportMcpServersRequest = {
  profiles: string[]
  servers: ImportServerSpec[]
}

type McpServerEntry = {
  command?: unknown
  args?: unknown
  env?: unknown
  cwd?: unknown
  transport?: unknown
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
    const command = entry.command
    if (typeof command !== 'string' || command.trim() === '') {
      errors.push(`${prefix}: command is required.`)
      return
    }

    if (entry.transport !== undefined && entry.transport !== 'stdio') {
      errors.push(`${prefix}: only stdio transport is supported.`)
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
