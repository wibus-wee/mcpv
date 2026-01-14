// Input: ProfileService, reloadConfig, profile mutations
// Output: useProfileActions hook for profile operations
// Position: Business logic hook for profile detail panel

import { ProfileService } from '@bindings/mcpd/internal/ui'
import { useCallback, useEffect, useState } from 'react'

import type { NoticeState } from '../../hooks/use-config-reload'
import { reloadConfig } from '../../lib/reload-config'
import type { ServerSpecWithKey } from './server-item'

interface UseProfileActionsOptions {
  profileName: string | null
  canEditServers: boolean
  canDeleteProfile: boolean
  mutateProfile: () => Promise<void>
  mutateProfiles: () => Promise<void>
}

interface UseProfileActionsReturn {
  notice: NoticeState | null
  pendingServerName: string | null
  deletingProfile: boolean
  clearNotice: () => void
  handleToggleDisabled: (server: ServerSpecWithKey, disabled: boolean) => Promise<void>
  handleDeleteServer: (server: ServerSpecWithKey) => Promise<void>
  handleDeleteProfile: () => Promise<void>
}

/**
 * Custom hook for managing profile actions (toggle, delete server/profile).
 * Handles loading states, error handling, and notifications.
 */
export function useProfileActions({
  profileName,
  canEditServers,
  canDeleteProfile,
  mutateProfile,
  mutateProfiles,
}: UseProfileActionsOptions): UseProfileActionsReturn {
  const [notice, setNotice] = useState<NoticeState | null>(null)
  const [pendingServerName, setPendingServerName] = useState<string | null>(null)
  const [deletingProfile, setDeletingProfile] = useState(false)

  // Reset state when profile changes
  useEffect(() => {
    setNotice(null)
    setPendingServerName(null)
    setDeletingProfile(false)
  }, [profileName])

  const clearNotice = useCallback(() => {
    setNotice(null)
  }, [])

  const handleToggleDisabled = useCallback(async (
    server: ServerSpecWithKey,
    disabled: boolean,
  ) => {
    if (!canEditServers || pendingServerName || !profileName) {
      return
    }

    setPendingServerName(server.name)
    setNotice(null)

    try {
      await ProfileService.SetServerDisabled({
        profile: profileName,
        server: server.name,
        disabled,
      })

      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setNotice({
          variant: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }

      await Promise.all([mutateProfile(), mutateProfiles()])
      setNotice({
        variant: 'success',
        title: 'Saved',
        description: 'Changes applied.',
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Update failed.'
      setNotice({
        variant: 'error',
        title: 'Update failed',
        description: message,
      })
    } finally {
      setPendingServerName(null)
    }
  }, [canEditServers, pendingServerName, profileName, mutateProfile, mutateProfiles])

  const handleDeleteServer = useCallback(async (server: ServerSpecWithKey) => {
    if (!canEditServers || pendingServerName || !profileName) {
      return
    }

    setPendingServerName(server.name)
    setNotice(null)

    try {
      await ProfileService.DeleteServer({
        profile: profileName,
        server: server.name,
      })

      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setNotice({
          variant: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }

      await Promise.all([mutateProfile(), mutateProfiles()])
      setNotice({
        variant: 'success',
        title: 'Server deleted',
        description: 'Changes applied.',
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Delete failed.'
      setNotice({
        variant: 'error',
        title: 'Delete failed',
        description: message,
      })
    } finally {
      setPendingServerName(null)
    }
  }, [canEditServers, pendingServerName, profileName, mutateProfile, mutateProfiles])

  const handleDeleteProfile = useCallback(async () => {
    if (!canDeleteProfile || deletingProfile || !profileName) {
      return
    }

    setDeletingProfile(true)
    setNotice(null)

    try {
      await ProfileService.DeleteProfile({ name: profileName })

      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setNotice({
          variant: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }

      await Promise.all([mutateProfiles(), mutateProfile()])
      setNotice({
        variant: 'success',
        title: 'Profile deleted',
        description: 'Changes applied.',
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Delete failed.'
      setNotice({
        variant: 'error',
        title: 'Delete failed',
        description: message,
      })
    } finally {
      setDeletingProfile(false)
    }
  }, [canDeleteProfile, deletingProfile, profileName, mutateProfile, mutateProfiles])

  return {
    notice,
    pendingServerName,
    deletingProfile,
    clearNotice,
    handleToggleDisabled,
    handleDeleteServer,
    handleDeleteProfile,
  }
}
