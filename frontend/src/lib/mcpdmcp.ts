export type ClientTarget = 'cursor' | 'claude' | 'vscode'

export const defaultRpcAddress = 'unix:///tmp/mcpd.sock'

export function buildMcpCommand(path: string, caller: string, rpc = defaultRpcAddress) {
  const args = [path, caller]
  if (rpc && rpc !== defaultRpcAddress) {
    args.push('--rpc', rpc)
  }
  return args.join(' ')
}

export function buildClientConfig(
  target: ClientTarget,
  path: string,
  caller: string,
  rpc = defaultRpcAddress,
) {
  const base = {
    command: path,
    args: rpc && rpc !== defaultRpcAddress ? [caller, '--rpc', rpc] : [caller],
  }

  const payload = {
    mcpServers: {
      mcpd: base,
    },
  }

  return JSON.stringify(payload, null, 2)
}

export function buildCliSnippet(path: string, caller: string, rpc = defaultRpcAddress, tool: 'claude' | 'codex') {
  const rpcArgs = rpc && rpc !== defaultRpcAddress ? ` --rpc ${rpc}` : ''
  if (tool === 'claude') {
    return `claude mcp add --transport stdio mcpd -- ${path} ${caller}${rpcArgs}`
  }
  return `codex mcp add mcpd -- ${path} ${caller}${rpcArgs}`
}

export function buildTomlConfig(path: string, caller: string, rpc = defaultRpcAddress) {
  const rpcArgs = rpc && rpc !== defaultRpcAddress ? `", "--rpc", "${rpc}` : ''
  const argsArray = rpc && rpc !== defaultRpcAddress
    ? `args = ["${caller}", "--rpc", "${rpc}"]`
    : `args = ["${caller}"]`
  return [
    `[mcp_servers.mcpd]`,
    `command = "${path}"`,
    argsArray,
    ``,
    `[mcp_servers.mcpd.env]`,
    `# MY_ENV_VAR = "MY_ENV_VALUE"`,
  ].join('\n')
}
