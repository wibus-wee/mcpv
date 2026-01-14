// Input: RuntimeService bindings, SWR
// Output: Active callers SWR hook
// Position: Shared hook for active caller registrations

import type { ActiveCaller } from '@bindings/mcpd/internal/ui'
import { RuntimeService } from '@bindings/mcpd/internal/ui'
import useSWR from 'swr'

export const activeCallersKey = 'active-callers'

export function useActiveCallers() {
  return useSWR<ActiveCaller[]>(activeCallersKey, () => RuntimeService.GetActiveCallers(), {
    revalidateOnFocus: false,
    dedupingInterval: 5000,
  })
}
