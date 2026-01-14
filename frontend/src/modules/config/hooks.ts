// Input: SWR, Config/Profile/Runtime service bindings, jotai atoms
// Output: Config data fetching hooks including profile details and runtime status
// Position: Data fetching hooks for config module

import type {
  ConfigModeResponse,
  ProfileDetail,
  ProfileSummary,
  ServerInitStatus,
  ServerRuntimeStatus,
} from '@bindings/mcpd/internal/ui'
import { ConfigService, ProfileService, RuntimeService } from '@bindings/mcpd/internal/ui'
import { useSetAtom } from 'jotai'
import { useCallback, useEffect, useState } from 'react'
import useSWR from 'swr'

import { withSWRPreset } from '@/lib/swr-config'

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
    () => ConfigService.GetConfigMode(),
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
    () => ProfileService.ListProfiles(),
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
    () => (name ? ProfileService.GetProfile(name) : null),
  )

  useEffect(() => {
    if (data !== undefined) {
      setSelectedProfile(data)
    }
  }, [data, setSelectedProfile])

  return { data, error, isLoading, mutate }
}

export function useProfileDetails(profiles: ProfileSummary[] | undefined) {
  const profileNames = profiles?.map(profile => profile.name) ?? []

  const { data, error, isLoading, mutate } = useSWR<ProfileDetail[]>(
    profileNames.length > 0 ? ['profile-details', ...profileNames] : null,
    async () => {
      const results = await Promise.all(
        profileNames.map(name => ProfileService.GetProfile(name)),
      )

      return results.filter(
        (profile): profile is ProfileDetail => profile !== null,
      )
    },
  )

  return { data, error, isLoading, mutate }
}

export function useCallers() {
  const setCallers = useSetAtom(callersAtom)

  const { data, error, isLoading, mutate } = useSWR<Record<string, string>>(
    'callers',
    () => ProfileService.GetCallers(),
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
