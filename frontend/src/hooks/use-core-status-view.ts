// Input: UI settings hook, core connection mode, local/remote core state hooks
// Output: Core status view selection and resolved core state helpers
// Position: Shared hook for choosing which core status the UI displays

import { useCallback, useMemo } from 'react'

import { useCoreConnectionMode } from '@/hooks/use-core-connection'
import { useLocalCoreState, useRemoteCoreState } from '@/hooks/use-core-state'
import { useUISettings } from '@/hooks/use-ui-settings'

export type CoreStatusView = 'local' | 'remote'

type CoreStatusViewSection = {
  view?: string
}

export const CORE_STATUS_VIEW_SECTION_KEY = 'core-status-view'

const coerceView = (value?: string): CoreStatusView => {
  return value === 'remote' ? 'remote' : 'local'
}

const parseSection = (section: unknown): CoreStatusViewSection | null => {
  if (!section) return null
  if (typeof section === 'string') {
    const trimmed = section.trim()
    if (!trimmed) return null
    if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
      try {
        const parsed = JSON.parse(trimmed) as unknown
        if (parsed && typeof parsed === 'object') {
          return parsed as CoreStatusViewSection
        }
      }
      catch {
        return { view: trimmed }
      }
    }
    return { view: trimmed }
  }
  if (typeof section === 'object') {
    return section as CoreStatusViewSection
  }
  return null
}

export function useCoreStatusView() {
  const { isRemote } = useCoreConnectionMode()
  const { sections, updateUISettings, isLoading, error } = useUISettings({ scope: 'global' })
  const section = sections?.[CORE_STATUS_VIEW_SECTION_KEY]

  const storedView = useMemo(() => {
    const payload = parseSection(section)
    return payload?.view ? coerceView(payload.view) : null
  }, [section])

  const defaultView: CoreStatusView = isRemote ? 'remote' : 'local'
  const view = isRemote ? (storedView ?? defaultView) : 'local'

  const setView = useCallback(async (next: CoreStatusView) => {
    const payload = { view: coerceView(next) }
    await updateUISettings({ [CORE_STATUS_VIEW_SECTION_KEY]: payload })
  }, [updateUISettings])

  return {
    view,
    setView,
    isRemoteAvailable: isRemote,
    isLoading,
    error,
  }
}

export function useCoreStatusViewState() {
  const {
    view,
    setView,
    isRemoteAvailable,
    isLoading: viewLoading,
    error: viewError,
  } = useCoreStatusView()

  const localState = useLocalCoreState()
  const remoteState = useRemoteCoreState(isRemoteAvailable && view === 'remote')
  const activeState = (view === 'remote' && isRemoteAvailable) ? remoteState : localState

  return {
    view,
    setView,
    isRemoteAvailable,
    coreStatus: activeState.coreStatus,
    data: activeState.data,
    isLoading: activeState.isLoading || viewLoading,
    error: activeState.error ?? viewError,
  }
}
