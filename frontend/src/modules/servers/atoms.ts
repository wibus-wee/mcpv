// Input: jotai atom primitives, bindings types
// Output: Combined atoms for servers module
// Position: Global state for servers management

import type {
  ConfigModeResponse,
  ServerDetail,
} from '@bindings/mcpv/internal/ui/types'
import { atom } from 'jotai'

// Config mode and path
export const configModeAtom = atom<ConfigModeResponse | null>(null)

// Selected server detail
export const selectedServerAtom = atom<ServerDetail | null>(null)

// Loading states
export const configLoadingAtom = atom(false)
export const serverLoadingAtom = atom(false)
