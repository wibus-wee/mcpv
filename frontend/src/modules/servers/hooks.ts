// Input: Config hooks, tools hooks, SWR, analytics
// Output: Combined data hooks for servers module
// Position: Data layer for unified servers module

import { ConfigService, DiscoveryService, RuntimeService, ServerService } from '@bindings/mcpv/internal/ui/services'
import type {
  ActiveClient,
  ConfigModeResponse,
  ServerDetail,
  ServerGroup,
  ServerInitStatus,
  ServerRuntimeStatus,
  ServerSummary,
  ToolEntry,
} from '@bindings/mcpv/internal/ui/types'
import { useCallback, useMemo, useState } from 'react'
import useSWR from 'swr'

import { AnalyticsEvents, track } from '@/lib/analytics'
import { withSWRPreset } from '@/lib/swr-config'
import { swrKeys } from '@/lib/swr-keys'

import { reloadConfig } from './lib/reload-config'

export function useConfigMode() {
  return useSWR<ConfigModeResponse | null>(
    swrKeys.configMode,
    () => ConfigService.GetConfigMode(),
  )
}

export function useServers() {
  return useSWR<ServerSummary[]>(
    swrKeys.servers,
    () => ServerService.ListServers(),
    {
      refreshInterval: 5000,
    },
  )
}

export function useTools() {
  return useSWR<ToolEntry[]>(
    swrKeys.tools,
    () => DiscoveryService.ListTools(),
    {
      refreshInterval: 10000,
      dedupingInterval: 10000,
    },
  )
}

export function useServer(name: string | null) {
  return useSWR<ServerDetail | null>(
    name ? [swrKeys.server, name] : null,
    () => (name ? ServerService.GetServer(name) : null),
  )
}

export function useServerDetails(servers: ServerSummary[] | undefined) {
  const serverNames = servers?.map(server => server.name) ?? []

  return useSWR<ServerDetail[]>(
    serverNames.length > 0 ? [swrKeys.serverDetails, ...serverNames] : null,
    async () => {
      const results = await Promise.all(
        serverNames.map(name => ServerService.GetServer(name)),
      )

      return results.filter(
        (server): server is ServerDetail => server !== null,
      )
    },
  )
}

export function useClients() {
  return useSWR<ActiveClient[]>(
    'active-clients',
    () => RuntimeService.GetActiveClients(),
    withSWRPreset('cached'),
  )
}

export function useOpenConfigInEditor() {
  const [isOpening, setIsOpening] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const openInEditor = useCallback(async () => {
    setIsOpening(true)
    setError(null)
    try {
      await ConfigService.OpenConfigInEditor()
    }
    catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
    }
    finally {
      setIsOpening(false)
    }
  }, [])

  return { openInEditor, isOpening, error }
}

export function useRuntimeStatus() {
  return useSWR<ServerRuntimeStatus[]>(
    swrKeys.runtimeStatus,
    () => RuntimeService.GetRuntimeStatus(),
    withSWRPreset('cached'),
  )
}

export function useServerInitStatus() {
  return useSWR<ServerInitStatus[]>(
    swrKeys.serverInitStatus,
    () => RuntimeService.GetServerInitStatus(),
    withSWRPreset('cached'),
  )
}

export function useToolsByServer() {
  const { data: serverGroups, isLoading: groupsLoading, error: groupsError }
    = useSWR<ServerGroup[]>(
      swrKeys.serverGroups,
      () => ServerService.ListServerGroups(),
      {
        refreshInterval: 10000,
        dedupingInterval: 10000,
      },
    )
  const { data: runtimeStatus, isLoading: runtimeLoading, error: runtimeError } = useRuntimeStatus()

  const isLoading = groupsLoading || runtimeLoading
  const error = groupsError || runtimeError

  const serverMap = useMemo(() => {
    const map = new Map<string, ServerGroup>()
    if (serverGroups) {
      serverGroups.forEach((group) => {
        map.set(group.specKey, group)
      })
    }
    return map
  }, [serverGroups])

  return {
    servers: serverGroups ?? [],
    serverMap,
    isLoading,
    error,
    runtimeStatus: runtimeStatus || [],
  }
}

type ErrorHandler = (title: string, description: string) => void
type SuccessHandler = (title: string, description: string) => void

export function useServerOperation(
  canEdit: boolean,
  mutateServers: () => Promise<any>,
  mutateServer?: () => Promise<any>,
  onDeleted?: (serverName: string) => void,
  errorHandler?: ErrorHandler,
  successHandler?: SuccessHandler,
) {
  const [isWorking, setIsWorking] = useState(false)

  const executeOperation = useCallback(async (
    operation: 'toggle' | 'delete',
    server: { name: string, disabled?: boolean },
  ) => {
    if (!canEdit || isWorking) return
    setIsWorking(true)

    try {
      const nextState = server.disabled ? 'enabled' : 'disabled'
      if (operation === 'toggle') {
        await ServerService.SetServerDisabled({
          server: server.name,
          disabled: !server.disabled,
        })
      }
      else if (operation === 'delete') {
        await ServerService.DeleteServer({ server: server.name })
      }

      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        track(
          operation === 'toggle'
            ? AnalyticsEvents.SERVER_TOGGLE_DISABLED
            : AnalyticsEvents.SERVER_DELETE,
          {
            result: 'reload_failed',
            ...(operation === 'toggle' ? { next_state: nextState } : {}),
          },
        )
        errorHandler?.('Reload failed', reloadResult.message)
        return
      }

      await Promise.all([
        mutateServers(),
        mutateServer?.(),
      ])

      if (operation === 'toggle') {
        track(AnalyticsEvents.SERVER_TOGGLE_DISABLED, {
          result: 'success',
          next_state: nextState,
        })
        successHandler?.(
          server.disabled ? 'Server enabled' : 'Server disabled',
          'Changes applied.',
        )
      }
      else if (operation === 'delete') {
        track(AnalyticsEvents.SERVER_DELETE, { result: 'success' })
        successHandler?.('Server deleted', 'Changes applied.')
        onDeleted?.(server.name)
      }
    }
    catch (err) {
      const message = err instanceof Error ? err.message : `${operation} failed.`
      track(
        operation === 'toggle'
          ? AnalyticsEvents.SERVER_TOGGLE_DISABLED
          : AnalyticsEvents.SERVER_DELETE,
        {
          result: 'error',
          ...(operation === 'toggle' ? { next_state: server.disabled ? 'enabled' : 'disabled' } : {}),
        },
      )
      errorHandler?.(`${operation === 'toggle' ? 'Update' : 'Delete'} failed`, message)
    }
    finally {
      setIsWorking(false)
    }
  }, [canEdit, isWorking, mutateServers, mutateServer, onDeleted, errorHandler, successHandler])

  const toggleDisabled = useCallback((server: { name: string, disabled?: boolean }) =>
    executeOperation('toggle', server), [executeOperation])

  const deleteServer = useCallback((server: { name: string, disabled?: boolean }) =>
    executeOperation('delete', server), [executeOperation])

  return {
    isWorking,
    toggleDisabled,
    deleteServer,
  }
}

export function useFilteredServers(
  servers: ServerSummary[],
  searchQuery: string,
  selectedTags: string[] = [],
) {
  return useMemo(() => {
    let filtered = servers

    if (searchQuery.trim() !== '') {
      const query = searchQuery.trim().toLowerCase()
      filtered = filtered.filter(server =>
        server.name.toLowerCase().includes(query)
        || (server.tags?.some(tag => tag.toLowerCase().includes(query)) ?? false),
      )
    }

    if (selectedTags.length > 0) {
      filtered = filtered.filter(server =>
        selectedTags.every(tag => server.tags?.includes(tag)),
      )
    }

    return filtered
  }, [servers, searchQuery, selectedTags])
}
