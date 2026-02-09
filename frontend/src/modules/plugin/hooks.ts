import { PluginService } from '@bindings/mcpv/internal/ui/services'
import type { PluginListEntry, PluginMetrics } from '@bindings/mcpv/internal/ui/types'
import { useCallback, useMemo } from 'react'
import useSWR from 'swr'

import { swrKeys } from '@/lib/swr-keys'

export function usePluginList() {
  return useSWR<PluginListEntry[]>(
    swrKeys.pluginList,
    () => PluginService.GetPluginList(),
    {
      refreshInterval: 5000,
      revalidateOnMount: true,
      dedupingInterval: 2000,
    },
  )
}

export function usePluginMetrics() {
  return useSWR<Record<string, PluginMetrics | undefined>>(
    swrKeys.pluginMetrics,
    () => PluginService.GetPluginMetrics(),
    {
      refreshInterval: 10000,
      dedupingInterval: 5000,
    },
  )
}

export function useTogglePlugin() {
  return useCallback(async (name: string, enabled: boolean) => {
    await PluginService.TogglePlugin({ name, enabled })
  }, [])
}

export function useFilteredPlugins(plugins: PluginListEntry[], searchQuery: string) {
  return useMemo(() => {
    if (!searchQuery.trim()) { return plugins }

    const lowerQuery = searchQuery.toLowerCase()
    return plugins.filter((plugin) => {
      return (
        plugin.name.toLowerCase().includes(lowerQuery)
        || plugin.category.toLowerCase().includes(lowerQuery)
        || plugin.flows.some(flow => flow.toLowerCase().includes(lowerQuery))
      )
    })
  }, [plugins, searchQuery])
}
