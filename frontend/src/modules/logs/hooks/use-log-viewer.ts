// Hooks for log state management
// Provides filter, selection, and search state with memoized callbacks

import { useCallback, useMemo, useState } from 'react'

import type { LogEntry, LogFilters } from '../types'
import { countByLevel, extractServerNames, filterLogs } from '../utils'

/**
 * Hook for managing log filters
 */
export function useLogFilters(logs: LogEntry[]) {
  const [filters, setFilters] = useState<LogFilters>({
    level: 'all',
    source: 'all',
    server: 'all',
    search: '',
  })

  const filteredLogs = useMemo(
    () => filterLogs(logs, filters),
    [logs, filters],
  )

  const serverOptions = useMemo(
    () => extractServerNames(logs),
    [logs],
  )

  const logCounts = useMemo(() => ({
    total: logs.length,
    filtered: filteredLogs.length,
    byLevel: countByLevel(logs),
  }), [logs, filteredLogs])

  const resetFilters = useCallback(() => {
    setFilters({
      level: 'all',
      source: 'all',
      server: 'all',
      search: '',
    })
  }, [])

  return {
    filters,
    setFilters,
    filteredLogs,
    serverOptions,
    logCounts,
    resetFilters,
  }
}

/**
 * Hook for managing log selection state
 */
export function useLogSelection(logs: LogEntry[]) {
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const selectedLog = useMemo(
    () => logs.find(log => log.id === selectedId) ?? null,
    [logs, selectedId],
  )

  const selectedIndex = useMemo(
    () => logs.findIndex(log => log.id === selectedId),
    [logs, selectedId],
  )

  const selectLog = useCallback((id: string | null) => {
    setSelectedId(id)
  }, [])

  const selectPrev = useCallback(() => {
    if (selectedIndex > 0) {
      setSelectedId(logs[selectedIndex - 1].id)
    }
  }, [logs, selectedIndex])

  const selectNext = useCallback(() => {
    if (selectedIndex < logs.length - 1) {
      setSelectedId(logs[selectedIndex + 1].id)
    }
  }, [logs, selectedIndex])

  const clearSelection = useCallback(() => {
    setSelectedId(null)
  }, [])

  return {
    selectedId,
    selectedLog,
    selectedIndex,
    selectLog,
    selectPrev,
    selectNext,
    clearSelection,
    hasPrev: selectedIndex > 0,
    hasNext: selectedIndex < logs.length - 1,
  }
}

/**
 * Hook for managing expanded log rows
 */
export function useLogExpansion() {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set())

  const toggleExpand = useCallback((id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      }
      else {
        next.add(id)
      }
      return next
    })
  }, [])

  const expandAll = useCallback((ids: string[]) => {
    setExpandedIds(new Set(ids))
  }, [])

  const collapseAll = useCallback(() => {
    setExpandedIds(new Set())
  }, [])

  const isExpanded = useCallback(
    (id: string) => expandedIds.has(id),
    [expandedIds],
  )

  return {
    expandedIds,
    toggleExpand,
    expandAll,
    collapseAll,
    isExpanded,
  }
}

/**
 * Combined hook for all log viewer state
 */
export function useLogViewer(logs: LogEntry[]) {
  const [autoScroll, setAutoScroll] = useState(true)

  const {
    filters,
    setFilters,
    filteredLogs,
    serverOptions,
    logCounts,
    resetFilters,
  } = useLogFilters(logs)

  const {
    selectedId,
    selectedLog,
    selectLog,
    selectPrev,
    selectNext,
    clearSelection,
    hasPrev,
    hasNext,
  } = useLogSelection(filteredLogs)

  const {
    expandedIds,
    toggleExpand,
    collapseAll,
  } = useLogExpansion()

  return {
    // Filter state
    filters,
    setFilters,
    filteredLogs,
    serverOptions,
    logCounts,
    resetFilters,

    // Selection state
    selectedId,
    selectedLog,
    selectLog,
    selectPrev,
    selectNext,
    clearSelection,
    hasPrev,
    hasNext,

    // Expansion state
    expandedIds,
    toggleExpand,
    collapseAll,

    // Scroll state
    autoScroll,
    setAutoScroll,
  }
}
