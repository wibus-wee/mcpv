// Input: runtime bindings, SWR, react-hook-form, analytics
// Output: runtime settings state + save handler
// Position: Settings runtime hook

import { ConfigService } from '@bindings/mcpv/internal/ui/services'
import type { RuntimeConfigDetail } from '@bindings/mcpv/internal/ui/types'
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import useSWR, { useSWRConfig } from 'swr'

import { toastManager } from '@/components/ui/toast'
import { AnalyticsEvents, track } from '@/lib/analytics'
import { reloadConfig } from '@/modules/servers/lib/reload-config'

import type { RuntimeFormState } from '../lib/runtime-config'
import {
  DEFAULT_RUNTIME_FORM,
  toRuntimeFormState,
} from '../lib/runtime-config'

type UseRuntimeSettingsOptions = {
  canEdit: boolean
}

export const useRuntimeSettings = ({ canEdit }: UseRuntimeSettingsOptions) => {
  const { mutate } = useSWRConfig()
  const form = useForm<RuntimeFormState>({
    defaultValues: DEFAULT_RUNTIME_FORM,
  })
  const { reset, formState } = form
  const { isDirty } = formState

  const {
    data: runtimeConfig,
    error: runtimeError,
    isLoading: runtimeLoading,
    mutate: mutateRuntime,
  } = useSWR<RuntimeConfigDetail>(
    'runtime-config',
    () => ConfigService.GetRuntimeConfig(),
    { revalidateOnFocus: false },
  )

  const runtimeSnapshotRef = useRef<string | null>(null)

  useEffect(() => {
    if (!runtimeConfig) {
      return
    }
    if (isDirty) {
      return
    }
    const nextState = toRuntimeFormState(runtimeConfig)
    const snapshot = JSON.stringify(nextState)
    if (snapshot !== runtimeSnapshotRef.current) {
      runtimeSnapshotRef.current = snapshot
      reset(nextState, { keepDirty: false })
    }
  }, [runtimeConfig, reset, isDirty])

  const statusLabel = useMemo(() => {
    if (runtimeLoading) {
      return 'Loading runtime settings'
    }
    if (runtimeError) {
      return 'Runtime settings unavailable'
    }
    if (isDirty) {
      return 'Unsaved changes'
    }
    return 'All changes saved'
  }, [runtimeError, runtimeLoading, isDirty])

  const saveDisabledReason = useMemo(() => {
    if (runtimeLoading) {
      return 'Runtime settings are still loading'
    }
    if (runtimeError) {
      return 'Runtime settings are unavailable'
    }
    if (!canEdit) {
      return 'Configuration is read-only'
    }
    if (!isDirty) {
      return 'No changes to save'
    }
    return
  }, [canEdit, runtimeError, runtimeLoading, isDirty])

  const handleSave = form.handleSubmit(async (values) => {
    if (!canEdit) {
      return
    }
    const dirtyFieldCount = Object.keys(formState.dirtyFields ?? {}).length
    try {
      await ConfigService.UpdateRuntimeConfig(values)

      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        track(AnalyticsEvents.SETTINGS_RUNTIME_SAVE, {
          result: 'reload_failed',
          dirty_fields_count: dirtyFieldCount,
        })
        toastManager.add({
          type: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }

      await Promise.all([
        mutateRuntime(),
        mutate('runtime-status'),
      ])
      reset(values, { keepDirty: false })

      track(AnalyticsEvents.SETTINGS_RUNTIME_SAVE, {
        result: 'success',
        dirty_fields_count: dirtyFieldCount,
      })
      toastManager.add({
        type: 'success',
        title: 'Runtime updated',
        description: 'Changes applied successfully.',
      })
    }
    catch (err) {
      track(AnalyticsEvents.SETTINGS_RUNTIME_SAVE, {
        result: 'error',
        dirty_fields_count: dirtyFieldCount,
      })
      toastManager.add({
        type: 'error',
        title: 'Update failed',
        description: err instanceof Error ? err.message : 'Update failed',
      })
    }
  })

  return {
    form,
    runtimeConfig,
    runtimeError,
    runtimeLoading,
    statusLabel,
    saveDisabledReason,
    handleSave,
  }
}
