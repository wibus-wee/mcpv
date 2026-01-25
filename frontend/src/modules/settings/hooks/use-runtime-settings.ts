// Input: runtime bindings, SWR, react-hook-form
// Output: runtime settings state + save handler
// Position: Settings runtime hook

import type { RuntimeConfigDetail } from '@bindings/mcpd/internal/ui'
import { ConfigService } from '@bindings/mcpd/internal/ui'
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import useSWR, { useSWRConfig } from 'swr'

import { toastManager } from '@/components/ui/toast'
import { reloadConfig } from '@/modules/config/lib/reload-config'
import {
  DEFAULT_RUNTIME_FORM,
  type RuntimeFormState,
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
  const isDirty = formState.isDirty

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
    return undefined
  }, [canEdit, runtimeError, runtimeLoading, isDirty])

  const handleSave = form.handleSubmit(async (values) => {
    if (!canEdit) {
      return
    }
    try {
      await ConfigService.UpdateRuntimeConfig(values)

      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
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

      toastManager.add({
        type: 'success',
        title: 'Runtime updated',
        description: 'Changes applied successfully.',
      })
    } catch (err) {
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
