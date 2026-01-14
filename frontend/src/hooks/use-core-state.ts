// Input: CoreService bindings, SWR hooks
// Output: Core state hooks and actions with CoreStatus type
// Position: Shared core state accessors for app-wide status

import type { CoreStateResponse } from '@bindings/mcpd/internal/ui'
import { CoreService } from '@bindings/mcpd/internal/ui'
import { useCallback } from 'react'
import useSWR, { useSWRConfig } from 'swr'

export type CoreStatus = 'stopped' | 'starting' | 'running' | 'stopping' | 'error'
type StartCoreOptions = {
  mode?: 'dev' | 'prod'
  configPath?: string
  metricsEnabled?: boolean
  healthzEnabled?: boolean
}

export const coreStateKey = 'core-state'

const toCoreStatus = (state?: string): CoreStatus => {
  return state ? (state as CoreStatus) : 'stopped'
}

export function useCoreState() {
  const swr = useSWR<CoreStateResponse>(
    coreStateKey,
    () => CoreService.GetCoreState(),
    {
      refreshInterval: 5000,
      revalidateOnFocus: true,
    },
  )

  const coreStatus = toCoreStatus(swr.data?.state)

  return {
    ...swr,
    coreStatus,
  }
}

export function useCoreActions() {
  const { cache, mutate } = useSWRConfig()

  const updateCoreState = useCallback(
    (status: CoreStatus) => {
      mutate(
        coreStateKey,
        (current?: CoreStateResponse) => ({
          ...(current ?? { state: status, uptime: 0 }),
          state: status,
        }),
        { revalidate: false },
      )
    },
    [mutate],
  )

  const getCurrentStatus = useCallback(() => {
    const cached = cache.get(coreStateKey) as CoreStateResponse | undefined
    return toCoreStatus(cached?.state)
  }, [cache])

  const refreshCoreState = useCallback(() => mutate(coreStateKey), [mutate])

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
      updateCoreState(previousStatus)
      throw error
    }
    await refreshCoreState()
  }, [getCurrentStatus, refreshCoreState, updateCoreState])

  const restartCore = useCallback(async () => {
    updateCoreState('starting')
    try {
      await CoreService.RestartCore()
    }
    catch (error) {
      updateCoreState('error')
      throw error
    }
    await refreshCoreState()
  }, [refreshCoreState, updateCoreState])

  return {
    refreshCoreState,
    restartCore,
    startCore,
    stopCore,
  }
}
