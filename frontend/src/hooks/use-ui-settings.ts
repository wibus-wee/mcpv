// Input: UISettingsService bindings, SWR, SWR presets
// Output: UI settings hooks with update and reset helpers
// Position: Shared UI settings data accessors

import { UISettingsService } from '@bindings/mcpv/internal/ui/services'
import type {
  UISettingsSnapshot,
  UISettingsWorkspaceIDResponse,
} from '@bindings/mcpv/internal/ui/types'
import { useCallback } from 'react'
import useSWR, { useSWRConfig } from 'swr'

import { swrPresets } from '@/lib/swr-config'
import { swrKeys } from '@/lib/swr-keys'

export type UISettingsScope = 'global' | 'workspace'

type UseUISettingsOptions = {
  scope?: UISettingsScope
  workspaceId?: string
  enabled?: boolean
}

type UseEffectiveUISettingsOptions = {
  workspaceId?: string
  enabled?: boolean
}

type UpdatePayload = Record<string, unknown>

const autoWorkspaceKey = '__auto__'

const getScopeKey = (scope: UISettingsScope, workspaceId?: string) => {
  if (scope === 'workspace') {
    return [swrKeys.uiSettings, scope, workspaceId ?? autoWorkspaceKey] as const
  }
  return [swrKeys.uiSettings, scope] as const
}

const getEffectiveKey = (workspaceId?: string) => {
  if (workspaceId) {
    return [swrKeys.uiSettingsEffective, workspaceId] as const
  }
  return [swrKeys.uiSettingsEffective, autoWorkspaceKey] as const
}

export function useUISettings({
  scope = 'global',
  workspaceId,
  enabled = true,
}: UseUISettingsOptions = {}) {
  const key = enabled ? getScopeKey(scope, workspaceId) : null
  const { mutate: mutateGlobal } = useSWRConfig()

  const swr = useSWR<UISettingsSnapshot>(
    key,
    () => UISettingsService.GetUISettings({ scope, workspaceId }),
    swrPresets.static,
  )

  const refresh = useCallback(() => swr.mutate(), [swr])

  const update = useCallback(async (updates: UpdatePayload, removes: string[] = []) => {
    const snapshot = await UISettingsService.UpdateUISettings({
      scope,
      workspaceId,
      updates: updates as Record<string, any>,
      removes,
    })
    await swr.mutate(snapshot, { revalidate: false })
    if (scope === 'global' || scope === 'workspace') {
      const effectiveCacheKey = getEffectiveKey(workspaceId)
      void mutateGlobal(effectiveCacheKey)
    }
    return snapshot
  }, [mutateGlobal, scope, swr, workspaceId])

  const reset = useCallback(async () => {
    const snapshot = await UISettingsService.ResetUISettings({ scope, workspaceId })
    await swr.mutate(snapshot, { revalidate: false })
    if (scope === 'global' || scope === 'workspace') {
      const effectiveCacheKey = getEffectiveKey(workspaceId)
      void mutateGlobal(effectiveCacheKey)
    }
    return snapshot
  }, [mutateGlobal, scope, swr, workspaceId])

  return {
    ...swr,
    sections: swr.data?.sections ?? {},
    updateUISettings: update,
    resetUISettings: reset,
    refreshUISettings: refresh,
  }
}

export function useEffectiveUISettings({
  workspaceId,
  enabled = true,
}: UseEffectiveUISettingsOptions = {}) {
  const key = enabled ? getEffectiveKey(workspaceId) : null

  const swr = useSWR<UISettingsSnapshot>(
    key,
    () => UISettingsService.GetEffectiveUISettings({ workspaceId }),
    swrPresets.static,
  )

  return {
    ...swr,
    sections: swr.data?.sections ?? {},
  }
}

export function useWorkspaceId() {
  return useSWR<UISettingsWorkspaceIDResponse>(
    swrKeys.uiSettingsWorkspaceId,
    () => UISettingsService.GetWorkspaceID(),
    swrPresets.longCached,
  )
}
