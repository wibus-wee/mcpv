// Input: None (pure utility functions)
// Output: mcpvmcp command and config builders with support for server/tag modes and build options
// Position: Utility library for generating IDE connection configs and CLI snippets

export type ClientTarget = 'cursor' | 'claude' | 'vscode' | 'codex'
export type SelectorMode = 'server' | 'tag'
export type TransportType = 'stdio' | 'streamable-http'

export const defaultRpcAddress = 'unix:///tmp/mcpv.sock'

export type SelectorConfig = {
  mode: SelectorMode
  value: string
}

export type BuildOptions = {
  // General
  transport?: TransportType
  launchUIOnFail?: boolean
  caller?: string
  urlScheme?: string

  // RPC Settings
  rpcMaxRecvMsgSize?: number
  rpcMaxSendMsgSize?: number
  rpcKeepaliveTime?: number
  rpcKeepaliveTimeout?: number
  rpcTLSEnabled?: boolean
  rpcTLSCertFile?: string
  rpcTLSKeyFile?: string
  rpcTLSCAFile?: string

  // HTTP Settings (for streamable-http transport)
  httpAddr?: string
  httpPath?: string
  httpToken?: string
  httpAllowedOrigins?: string[]
  httpJSONResponse?: boolean
  httpSessionTimeout?: number
  httpTLSEnabled?: boolean
  httpTLSCertFile?: string
  httpTLSKeyFile?: string
  httpEventStore?: boolean
  httpEventStoreBytes?: number
}

const buildArgs = (selector: SelectorConfig, rpc = defaultRpcAddress, options: BuildOptions = {}) => {
  const args = selector.mode === 'tag'
    ? ['--tag', selector.value]
    : [selector.value]

  // RPC settings
  if (rpc && rpc !== defaultRpcAddress) {
    args.push('--rpc', rpc)
  }
  if (options.rpcMaxRecvMsgSize) {
    args.push('--rpc-max-recv', String(options.rpcMaxRecvMsgSize))
  }
  if (options.rpcMaxSendMsgSize) {
    args.push('--rpc-max-send', String(options.rpcMaxSendMsgSize))
  }
  if (options.rpcKeepaliveTime) {
    args.push('--rpc-keepalive-time', String(options.rpcKeepaliveTime))
  }
  if (options.rpcKeepaliveTimeout) {
    args.push('--rpc-keepalive-timeout', String(options.rpcKeepaliveTimeout))
  }
  if (options.rpcTLSEnabled) {
    args.push('--rpc-tls')
    if (options.rpcTLSCertFile) {
      args.push('--rpc-tls-cert', options.rpcTLSCertFile)
    }
    if (options.rpcTLSKeyFile) {
      args.push('--rpc-tls-key', options.rpcTLSKeyFile)
    }
    if (options.rpcTLSCAFile) {
      args.push('--rpc-tls-ca', options.rpcTLSCAFile)
    }
  }

  // General settings
  if (options.caller) {
    args.push('--caller', options.caller)
  }
  if (options.launchUIOnFail) {
    args.push('--launch-ui-on-fail')
  }
  if (options.urlScheme && options.urlScheme !== 'mcpv') {
    args.push('--url-scheme', options.urlScheme)
  }

  // Transport settings
  if (options.transport && options.transport !== 'stdio') {
    args.push('--transport', options.transport)
  }

  // HTTP settings (for streamable-http transport)
  if (options.transport === 'streamable-http') {
    if (options.httpAddr) {
      args.push('--http-addr', options.httpAddr)
    }
    if (options.httpPath) {
      args.push('--http-path', options.httpPath)
    }
    if (options.httpToken) {
      args.push('--http-token', options.httpToken)
    }
    if (options.httpAllowedOrigins && options.httpAllowedOrigins.length > 0) {
      options.httpAllowedOrigins.forEach((origin) => {
        args.push('--http-allowed-origin', origin)
      })
    }
    if (options.httpJSONResponse) {
      args.push('--http-json-response')
    }
    if (options.httpSessionTimeout) {
      args.push('--http-session-timeout', String(options.httpSessionTimeout))
    }
    if (options.httpTLSEnabled) {
      args.push('--http-tls')
      if (options.httpTLSCertFile) {
        args.push('--http-tls-cert', options.httpTLSCertFile)
      }
      if (options.httpTLSKeyFile) {
        args.push('--http-tls-key', options.httpTLSKeyFile)
      }
    }
    if (options.httpEventStore) {
      args.push('--http-event-store')
      if (options.httpEventStoreBytes) {
        args.push('--http-event-store-bytes', String(options.httpEventStoreBytes))
      }
    }
  }

  return args
}

export function buildMcpCommand(path: string, selector: SelectorConfig, rpc = defaultRpcAddress, options: BuildOptions = {}) {
  const args = buildArgs(selector, rpc, options)
  return [path, ...args].join(' ')
}

export function buildClientConfig(
  _target: ClientTarget,
  path: string,
  selector: SelectorConfig,
  rpc = defaultRpcAddress,
  options: BuildOptions = {},
) {
  const base = {
    command: path,
    args: buildArgs(selector, rpc, options),
  }

  const serverName = selector.mode === 'server'
    ? selector.value
    : `mcpv-${selector.value}`

  const payload = {
    mcpServers: {
      [serverName]: base,
    },
  }

  return JSON.stringify(payload, null, 2)
}

export function buildCliSnippet(
  path: string,
  selector: SelectorConfig,
  rpc = defaultRpcAddress,
  tool: 'claude' | 'codex',
  options: BuildOptions = {},
) {
  const args = buildArgs(selector, rpc, options).map(arg => (arg.includes(' ') ? `"${arg}"` : arg)).join(' ')
  if (tool === 'claude') {
    return `claude mcp add --transport stdio mcpv -- ${path} ${args}`
  }
  return `codex mcp add mcpv -- ${path} ${args}`
}

export function buildTomlConfig(path: string, selector: SelectorConfig, rpc = defaultRpcAddress, options: BuildOptions = {}) {
  const args = buildArgs(selector, rpc, options)
  const argsArray = `args = ${JSON.stringify(args)}`
  const serverName = selector.mode === 'server'
    ? selector.value
    : `mcpv-${selector.value}`
  return [
    `[mcp_servers.${serverName}]`,
    `command = "${path}"`,
    argsArray,
    // ``,
    // `[mcp_servers.${serverName}.env]`,
    // `# MY_ENV_VAR = "MY_ENV_VALUE"`,
  ].join('\n')
}
