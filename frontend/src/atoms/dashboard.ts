// Input: jotai atom primitives, WailsService bindings
// Output: Dashboard state atoms (info, tools, resources, prompts, logs)
// Position: Global state for dashboard data

import type {
  InfoResponse,
  PromptEntry,
  ResourceEntry,
  ToolEntry,
} from '@bindings/mcpd/internal/ui'
import { atom } from 'jotai'

export interface LogEntry {
  id: string
  timestamp: Date
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
  source?: string
}

export const appInfoAtom = atom<InfoResponse | null>(null)
export const toolsAtom = atom<ToolEntry[]>([])
export const resourcesAtom = atom<ResourceEntry[]>([])
export const promptsAtom = atom<PromptEntry[]>([])
export const logsAtom = atom<LogEntry[]>([])

export const toolsCountAtom = atom(get => get(toolsAtom).length)
export const resourcesCountAtom = atom(get => get(resourcesAtom).length)
export const promptsCountAtom = atom(get => get(promptsAtom).length)

export const recentLogsAtom = atom(get => get(logsAtom).slice(0, 10))
