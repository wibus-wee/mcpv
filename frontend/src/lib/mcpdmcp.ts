export type ClientTarget = 'cursor' | 'claude' | 'vscode' | 'codex'
export type SelectorMode = 'server' | 'tag'

export const defaultRpcAddress = 'unix:///tmp/mcpd.sock'

export type SelectorConfig = {
  mode: SelectorMode
  value: string
}

const buildArgs = (selector: SelectorConfig, rpc = defaultRpcAddress) => {
  const args = selector.mode === 'tag'
    ? ['--tag', selector.value]
    : [selector.value]
  if (rpc && rpc !== defaultRpcAddress) {
    args.push('--rpc', rpc)
  }
  return args
}

export function buildMcpCommand(path: string, selector: SelectorConfig, rpc = defaultRpcAddress) {
  const args = buildArgs(selector, rpc)
  return [path, ...args].join(' ')
}

export function buildClientConfig(
  _target: ClientTarget,
  path: string,
  selector: SelectorConfig,
  rpc = defaultRpcAddress,
) {
  const base = {
    command: path,
    args: buildArgs(selector, rpc),
  }

  const serverName = selector.mode === 'server'
    ? selector.value
    : `mcpd-${selector.value}`

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
) {
  const args = buildArgs(selector, rpc).map(arg => (arg.includes(' ') ? `"${arg}"` : arg)).join(' ')
  if (tool === 'claude') {
    return `claude mcp add --transport stdio mcpd -- ${path} ${args}`
  }
  return `codex mcp add mcpd -- ${path} ${args}`
}

export function buildTomlConfig(path: string, selector: SelectorConfig, rpc = defaultRpcAddress) {
  const args = buildArgs(selector, rpc)
  const argsArray = `args = ${JSON.stringify(args)}`
  const serverName = selector.mode === 'server'
    ? selector.value
    : `mcpd-${selector.value}`
  return [
    `[mcp_servers.${serverName}]`,
    `command = "${path}"`,
    argsArray,
    ``,
    `[mcp_servers.${serverName}.env]`,
    `# MY_ENV_VAR = "MY_ENV_VALUE"`,
  ].join('\n')
}
