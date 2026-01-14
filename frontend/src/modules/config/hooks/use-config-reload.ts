// Input: Config reload service, notice state management
// Output: useConfigReload hook - handles config changes with notification feedback
// Position: Module hook - eliminates repeated reload + notify pattern in config components

import { useCallback, useState } from 'react'

import { reloadConfig } from '@/modules/config/lib/reload-config'

/**
 * Notice state for displaying operation feedback
 */
export interface NoticeState {
  variant: 'success' | 'error' | 'warning' | 'info'
  title: string
  description?: string
}

/**
 * Options for executeWithReload
 */
interface ExecuteOptions {
  /** Success message title */
  successTitle?: string
  /** Success message description */
  successDescription?: string
  /** Error message title (defaults to 'Operation failed') */
  errorTitle?: string
  /** Whether to reload config after successful operation */
  reload?: boolean
  /** Callback to refresh data after successful reload */
  onSuccess?: () => void | Promise<void>
}

/**
 * Return type for useConfigReload hook
 */
interface UseConfigReloadReturn {
  /** Current notice state (null if no notice) */
  notice: NoticeState | null
  /** Whether an operation is in progress */
  isPending: boolean
  /** Clear the current notice */
  clearNotice: () => void
  /** Set a custom notice */
  setNotice: (notice: NoticeState | null) => void
  /**
   * Execute an async operation with automatic reload and notification.
   * Handles errors and shows appropriate feedback.
   *
   * @example
   * await executeWithReload(
 *   () => ProfileService.CreateProfile({ name }),
   *   {
   *     successTitle: 'Profile created',
   *     onSuccess: () => mutateProfiles(),
   *   }
   * )
   */
  executeWithReload: <T>(
    operation: () => Promise<T>,
    options?: ExecuteOptions,
  ) => Promise<{ ok: true, data: T } | { ok: false, error: string }>
  /**
   * Just reload config and show notification.
   * Use when you've already performed the operation.
   */
  reloadAndNotify: (options?: Omit<ExecuteOptions, 'reload'>) => Promise<boolean>
}

/**
 * Hook for managing config operations with automatic reload and notifications.
 * Eliminates the repeated pattern of:
 * 1. Clear notice
 * 2. Execute operation
 * 3. Reload config
 * 4. Show success/error notice
 * 5. Refresh data
 *
 * @example
 * function ProfileActions() {
 *   const { notice, isPending, executeWithReload, clearNotice } = useConfigReload()
 *
 *   const handleCreate = async () => {
 *     await executeWithReload(
 *       () => ProfileService.CreateProfile({ name }),
 *       {
 *         successTitle: 'Profile created',
 *         successDescription: 'Changes applied.',
 *         onSuccess: () => mutateProfiles(),
 *       }
 *     )
 *   }
 *
 *   return (
 *     <>
 *       {notice && <NoticeAlert notice={notice} onDismiss={clearNotice} />}
 *       <Button onClick={handleCreate} disabled={isPending}>
 *         Create Profile
 *       </Button>
 *     </>
 *   )
 * }
 */
export function useConfigReload(): UseConfigReloadReturn {
  const [notice, setNotice] = useState<NoticeState | null>(null)
  const [isPending, setIsPending] = useState(false)

  const clearNotice = useCallback(() => {
    setNotice(null)
  }, [])

  const reloadAndNotify = useCallback(async (
    options?: Omit<ExecuteOptions, 'reload'>,
  ): Promise<boolean> => {
    const {
      successTitle = 'Saved',
      successDescription = 'Changes applied.',
      onSuccess,
    } = options ?? {}

    setIsPending(true)
    setNotice(null)

    try {
      const result = await reloadConfig()

      if (!result.ok) {
        setNotice({
          variant: 'error',
          title: 'Reload failed',
          description: result.message,
        })
        return false
      }

      await onSuccess?.()

      setNotice({
        variant: 'success',
        title: successTitle,
        description: successDescription,
      })

      return true
    } finally {
      setIsPending(false)
    }
  }, [])

  const executeWithReload = useCallback(async <T>(
    operation: () => Promise<T>,
    options?: ExecuteOptions,
  ): Promise<{ ok: true, data: T } | { ok: false, error: string }> => {
    const {
      successTitle = 'Saved',
      successDescription = 'Changes applied.',
      errorTitle = 'Operation failed',
      reload = true,
      onSuccess,
    } = options ?? {}

    setIsPending(true)
    setNotice(null)

    try {
      // Execute the main operation
      const data = await operation()

      // Reload config if requested
      if (reload) {
        const reloadResult = await reloadConfig()
        if (!reloadResult.ok) {
          setNotice({
            variant: 'error',
            title: 'Reload failed',
            description: reloadResult.message,
          })
          return { ok: false, error: reloadResult.message }
        }
      }

      // Call success callback
      await onSuccess?.()

      // Show success notice
      setNotice({
        variant: 'success',
        title: successTitle,
        description: successDescription,
      })

      return { ok: true, data }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error'
      setNotice({
        variant: 'error',
        title: errorTitle,
        description: message,
      })
      return { ok: false, error: message }
    } finally {
      setIsPending(false)
    }
  }, [])

  return {
    notice,
    isPending,
    clearNotice,
    setNotice,
    executeWithReload,
    reloadAndNotify,
  }
}

/**
 * Props for NoticeAlert component
 */
export interface NoticeAlertProps {
  notice: NoticeState
  onDismiss: () => void
  className?: string
}
