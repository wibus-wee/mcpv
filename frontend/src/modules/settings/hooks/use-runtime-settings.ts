// Input: runtime bindings, SWR, react-hook-form
// Output: runtime settings state + save handler
// Position: Settings runtime hook

import type {
  ProfileDetail,
  ProfileSummary,
} from '@bindings/mcpd/internal/ui'
import { ConfigService, ProfileService } from '@bindings/mcpd/internal/ui'
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
    data: profiles,
    error: profilesError,
    isLoading: profilesLoading,
  } = useSWR<ProfileSummary[]>(
    'profiles',
    () => ProfileService.ListProfiles(),
    {
      revalidateOnFocus: false,
    },
  )

  const runtimeProfileName = useMemo(() => {
    return profiles?.find(profile => profile.isDefault)?.name
      ?? profiles?.[0]?.name
      ?? null
  }, [profiles])

  const {
    data: runtimeProfile,
    error: runtimeError,
    isLoading: runtimeLoading,
    mutate: mutateRuntimeProfile,
  } = useSWR<ProfileDetail | null>(
    runtimeProfileName ? ['profile', runtimeProfileName] : null,
    () => (runtimeProfileName ? ProfileService.GetProfile(runtimeProfileName) : null),
    {
      revalidateOnFocus: false,
    },
  )

  const runtimeSnapshotRef = useRef<string | null>(null)

  useEffect(() => {
    if (runtimeProfile?.runtime) {
      if (isDirty) {
        return
      }
      const nextState = toRuntimeFormState(runtimeProfile.runtime)
      const snapshot = JSON.stringify(nextState)
      if (snapshot !== runtimeSnapshotRef.current) {
        runtimeSnapshotRef.current = snapshot
        reset(nextState, { keepDirty: false })
      }
      return
    }
    if (runtimeProfile === null && !isDirty) {
      runtimeSnapshotRef.current = null
      reset(DEFAULT_RUNTIME_FORM, { keepDirty: false })
    }
  }, [runtimeProfile, reset, isDirty])

  const hasProfiles = (profiles?.length ?? 0) > 0
  const hasRuntimeProfile = Boolean(runtimeProfile?.runtime)
  const showRuntimeSkeleton = profilesLoading || runtimeLoading

  const statusLabel = useMemo(() => {
    if (showRuntimeSkeleton) {
      return 'Loading runtime settings'
    }
    if (!hasRuntimeProfile) {
      return 'Runtime data unavailable'
    }
    if (isDirty) {
      return 'Unsaved changes'
    }
    return 'All changes saved'
  }, [hasRuntimeProfile, isDirty, showRuntimeSkeleton])

  const saveDisabledReason = useMemo(() => {
    if (showRuntimeSkeleton) {
      return 'Runtime settings are still loading'
    }
    if (!hasRuntimeProfile) {
      return 'Runtime settings are unavailable'
    }
    if (!canEdit) {
      return 'Configuration is read-only'
    }
    if (!isDirty) {
      return 'No changes to save'
    }
    return undefined
  }, [canEdit, hasRuntimeProfile, isDirty, showRuntimeSkeleton])

  const handleSave = form.handleSubmit(async (values) => {
    if (!runtimeProfileName || !canEdit) {
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
        mutateRuntimeProfile(),
        mutate('profiles'),
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
    profiles,
    profilesError,
    profilesLoading,
    runtimeProfileName,
    runtimeProfile,
    runtimeError,
    runtimeLoading,
    hasProfiles,
    hasRuntimeProfile,
    showRuntimeSkeleton,
    statusLabel,
    saveDisabledReason,
    handleSave,
  }
}
