// Input: DiscoveryService bindings, SWR, runtime status hook
// Output: useToolsByServer hook for grouping tools by server
// Position: Data layer for tools module

import { useMemo } from 'react'
import useSWR from 'swr'

import type { ServerDetail, ServerSummary, ToolEntry } from '@bindings/mcpd/internal/ui'
import { DiscoveryService } from '@bindings/mcpd/internal/ui'

import {
  useRuntimeStatus,
  useServerDetails,
  useServers,
} from '@/modules/config/hooks'

export interface ServerGroup {
  id: string
  specKey: string
  serverName: string
  tools: ToolEntry[]
  tags: string[]
  hasToolData: boolean
  specDetail?: ServerDetail
}

export function useToolsByServer() {
  const {
    data: tools,
    isLoading: toolsLoading,
    error: toolsError,
  } = useSWR<ToolEntry[]>('tools', () => DiscoveryService.ListTools())

  const {
    data: runtimeStatus,
    isLoading: runtimeLoading,
    error: runtimeError,
  } = useRuntimeStatus()
  const {
    data: servers,
    isLoading: serversLoading,
    error: serversError,
  } = useServers()
  const {
    data: serverDetails,
    isLoading: detailsLoading,
    error: detailsError,
  } = useServerDetails(servers)

  const toolsBySpecKey = useMemo(() => {
    const map = new Map<string, ToolEntry[]>()
    if (!tools) return map

    tools.forEach(tool => {
      const specKey = tool.specKey || tool.serverName || tool.name
      if (!specKey) return
      const bucket = map.get(specKey)
      if (bucket) {
        bucket.push(tool)
      } else {
        map.set(specKey, [tool])
      }
    })

    return map
  }, [tools])

  const serversFromSummaries = useMemo(() => {
    const map = new Map<string, { summary: ServerSummary; tags: string[] }>()
    if (!servers) return map

    servers.forEach(summary => {
      if (!summary.specKey) return
      map.set(summary.specKey, {
        summary,
        tags: summary.tags ?? [],
      })
    })

    return map
  }, [servers])

  const serverMap = useMemo(() => {
    const map = new Map<string, ServerGroup>()

    const ensureServer = (
      specKey: string,
      serverName?: string,
      specDetail?: ServerDetail,
      tags?: string[],
    ) => {
      if (!specKey) return null
      const existing = map.get(specKey)
      if (existing) {
        if (!existing.serverName && serverName) {
          existing.serverName = serverName
        }
        if (!existing.specDetail && specDetail) {
          existing.specDetail = specDetail
        }
        if (tags && tags.length > 0 && existing.tags.length === 0) {
          existing.tags = tags
        }
        return existing
      }
      const entry: ServerGroup = {
        id: specKey,
        specKey,
        serverName: serverName || specKey,
        tools: [],
        tags: tags ?? [],
        hasToolData: false,
        specDetail,
      }
      map.set(specKey, entry)
      return entry
    }

    serversFromSummaries.forEach(({ summary, tags }, specKey) => {
      ensureServer(specKey, summary.name, undefined, tags)
    })

    serverDetails?.forEach(detail => {
      ensureServer(detail.specKey, detail.name, detail, detail.tags ?? [])
    })

    runtimeStatus?.forEach(status => {
      ensureServer(status.specKey, status.serverName)
    })

    toolsBySpecKey.forEach((toolList, specKey) => {
      const entry = ensureServer(specKey)
      if (entry) {
        entry.tools = toolList
        entry.hasToolData = true
      }
    })

    return map
  }, [runtimeStatus, serverDetails, serversFromSummaries, toolsBySpecKey])

  const groupedServers = useMemo(() => {
    return Array.from(serverMap.values()).sort((a, b) =>
      a.serverName.localeCompare(b.serverName),
    )
  }, [serverMap])

  const isLoading =
    toolsLoading || serversLoading || detailsLoading || runtimeLoading
  const error = toolsError || serversError || detailsError || runtimeError

  return {
    servers: groupedServers,
    serverMap,
    isLoading,
    error,
    runtimeStatus,
  }
}
