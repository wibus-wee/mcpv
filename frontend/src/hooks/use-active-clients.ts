// Input: RuntimeService bindings, SWR presets
// Output: Active clients hook
// Position: Shared hook for realtime client activity

import { RuntimeService } from '@bindings/mcpv/internal/ui/services'
import type { ActiveClient } from '@bindings/mcpv/internal/ui/types'
import useSWR from 'swr'

import { withSWRPreset } from '@/lib/swr-config'
import { swrKeys } from '@/lib/swr-keys'

export const activeClientsKey = swrKeys.activeClients

export function useActiveClients() {
  return useSWR<ActiveClient[]>(
    activeClientsKey,
    () => RuntimeService.GetActiveClients(),
    withSWRPreset('fastCached', {
      refreshInterval: 4000,
      dedupingInterval: 2000,
    }),
  )
}
