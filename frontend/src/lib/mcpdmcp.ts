export type ClientTarget = 'cursor' | 'claude' | 'vscode'

export const defaultRpcAddress = 'unix:///tmp/mcpd.sock'

export function buildMcpCommand(path: string, caller: string, rpc = defaultRpcAddress) {
  return `${path} ${caller} --rpc ${rpc}`
}

export function buildClientConfig(
  target: ClientTarget,
  path: string,
  caller: string,
  rpc = defaultRpcAddress,
) {
  const base = {
    command: path,
    args: [caller, '--rpc', rpc],
  }

  const payload = {
    mcpServers: {
      mcpd: base,
    },
  }

  return JSON.stringify(payload, null, 2)
}
