// Input: UI settings hook, core connection config helpers
// Output: Core connection mode and settings accessor
// Position: Shared hook for core connection state

import { useMemo } from 'react'

import { useEffectiveUISettings } from '@/hooks/use-ui-settings'
import {
  CORE_CONNECTION_SECTION_KEY,
  DEFAULT_CORE_CONNECTION_FORM,
  toCoreConnectionFormState,
} from '@/modules/settings/lib/core-connection-config'

export function useCoreConnectionMode() {
  const { sections, isLoading, error } = useEffectiveUISettings()
  const settings = useMemo(
    () => toCoreConnectionFormState(sections[CORE_CONNECTION_SECTION_KEY]),
    [sections],
  )

  return {
    settings,
    mode: settings.mode ?? DEFAULT_CORE_CONNECTION_FORM.mode,
    isRemote: settings.mode === 'remote',
    isLoading,
    error,
  }
}
