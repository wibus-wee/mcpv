// Input: jotai atom primitives
// Output: Core status atoms (coreStatusAtom, coreErrorAtom)
// Position: Global state for mcpd core runtime status

import { atom } from 'jotai'

export type CoreStatus = 'stopped' | 'starting' | 'running' | 'stopping' | 'error'

export const coreStatusAtom = atom<CoreStatus>('stopped')
export const coreErrorAtom = atom<string | null>(null)
