import type { PluginListEntry, PluginMetrics } from '@bindings/mcpv/internal/ui'
import { PluginService } from '@bindings/mcpv/internal/ui'
import { useCallback } from 'react'
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
  return useSWR<Record<string, PluginMetrics>>(
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
