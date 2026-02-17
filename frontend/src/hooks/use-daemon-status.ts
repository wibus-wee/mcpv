// Input: DaemonService binding, SWR hooks
// Output: Daemon status hook and refresh action
// Position: Shared daemon status accessor for app-wide use

import { DaemonService } from '@bindings/mcpv/internal/ui/services'
import type { DaemonStatus } from '@bindings/mcpv/internal/ui/services/models'
import { useCallback } from 'react'
import useSWR from 'swr'

import { swrPresets } from '@/lib/swr-config'
import { swrKeys } from '@/lib/swr-keys'

export const daemonStatusKey = swrKeys.daemonStatus

export function useDaemonStatus() {
  const swr = useSWR<DaemonStatus>(
    daemonStatusKey,
    () => DaemonService.Status(),
    swrPresets.cached,
  )

  const { mutate } = swr
  const refreshDaemonStatus = useCallback(() => mutate(), [mutate])

  return {
    ...swr,
    daemonStatus: swr.data,
    refreshDaemonStatus,
  }
}
