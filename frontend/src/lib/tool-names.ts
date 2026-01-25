export const getToolDisplayName = (toolName: string, serverName?: string) => {
  if (!serverName) return toolName
  const prefix = `${serverName}.`
  if (toolName.startsWith(prefix)) {
    return toolName.slice(prefix.length)
  }
  return toolName
}

export const getToolQualifiedName = (toolName: string, serverName?: string) => {
  if (!serverName) return toolName
  const prefix = `${serverName}.`
  if (toolName.startsWith(prefix)) {
    return toolName
  }
  return `${serverName}.${toolName}`
}
