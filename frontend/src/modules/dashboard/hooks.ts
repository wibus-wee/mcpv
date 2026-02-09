import { CoreService, DiscoveryService } from '@bindings/mcpv/internal/ui/services'
import type { BootstrapProgressResponse } from '@bindings/mcpv/internal/ui/types'
import useSWR from 'swr'

import { swrPresets } from '@/lib/swr-config'
import { swrKeys } from '@/lib/swr-keys'

/**
 * Bootstrap progress states - derived from BootstrapProgressResponse.state
 */
export type BootstrapState = BootstrapProgressResponse['state']

export function useAppInfo() {
  const swr = useSWR(
    swrKeys.appInfo,
    () => CoreService.GetInfo(),
    swrPresets.longCached,
  )
  return {
    ...swr,
    appInfo: swr.data ?? null,
  }
}

export function useTools() {
  const swr = useSWR(
    swrKeys.tools,
    () => DiscoveryService.ListTools(),
    swrPresets.cached,
  )
  return {
    ...swr,
    tools: swr.data ?? [],
  }
}

export function useResources() {
  const swr = useSWR(
    swrKeys.resources,
    async () => {
      const page = await DiscoveryService.ListResources('')
      return page
    },
    swrPresets.cached,
  )
  return {
    ...swr,
    resources: swr.data?.resources ?? [],
    nextCursor: swr.data?.nextCursor,
    hasNextPage: Boolean(swr.data?.nextCursor),
  }
}

export function usePrompts() {
  const swr = useSWR(
    swrKeys.prompts,
    async () => {
      const page = await DiscoveryService.ListPrompts('')
      return page
    },
    swrPresets.cached,
  )
  return {
    ...swr,
    prompts: swr.data?.prompts ?? [],
    nextCursor: swr.data?.nextCursor,
    hasNextPage: Boolean(swr.data?.nextCursor),
  }
}

/**
 * Hook to fetch and track bootstrap progress.
 * Polls rapidly during 'running' state, slower otherwise.
 */
export function useBootstrapProgress(enabled = true) {
  const swr = useSWR<BootstrapProgressResponse | null>(
    enabled ? swrKeys.bootstrapProgress : null,
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
