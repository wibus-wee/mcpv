// Input: UI settings hook, react-hook-form, core connection config helpers
// Output: Core connection settings state + save handler
// Position: Settings core connection hook

import type { BaseSyntheticEvent } from 'react'
import { useEffect, useMemo, useRef } from 'react'
import type { UseFormReturn } from 'react-hook-form'
import { useForm, useWatch } from 'react-hook-form'

import { toastManager } from '@/components/ui/toast'
import { useUISettings } from '@/hooks/use-ui-settings'

import type { CoreConnectionFormState } from '../lib/core-connection-config'
import {
  buildCoreConnectionPayload,
  CORE_CONNECTION_SECTION_KEY,
  DEFAULT_CORE_CONNECTION_FORM,
  toCoreConnectionFormState,
} from '../lib/core-connection-config'

type UseCoreConnectionSettingsOptions = {
  canEdit: boolean
}

type UseCoreConnectionSettingsResult = {
  form: UseFormReturn<CoreConnectionFormState>
  coreConnectionLoading: boolean
  coreConnectionError: unknown
  statusLabel: string
  saveDisabledReason?: string
  validationError?: string
  mode: CoreConnectionFormState['mode']
  authMode: CoreConnectionFormState['authMode']
  tlsEnabled: boolean
  handleSave: (event?: BaseSyntheticEvent) => void
}

export const useCoreConnectionSettings = ({
  canEdit,
}: UseCoreConnectionSettingsOptions): UseCoreConnectionSettingsResult => {
  const form = useForm<CoreConnectionFormState>({
    defaultValues: DEFAULT_CORE_CONNECTION_FORM,
  })
  const { reset, formState, setValue, control } = form
  const { isDirty } = formState

  const {
    error: coreConnectionError,
    isLoading: coreConnectionLoading,
    sections,
    updateUISettings,
  } = useUISettings({ scope: 'global' })

  const snapshotRef = useRef<string | null>(null)

  useEffect(() => {
    const nextState = toCoreConnectionFormState(sections?.[CORE_CONNECTION_SECTION_KEY])
    if (isDirty) return
    const snapshot = JSON.stringify(nextState)
    if (snapshot !== snapshotRef.current) {
      snapshotRef.current = snapshot
      reset(nextState, { keepDirty: false })
    }
  }, [isDirty, reset, sections])

  const mode = useWatch({
    control,
    name: 'mode',
    defaultValue: DEFAULT_CORE_CONNECTION_FORM.mode,
  })
  const rpcAddress = useWatch({
    control,
    name: 'rpcAddress',
    defaultValue: DEFAULT_CORE_CONNECTION_FORM.rpcAddress,
  })
  const authMode = useWatch({
    control,
    name: 'authMode',
    defaultValue: DEFAULT_CORE_CONNECTION_FORM.authMode,
  })
  const authToken = useWatch({
    control,
    name: 'authToken',
    defaultValue: DEFAULT_CORE_CONNECTION_FORM.authToken,
  })
  const authTokenEnv = useWatch({
    control,
    name: 'authTokenEnv',
    defaultValue: DEFAULT_CORE_CONNECTION_FORM.authTokenEnv,
  })
  const tlsEnabled = useWatch({
    control,
    name: 'tlsEnabled',
    defaultValue: DEFAULT_CORE_CONNECTION_FORM.tlsEnabled,
  })

  useEffect(() => {
    if (authMode === 'mtls' && !tlsEnabled) {
      setValue('tlsEnabled', true, { shouldDirty: true })
    }
  }, [authMode, setValue, tlsEnabled])

  const validationError = useMemo(() => {
    if (mode !== 'remote') return
    if (!rpcAddress || rpcAddress.trim() === '') {
      return 'RPC address is required for remote mode'
    }
    if (authMode === 'token' && !authToken.trim() && !authTokenEnv.trim()) {
      return 'Provide a token or token env for token auth'
    }
    if (authMode === 'mtls' && !tlsEnabled) {
      return 'Enable TLS to use mTLS authentication'
    }
  }, [authMode, authToken, authTokenEnv, mode, rpcAddress, tlsEnabled])

  const statusLabel = useMemo(() => {
    if (coreConnectionLoading) return 'Loading connection settings'
    if (coreConnectionError) return 'Connection settings unavailable'
    if (validationError) return validationError
    if (isDirty) return 'Unsaved changes'
    return 'All changes saved'
  }, [coreConnectionError, coreConnectionLoading, isDirty, validationError])

  const saveDisabledReason = useMemo(() => {
    if (coreConnectionLoading) return 'Connection settings are still loading'
    if (coreConnectionError) return 'Connection settings are unavailable'
    if (validationError) return validationError
    if (!canEdit) return 'Configuration is read-only'
    if (!isDirty) return 'No changes to save'
    return
  }, [canEdit, coreConnectionError, coreConnectionLoading, isDirty, validationError])

  const handleSave = form.handleSubmit(async (values) => {
    if (!canEdit || validationError) {
      if (validationError) {
        toastManager.add({
          type: 'error',
          title: 'Validation required',
          description: validationError,
        })
      }
      return
    }
    try {
      const payload = buildCoreConnectionPayload(values)
      const snapshot = await updateUISettings({
        [CORE_CONNECTION_SECTION_KEY]: payload,
      })
      reset(values, { keepDirty: false })
      toastManager.add({
        type: 'success',
        title: 'Connection updated',
        description: 'Core connection settings saved.',
      })
      return snapshot
    }
    catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Update failed',
        description: err instanceof Error ? err.message : 'Unable to update connection settings',
      })
    }
  })

  return {
    form,
    coreConnectionLoading,
    coreConnectionError,
    statusLabel,
    saveDisabledReason,
    validationError,
    mode,
    authMode,
    tlsEnabled: Boolean(tlsEnabled),
    handleSave,
  }
}
