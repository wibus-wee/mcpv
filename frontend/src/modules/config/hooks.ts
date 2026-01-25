// Input: SWR, Config/Server/Runtime service bindings, jotai atoms
// Output: Config data fetching hooks including server details and runtime status
// Position: Data fetching hooks for config module

import type {
  ActiveClient,
  ConfigModeResponse,
  ServerDetail,
  ServerInitStatus,
  ServerRuntimeStatus,
  ServerSummary,
} from '@bindings/mcpd/internal/ui'
import { ConfigService, RuntimeService, ServerService } from '@bindings/mcpd/internal/ui'
import { useSetAtom } from 'jotai'
import { useCallback, useEffect, useState } from 'react'
import useSWR from 'swr'

import { withSWRPreset } from '@/lib/swr-config'

import {
  activeClientsAtom,
  configModeAtom,
  selectedServerAtom,
  serversAtom,
} from './atoms'

export function useConfigMode() {
  const setConfigMode = useSetAtom(configModeAtom)

  const { data, error, isLoading, mutate } = useSWR<ConfigModeResponse>(
    'config-mode',
    () => ConfigService.GetConfigMode(),
  )

  useEffect(() => {
    if (data) {
      setConfigMode(data)
    }
  }, [data, setConfigMode])

  return { data, error, isLoading, mutate }
}

export function useServers() {
  const setServers = useSetAtom(serversAtom)

  const { data, error, isLoading, mutate } = useSWR<ServerSummary[]>(
    'servers',
    () => ServerService.ListServers(),
  )

  useEffect(() => {
    if (data) {
      setServers(data)
    }
  }, [data, setServers])

  return { data, error, isLoading, mutate }
}

export function useServer(name: string | null) {
  const setSelectedServer = useSetAtom(selectedServerAtom)

  const { data, error, isLoading, mutate } = useSWR<ServerDetail | null>(
    name ? ['server', name] : null,
    () => (name ? ServerService.GetServer(name) : null),
  )

  useEffect(() => {
    if (data !== undefined) {
      setSelectedServer(data)
    }
  }, [data, setSelectedServer])

  return { data, error, isLoading, mutate }
}

export function useServerDetails(servers: ServerSummary[] | undefined) {
  const serverNames = servers?.map(server => server.name) ?? []

  const { data, error, isLoading, mutate } = useSWR<ServerDetail[]>(
    serverNames.length > 0 ? ['server-details', ...serverNames] : null,
    async () => {
      const results = await Promise.all(
        serverNames.map(name => ServerService.GetServer(name)),
      )

      return results.filter(
        (server): server is ServerDetail => server !== null,
      )
    },
  )

  return { data, error, isLoading, mutate }
}

export function useClients() {
  const setActiveClients = useSetAtom(activeClientsAtom)

  const { data, error, isLoading, mutate } = useSWR<ActiveClient[]>(
    'active-clients',
    () => RuntimeService.GetActiveClients(),
  )

  useEffect(() => {
    if (data) {
      setActiveClients(data)
    }
  }, [data, setActiveClients])

  return { data, error, isLoading, mutate }
}

export function useOpenConfigInEditor() {
  const [isOpening, setIsOpening] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const openInEditor = useCallback(async () => {
    setIsOpening(true)
    setError(null)
    try {
      await ConfigService.OpenConfigInEditor()
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
    } finally {
      setIsOpening(false)
    }
  }, [])

  return { openInEditor, isOpening, error }
}

export function useRuntimeStatus() {
  return useSWR<ServerRuntimeStatus[]>(
    'runtime-status',
    () => RuntimeService.GetRuntimeStatus(),
    withSWRPreset('fastCached', {
      refreshInterval: 2000,
      dedupingInterval: 2000,
    }),
  )
}

export function useServerInitStatus() {
  return useSWR<ServerInitStatus[]>(
    'server-init-status',
    () => RuntimeService.GetServerInitStatus(),
    withSWRPreset('fastCached', {
      refreshInterval: 2000,
      dedupingInterval: 2000,
    }),
  )
}
