// Input: WailsService bindings, SWR, runtime status hook
// Output: useToolsByServer hook for grouping tools by server
// Position: Data layer for tools module

import { useMemo } from 'react'
import useSWR from 'swr'

import { WailsService, type ToolEntry } from '@bindings/mcpd/internal/ui'

import { useRuntimeStatus } from '@/modules/config/hooks'

interface ServerGroup {
  id: string
  specKey: string
  serverName: string
  tools: ToolEntry[]
}

export function useToolsByServer() {
  const { data: tools, isLoading, error } = useSWR<ToolEntry[]>(
    'tools',
    () => WailsService.ListTools()
  )

  const { data: runtimeStatus } = useRuntimeStatus()

  const serverMap = useMemo(() => {
    if (!tools) return new Map<string, ServerGroup>()

    const map = new Map<string, ServerGroup>()

    tools.forEach(tool => {
      const groupKey = tool.serverName || tool.specKey || tool.name
      if (!groupKey) return

      if (!map.has(groupKey)) {
        map.set(groupKey, {
          id: groupKey,
          specKey: tool.specKey || groupKey,
          serverName: tool.serverName || groupKey,
          tools: []
        })
      }
      map.get(groupKey)!.tools.push(tool)
    })

    return map
  }, [tools])

  const servers = useMemo(() => {
    return Array.from(serverMap.values()).sort((a, b) =>
      a.serverName.localeCompare(b.serverName)
    )
  }, [serverMap])

  return {
    servers,
    serverMap,
    isLoading,
    error,
    runtimeStatus
  }
}
