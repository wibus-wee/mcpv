// Input: SWR, WailsService bindings, jotai atoms
// Output: Dashboard data fetching hooks (useAppInfo, useTools, useResources, usePrompts, useCoreState)
// Position: Data fetching hooks for dashboard module

import { WailsService } from '@bindings/mcpd/internal/ui'
import { useSetAtom } from 'jotai'
import { useEffect } from 'react'
import useSWR from 'swr'

import { coreStatusAtom } from '@/atoms/core'
import {
  appInfoAtom,
  promptsAtom,
  resourcesAtom,
  toolsAtom,
} from '@/atoms/dashboard'
import { jotaiStore } from '@/lib/jotai'

export function useAppInfo() {
  const setAppInfo = useSetAtom(appInfoAtom)

  const { data, error, isLoading, mutate } = useSWR(
    'app-info',
    () => WailsService.GetInfo(),
    {
      revalidateOnFocus: false,
      dedupingInterval: 30000,
    },
  )

  useEffect(() => {
    if (data) {
      setAppInfo(data)
    }
  }, [data, setAppInfo])

  return { data, error, isLoading, mutate }
}

export function useCoreState() {
  const setCoreStatus = useSetAtom(coreStatusAtom)

  const { data, error, isLoading, mutate } = useSWR(
    'core-state',
    () => WailsService.GetCoreState(),
    {
      refreshInterval: 5000,
      revalidateOnFocus: true,
    },
  )

  useEffect(() => {
    if (data) {
      const status = data.state as 'stopped' | 'starting' | 'running' | 'stopping' | 'error'
      setCoreStatus(status)
    }
  }, [data, setCoreStatus])

  return { data, error, isLoading, mutate }
}

export function useTools() {
  const setTools = useSetAtom(toolsAtom)

  const { data, error, isLoading, mutate } = useSWR(
    'tools',
    () => WailsService.ListTools(),
    {
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    },
  )

  useEffect(() => {
    if (data) {
      setTools(data)
    }
  }, [data, setTools])

  return { data, error, isLoading, mutate }
}

export function useResources() {
  const setResources = useSetAtom(resourcesAtom)

  const { data, error, isLoading, mutate } = useSWR(
    'resources',
    async () => {
      const page = await WailsService.ListResources('')
      return page?.resources ?? []
    },
    {
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    },
  )

  useEffect(() => {
    if (data) {
      setResources(data)
    }
  }, [data, setResources])

  return { data, error, isLoading, mutate }
}

export function usePrompts() {
  const setPrompts = useSetAtom(promptsAtom)

  const { data, error, isLoading, mutate } = useSWR(
    'prompts',
    async () => {
      const page = await WailsService.ListPrompts('')
      return page?.prompts ?? []
    },
    {
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    },
  )

  useEffect(() => {
    if (data) {
      setPrompts(data)
    }
  }, [data, setPrompts])

  return { data, error, isLoading, mutate }
}

export async function startCore() {
  jotaiStore.set(coreStatusAtom, 'starting')
  try {
    await WailsService.StartCore()
  }
  catch (error) {
    jotaiStore.set(coreStatusAtom, 'stopped')
    throw error
  }
}

export async function stopCore() {
  const previousStatus = jotaiStore.get(coreStatusAtom)
  jotaiStore.set(coreStatusAtom, 'stopped')
  try {
    await WailsService.StopCore()
  }
  catch (error) {
    jotaiStore.set(coreStatusAtom, previousStatus)
    throw error
  }
}

export async function restartCore() {
  jotaiStore.set(coreStatusAtom, 'starting')
  try {
    await WailsService.RestartCore()
  }
  catch (error) {
    jotaiStore.set(coreStatusAtom, 'error')
    throw error
  }
}
