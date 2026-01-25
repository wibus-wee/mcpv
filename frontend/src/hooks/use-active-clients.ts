// Input: RuntimeService bindings, SWR presets
// Output: Active clients hook
// Position: Shared hook for realtime client activity

import type { ActiveClient } from '@bindings/mcpd/internal/ui'
import { RuntimeService } from '@bindings/mcpd/internal/ui'
import useSWR from 'swr'

import { withSWRPreset } from '@/lib/swr-config'

export const activeClientsKey = 'active-clients'

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
