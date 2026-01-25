// Input: Jotai atoms, SWR configuration
// Output: useSWRAtom hook - bridges SWR data fetching with Jotai state management
// Position: Shared hook - eliminates repeated SWR + useEffect + setAtom pattern

import type { PrimitiveAtom } from 'jotai'
import { useSetAtom } from 'jotai'
import { useEffect } from 'react'
import type { SWRConfiguration, SWRResponse } from 'swr'
import useSWR from 'swr'

import { jotaiStore } from '@/lib/jotai'
import type { SWRPresetName } from '@/lib/swr-config'
import { swrPresets } from '@/lib/swr-config'

/**
 * Options for useSWRAtom hook
 */
interface UseSWRAtomOptions<T> extends SWRConfiguration<T> {
  /** Use a preset configuration instead of custom options */
  preset?: SWRPresetName
  /** Transform data before storing in atom */
  transform?: (data: T) => T
  /** Only sync to atom when condition is true */
  syncWhen?: (data: T) => boolean
}

/**
 * Custom hook that combines SWR data fetching with Jotai atom synchronization.
 * Eliminates the repeated pattern of useSWR + useEffect + setAtom.
 *
 * @example
 * // Basic usage - syncs SWR data to atom automatically
 * const serversAtom = atom<ServerSummary[]>([])
 *
 * export function useServers() {
 *   return useSWRAtom(
 *     'servers',
 *     () => ServerService.ListServers(),
 *     serversAtom,
 *     { preset: 'cached' }
 *   )
 * }
 *
 * @example
 * // With data transformation
 * export function useHealthyServers() {
 *   return useSWRAtom(
 *     'servers',
 *     () => ServerService.ListServers(),
 *     healthyServersAtom,
 *     {
 *       preset: 'cached',
 *       transform: (servers) => servers.filter(s => !s.disabled),
 *     }
 *   )
 * }
 */
export function useSWRAtom<T>(
  key: string | null,
  fetcher: (() => Promise<T>) | null,
  atom: PrimitiveAtom<T>,
  options?: UseSWRAtomOptions<T>,
): SWRResponse<T> {
  const { preset, transform, syncWhen, ...swrOptions } = options ?? {}
  const setAtom = useSetAtom(atom)

  // Merge preset with custom options
  const finalOptions: SWRConfiguration<T> = {
    ...(preset ? swrPresets[preset] : {}),
    ...swrOptions,
  }

  const swr = useSWR<T>(key, fetcher, finalOptions)

  // Sync SWR data to Jotai atom
  useEffect(() => {
    if (swr.data === undefined) return

    // Check sync condition if provided
    if (syncWhen && !syncWhen(swr.data)) return

    // Apply transform if provided
    const dataToStore = transform ? transform(swr.data) : swr.data
    setAtom(dataToStore)
  }, [swr.data, setAtom, transform, syncWhen])

  return swr
}

/**
 * Get current atom value outside of React (for imperative code)
 */
export function getAtomValue<T>(atom: PrimitiveAtom<T>): T {
  return jotaiStore.get(atom)
}

/**
 * Set atom value outside of React (for imperative code)
 */
export function setAtomValue<T>(atom: PrimitiveAtom<T>, value: T): void {
  jotaiStore.set(atom, value)
}

/**
 * Options for useSWRWithFallback hook
 */
interface UseSWRWithFallbackOptions<T> extends SWRConfiguration<T> {
  preset?: SWRPresetName
}

/**
 * SWR hook with built-in fallback value support.
 * Returns fallback while loading, then actual data.
 *
 * @example
 * const { data } = useSWRWithFallback('path', fetcher, '/default/path')
 * // data is always defined (fallback or actual value)
 */
export function useSWRWithFallback<T>(
  key: string | null,
  fetcher: (() => Promise<T>) | null,
  fallbackValue: T,
  options?: UseSWRWithFallbackOptions<T>,
): SWRResponse<T> & { data: T } {
  const { preset, ...swrOptions } = options ?? {}

  const finalOptions: SWRConfiguration<T> = {
    ...(preset ? swrPresets[preset] : {}),
    ...swrOptions,
    fallbackData: fallbackValue,
  }

  const swr = useSWR<T>(key, fetcher, finalOptions)

  // Type assertion is safe because fallbackData ensures data is always defined
  return swr as SWRResponse<T> & { data: T }
}
