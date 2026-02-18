// Input: CoreService bindings, core connection hook, SWR hooks
// Output: Core state hooks and actions with CoreStatus type
// Position: Shared core state accessors for app-wide status

import { CoreService } from '@bindings/mcpv/internal/ui/services'
import type { CoreStateResponse } from '@bindings/mcpv/internal/ui/types'
import { useCallback } from 'react'
import useSWR, { useSWRConfig } from 'swr'

import { useCoreConnectionMode } from '@/hooks/use-core-connection'
import { swrPresets } from '@/lib/swr-config'
import { swrKeys } from '@/lib/swr-keys'

export type CoreStatus = 'stopped' | 'starting' | 'running' | 'stopping' | 'error'
type StartCoreOptions = {
  mode?: 'dev' | 'prod'
  configPath?: string
  metricsEnabled?: boolean
  healthzEnabled?: boolean
}

export const coreStateKey = swrKeys.coreState
export const coreStateLocalKey = swrKeys.coreStateLocal
export const coreStateRemoteKey = swrKeys.coreStateRemote

const toCoreStatus = (state?: string): CoreStatus => {
  return state ? (state as CoreStatus) : 'stopped'
}

export function useCoreState() {
  const swr = useSWR<CoreStateResponse>(
    coreStateKey,
    () => CoreService.GetCoreState(),
    swrPresets.realtime,
  )

  const coreStatus = toCoreStatus(swr.data?.state)

  return {
    ...swr,
    coreStatus,
  }
}

export function useLocalCoreState() {
  const swr = useSWR<CoreStateResponse>(
    coreStateLocalKey,
    () => CoreService.GetLocalCoreState(),
    swrPresets.realtime,
  )
  const coreStatus = toCoreStatus(swr.data?.state)

  return {
    ...swr,
    coreStatus,
  }
}

export function useRemoteCoreState(enabled = true) {
  const key = enabled ? coreStateRemoteKey : null
  const swr = useSWR<CoreStateResponse>(
    key,
    () => CoreService.GetRemoteCoreState(),
    swrPresets.realtime,
  )
  const coreStatus = toCoreStatus(swr.data?.state)

  return {
    ...swr,
    coreStatus,
  }
}

export function useCoreActions() {
  const { cache, mutate } = useSWRConfig()
  const { isRemote } = useCoreConnectionMode()

  const updateCoreState = useCallback(
    (status: CoreStatus) => {
      mutate(
        coreStateLocalKey,
        (current?: CoreStateResponse) => ({
          ...(current ?? { state: status, uptime: 0 }),
          state: status,
        }),
        { revalidate: false },
      )
      if (!isRemote) {
        mutate(
          coreStateKey,
          (current?: CoreStateResponse) => ({
            ...(current ?? { state: status, uptime: 0 }),
            state: status,
          }),
          { revalidate: false },
        )
      }
    },
    [isRemote, mutate],
  )

  const getCurrentStatus = useCallback(() => {
    const cached = cache.get(coreStateLocalKey) as CoreStateResponse | undefined
    return toCoreStatus(cached?.state)
  }, [cache])

  const refreshCoreState = useCallback(async () => {
    const tasks = [mutate(coreStateLocalKey)]
    tasks.push(mutate(coreStateKey))
    tasks.push(mutate(coreStateRemoteKey))
    return Promise.all(tasks)
  }, [mutate])

  const startCore = useCallback(async () => {
    updateCoreState('starting')
    try {
      const options: StartCoreOptions = {
        mode: import.meta.env.DEV ? 'dev' : 'prod',
      }
      await CoreService.StartCoreWithOptions(options)
    }
    catch (error) {
      updateCoreState('stopped')
      throw error
    }
    await refreshCoreState()
  }, [refreshCoreState, updateCoreState])

  const stopCore = useCallback(async () => {
    const previousStatus = getCurrentStatus()
    updateCoreState('stopped')
    try {
      await CoreService.StopCore()
    }
    catch (error) {
      if (!isRemote) {
        updateCoreState(previousStatus)
      }
      throw error
    }
    await refreshCoreState()
  }, [getCurrentStatus, isRemote, refreshCoreState, updateCoreState])

  const restartCore = useCallback(async () => {
    updateCoreState('starting')
    try {
      await CoreService.RestartCore()
    }
    catch (error) {
      if (!isRemote) {
        updateCoreState('error')
      }
      throw error
    }
    await refreshCoreState()
  }, [isRemote, refreshCoreState, updateCoreState])

  return {
    refreshCoreState,
    restartCore,
    startCore,
    stopCore,
  }
}
