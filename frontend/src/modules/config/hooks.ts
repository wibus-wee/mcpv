// Input: SWR, WailsService bindings, jotai atoms
// Output: Config data fetching hooks including runtime status
// Position: Data fetching hooks for config module

import type {
  ConfigModeResponse,
  ProfileDetail,
  ProfileSummary,
  ServerRuntimeStatus,
} from '@bindings/mcpd/internal/ui'
import { WailsService } from '@bindings/mcpd/internal/ui'
import { useSetAtom } from 'jotai'
import { useCallback, useEffect, useState } from 'react'
import useSWR from 'swr'

import {
  callersAtom,
  configModeAtom,
  profilesAtom,
  selectedProfileAtom,
} from './atoms'

export function useConfigMode() {
  const setConfigMode = useSetAtom(configModeAtom)

  const { data, error, isLoading, mutate } = useSWR<ConfigModeResponse>(
    'config-mode',
    () => WailsService.GetConfigMode(),
  )

  useEffect(() => {
    if (data) {
      setConfigMode(data)
    }
  }, [data, setConfigMode])

  return { data, error, isLoading, mutate }
}

export function useProfiles() {
  const setProfiles = useSetAtom(profilesAtom)

  const { data, error, isLoading, mutate } = useSWR<ProfileSummary[]>(
    'profiles',
    () => WailsService.ListProfiles(),
  )

  useEffect(() => {
    if (data) {
      setProfiles(data)
    }
  }, [data, setProfiles])

  return { data, error, isLoading, mutate }
}

export function useProfile(name: string | null) {
  const setSelectedProfile = useSetAtom(selectedProfileAtom)

  const { data, error, isLoading, mutate } = useSWR<ProfileDetail | null>(
    name ? ['profile', name] : null,
    () => (name ? WailsService.GetProfile(name) : null),
  )

  useEffect(() => {
    if (data !== undefined) {
      setSelectedProfile(data)
    }
  }, [data, setSelectedProfile])

  return { data, error, isLoading, mutate }
}

export function useCallers() {
  const setCallers = useSetAtom(callersAtom)

  const { data, error, isLoading, mutate } = useSWR<Record<string, string>>(
    'callers',
    () => WailsService.GetCallers(),
  )

  useEffect(() => {
    if (data) {
      setCallers(data)
    }
  }, [data, setCallers])

  return { data, error, isLoading, mutate }
}

export function useOpenConfigInEditor() {
  const [isOpening, setIsOpening] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const openInEditor = useCallback(async () => {
    setIsOpening(true)
    setError(null)
    try {
      await WailsService.OpenConfigInEditor()
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
    () => WailsService.GetRuntimeStatus(),
    { refreshInterval: 2000 },
  )
}
