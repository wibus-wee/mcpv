// Input: None (pure utility functions)
// Output: mcpvmcp command and config builders with support for server/tag modes and build options
// Position: Utility library for generating IDE connection configs and CLI snippets

export type ClientTarget = 'cursor' | 'claude' | 'vscode' | 'codex'
export type SelectorMode = 'server' | 'tag'

export const defaultRpcAddress = 'unix:///tmp/mcpv.sock'

export type SelectorConfig = {
  mode: SelectorMode
  value: string
}

export type BuildOptions = {
  launchUIOnFail?: boolean
}

const buildArgs = (selector: SelectorConfig, rpc = defaultRpcAddress, options: BuildOptions = {}) => {
  const args = selector.mode === 'tag'
    ? ['--tag', selector.value]
    : [selector.value]
  if (rpc && rpc !== defaultRpcAddress) {
    args.push('--rpc', rpc)
  }
  if (options.launchUIOnFail) {
    args.push('--launch-ui-on-fail')
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
