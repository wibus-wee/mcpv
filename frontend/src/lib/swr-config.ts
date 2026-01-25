// Input: SWR configuration types
// Output: Centralized SWR configuration presets for consistent data fetching behavior
// Position: Infrastructure layer - provides type-safe SWR config templates

import type { SWRConfiguration } from 'swr'

/**
 * SWR configuration presets for different data fetching patterns.
 * Use these presets to ensure consistent behavior across the application.
 *
 * @example
 * // For real-time data that needs frequent updates
 * useSWR('core-state', fetcher, swrPresets.realtime)
 *
 * // For cached data that doesn't change often
 * useSWR('tools', fetcher, swrPresets.cached)
 */
export const swrPresets = {
  /**
   * Real-time preset: For data that needs frequent polling and immediate updates.
   * Use for: Core state, active connections, live metrics
   */
  realtime: {
    refreshInterval: 5000,
    revalidateOnFocus: true,
    revalidateOnReconnect: true,
  },

  /**
   * Cached preset: For data that changes infrequently.
   * Dedupes requests and avoids unnecessary refetches.
   * Use for: Tool lists, resource lists, server summaries
   */
  cached: {
    revalidateOnFocus: false,
    dedupingInterval: 10000,
    revalidateOnReconnect: true,
  },

  /**
   * Fast-cached preset: Similar to cached but with shorter dedup interval.
   * Use for: Active clients, runtime status
   */
  fastCached: {
    revalidateOnFocus: false,
    dedupingInterval: 5000,
    revalidateOnReconnect: true,
  },

  /**
   * Fast-realtime preset: For high-frequency data that needs very frequent updates.
   * Use for: Live runtime status, server health metrics
   */
  fastRealtime: {
    refreshInterval: 2000,
    revalidateOnFocus: false,
    dedupingInterval: 1000,
    revalidateOnReconnect: true,
  },

  /**
   * Static preset: For data that is managed externally (e.g., via events).
   * Completely disables automatic revalidation.
   * Use for: Log entries (updated via Wails events), configuration cache
   */
  static: {
    revalidateIfStale: false,
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
  },

  /**
   * Long-cached preset: For data that changes very infrequently.
   * Use for: App info, static configuration
   */
  longCached: {
    revalidateOnFocus: false,
    dedupingInterval: 30000,
    revalidateOnReconnect: true,
  },
} as const satisfies Record<string, SWRConfiguration>

/**
 * Type helper for SWR preset names
 */
export type SWRPresetName = keyof typeof swrPresets

/**
 * Get a SWR preset by name with type safety
 */
export function getSWRPreset(name: SWRPresetName): SWRConfiguration {
  return swrPresets[name]
}

/**
 * Merge custom options with a preset
 */
export function withSWRPreset(
  preset: SWRPresetName,
  options?: SWRConfiguration,
): SWRConfiguration {
  return {
    ...swrPresets[preset],
    ...options,
  }
}
