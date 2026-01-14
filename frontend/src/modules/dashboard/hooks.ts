// Input: SWR, CoreService/DiscoveryService bindings
// Output: Dashboard data fetching hooks (useAppInfo, useTools, useResources, usePrompts, useBootstrapProgress)
// Position: Data fetching hooks for dashboard module

import type { BootstrapProgressResponse } from '@bindings/mcpd/internal/ui'
import { CoreService, DiscoveryService } from '@bindings/mcpd/internal/ui'
import useSWR from 'swr'

export function useAppInfo() {
  const swr = useSWR(
    'app-info',
    () => CoreService.GetInfo(),
    {
      revalidateOnFocus: false,
      dedupingInterval: 30000,
    },
  )
  return {
    ...swr,
    appInfo: swr.data ?? null,
  }
}

export function useTools() {
  const swr = useSWR(
    'tools',
    () => DiscoveryService.ListTools(),
    {
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    },
  )
  return {
    ...swr,
    tools: swr.data ?? [],
  }
}

export function useResources() {
  const swr = useSWR(
    'resources',
    async () => {
      const page = await DiscoveryService.ListResources('')
      return page?.resources ?? []
    },
    {
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    },
  )
  return {
    ...swr,
    resources: swr.data ?? [],
  }
}

export function usePrompts() {
  const swr = useSWR(
    'prompts',
    async () => {
      const page = await DiscoveryService.ListPrompts('')
      return page?.prompts ?? []
    },
    {
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    },
  )
  return {
    ...swr,
    prompts: swr.data ?? [],
  }
}

/**
 * Bootstrap progress states
 */
export type BootstrapState = 'pending' | 'running' | 'completed' | 'failed'

/**
 * Hook to fetch and track bootstrap progress.
 * Polls rapidly during 'running' state, slower otherwise.
 */
export function useBootstrapProgress(enabled = true) {
  const swr = useSWR<BootstrapProgressResponse | null>(
    enabled ? 'bootstrap-progress' : null,
    () => CoreService.GetBootstrapProgress(),
    {
      refreshInterval: (data) => {
        // Poll faster during active bootstrap
        if (data?.state === 'running') return 500
        if (data?.state === 'pending') return 1000
        return 5000
      },
      revalidateOnFocus: false,
      dedupingInterval: 300,
    },
  )

  const progress = swr.data
  const state = (progress?.state ?? 'pending') as BootstrapState
  const total = progress?.total ?? 0
  const completed = progress?.completed ?? 0
  const failed = progress?.failed ?? 0
  const current = progress?.current ?? ''
  const errors = progress?.errors ?? {}

  // Calculate percentage
  const percentage = total > 0 ? Math.round(((completed + failed) / total) * 100) : 0

  return {
    ...swr,
    progress,
    state,
    total,
    completed,
    failed,
    current,
    errors,
    percentage,
    isBootstrapping: state === 'running',
    isComplete: state === 'completed' || state === 'failed',
    hasErrors: failed > 0 || Object.keys(errors).length > 0,
  }
}
