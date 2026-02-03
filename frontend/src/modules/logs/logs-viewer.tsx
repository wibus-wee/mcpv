// Input: log stream data, log viewer hooks, resizable panel hooks, analytics
// Output: LogsViewer component with table, detail panel, and bottom panel
// Position: Logs module main view for /logs route

import { useSetAtom } from 'jotai'
import { useCallback, useDeferredValue, useEffect, useRef, useState } from 'react'

import { logStreamTokenAtom } from '@/atoms/logs'
import { useCoreState } from '@/hooks/use-core-state'
import { useLogs } from '@/hooks/use-logs'
import { useResizable } from '@/hooks/use-resizable'
import { AnalyticsEvents, track } from '@/lib/analytics'
import { cn } from '@/lib/utils'

import {
  LogDetailPanel,
  LogsBottomPanel,
  LogTableRow,
  LogToolbar,
} from './components'
import { logTableColumnsClassName } from './components/log-table-row'
import { useKeyboardNavigation, useLogViewer } from './hooks'
import type { LogFilters } from './types'

function trackFilterChange<T>(
  filterName: string,
  prevValue: T,
  nextValue: T,
  transformValue?: (value: T) => string | number | boolean,
) {
  if (prevValue !== nextValue) {
    track(AnalyticsEvents.LOGS_FILTER_CHANGED, {
      filter: filterName,
      value: transformValue ? transformValue(nextValue) : nextValue,
    })
  }
}

export function LogsViewer() {
  const { logs, mutate } = useLogs()
  const { coreStatus } = useCoreState()
  const bumpLogStreamToken = useSetAtom(logStreamTokenAtom)
  const [isBottomPanelOpen, setIsBottomPanelOpen] = useState(false)
  const [isBottomPanelDismissed, setIsBottomPanelDismissed] = useState(false)
  const filtersRef = useRef<LogFilters | null>(null)
  const searchTrackTimerRef = useRef<number | null>(null)
  const lastSelectedLogIdRef = useRef<string | null>(null)
  const lastDetailOpenRef = useRef(false)

  const {
    // Filter state
    filters,
    setFilters,
    filteredLogs,
    serverOptions,
    logCounts,

    // Selection state
    selectedId,
    selectedLog,
    selectLog,
    selectPrev,
    selectNext,
    clearSelection,
    hasPrev,
    hasNext,

    // Scroll state
    // autoScroll,
    // setAutoScroll,
  } = useLogViewer(logs)

  useEffect(() => {
    filtersRef.current = filters
  }, [filters])

  useEffect(() => {
    return () => {
      if (searchTrackTimerRef.current !== null) {
        window.clearTimeout(searchTrackTimerRef.current)
        searchTrackTimerRef.current = null
      }
    }
  }, [])

  // Connection status
  const connectionStatus = (() => {
    if (coreStatus === 'stopped' || coreStatus === 'error') return 'disconnected'
    if (coreStatus === 'running' && logs.length > 0) return 'connected'
    return 'waiting'
  })() as 'connected' | 'disconnected' | 'waiting'

  // Actions
  const handleClear = useCallback(() => {
    mutate([], { revalidate: false })
    track(AnalyticsEvents.LOGS_CLEAR, { log_count: logs.length })
  }, [mutate, logs.length])

  const handleRefresh = useCallback(() => {
    bumpLogStreamToken(value => value + 1)
    track(AnalyticsEvents.LOGS_STREAM_REFRESH, { result: 'requested' })
  }, [bumpLogStreamToken])

  const handleFiltersChange = useCallback((next: LogFilters) => {
    const prev = filtersRef.current
    if (prev) {
      // Track simple filter changes
      trackFilterChange('level', prev.level, next.level)
      trackFilterChange('source', prev.source, next.source)
      trackFilterChange('server', prev.server, next.server, value =>
        value === 'all' ? 'all' : 'custom')

      // Track search changes with debouncing
      if (prev.search !== next.search) {
        if (searchTrackTimerRef.current !== null) {
          window.clearTimeout(searchTrackTimerRef.current)
        }
        searchTrackTimerRef.current = window.setTimeout(() => {
          track(AnalyticsEvents.LOGS_SEARCH, {
            query_len: next.search.trim().length,
            has_query: next.search.trim().length > 0,
          })
        }, 400)
      }
    }
    filtersRef.current = next
    setFilters(next)
  }, [setFilters])

  const handleSelectLog = useCallback(
    (id: string | null) => {
      selectLog(id)
    },
    [selectLog],
  )

  const deferredLogs = useDeferredValue(filteredLogs)
  const deferredLogCounts = useDeferredValue(logCounts)
  const deferredServerOptions = useDeferredValue(serverOptions)

  useKeyboardNavigation({
    onUp: selectPrev,
    onDown: selectNext,
    onEscape: clearSelection,
    enabled: deferredLogs.length > 0,
  })

  const {
    size: detailPanelWidth,
    resizeHandleProps: detailResizeHandleProps,
    isDragging: isDetailDragging,
  } = useResizable({
    defaultSize: 384,
    minSize: 320,
    maxSize: 560,
    storageKey: 'logs-detail-panel-width',
    direction: 'horizontal',
    handle: 'left',
  })

  const {
    size: bottomPanelHeight,
    resizeHandleProps: bottomResizeHandleProps,
    isDragging: isBottomDragging,
  } = useResizable({
    defaultSize: 220,
    minSize: 140,
    maxSize: 420,
    storageKey: 'logs-bottom-panel-height',
    direction: 'vertical',
    handle: 'top',
  })

  useEffect(() => {
    if (!selectedLog) {
      setIsBottomPanelOpen(false)
      setIsBottomPanelDismissed(false)
      return
    }

    if (!isBottomPanelOpen && !isBottomPanelDismissed) {
      setIsBottomPanelOpen(true)
    }
  }, [isBottomPanelDismissed, isBottomPanelOpen, selectedLog])

  useEffect(() => {
    const isOpen = Boolean(selectedLog)
    if (isOpen !== lastDetailOpenRef.current) {
      track(AnalyticsEvents.LOG_DETAIL_TOGGLE, { open: isOpen })
      lastDetailOpenRef.current = isOpen
    }
    if (selectedLog && selectedLog.id !== lastSelectedLogIdRef.current) {
      track(AnalyticsEvents.LOG_ROW_SELECTED, {
        level: selectedLog.level,
        source: selectedLog.source,
      })
      lastSelectedLogIdRef.current = selectedLog.id
    }
    if (!selectedLog) {
      lastSelectedLogIdRef.current = null
    }
  }, [selectedLog])

  const handleBottomPanelToggle = useCallback(
    (open: boolean) => {
      setIsBottomPanelOpen(open)
      setIsBottomPanelDismissed(!open)
      track(AnalyticsEvents.LOGS_BOTTOM_PANEL_TOGGLE, { open })
    },
    [],
  )

  return (
    <div className="flex h-full flex-col overflow-hidden bg-background">
      {/* Toolbar */}
      <LogToolbar
        filters={filters}
        onFiltersChange={handleFiltersChange}
        serverOptions={deferredServerOptions}
        logCounts={deferredLogCounts}
        // autoScroll={autoScroll}
        // onAutoScrollChange={setAutoScroll}
        connectionStatus={connectionStatus}
        onClear={handleClear}
        onRefresh={handleRefresh}
      />

      {/* Main content area */}
      <div
        className={cn(
          'relative min-h-0 flex-1',
          (isDetailDragging || isBottomDragging) && 'select-none',
        )}
      >
        {/* Left: Log list */}
        <div className="flex h-full min-w-0 flex-1 flex-col">
          {/* Table header */}
          <div
            className={cn(
              'h-8 shrink-0 border-b bg-muted/30 px-4 text-xs text-muted-foreground',
              logTableColumnsClassName,
            )}
          >
            <div>Time</div>
            <div className="text-center">Status</div>
            <div>Host</div>
            <div>Request</div>
            <div>Messages</div>
          </div>

          {/* Virtual list */}
          {deferredLogs.length > 0
            ? (
                <div className="min-h-0 flex-1 overflow-y-auto">
                  {deferredLogs.map(log => (
                    <LogTableRow
                      key={log.id}
                      log={log}
                      isSelected={log.id === selectedId}
                      onSelectLog={handleSelectLog}
                      columnsClassName={logTableColumnsClassName}
                    />
                  ))}
                </div>
              )
            : (
                <div className="flex min-h-0 flex-1 items-center justify-center text-muted-foreground">
                  {connectionStatus === 'disconnected'
                    ? 'Core is stopped'
                    : connectionStatus === 'waiting'
                      ? 'Waiting for logs...'
                      : 'No logs found'}
                </div>
              )}

          {/* Bottom logs panel */}
          {selectedLog && (
            <LogsBottomPanel
              log={selectedLog}
              isOpen={isBottomPanelOpen}
              onOpenChange={handleBottomPanelToggle}
              panelHeight={bottomPanelHeight}
              resizeHandleProps={bottomResizeHandleProps}
              isDragging={isBottomDragging}
              detailPanelWidth={detailPanelWidth}
              isDetailPanelOpen={!!selectedLog}
            />
          )}
        </div>

        {/* Right: Detail panel */}
        {selectedLog && (
          <div
            className="absolute bottom-0 right-0 top-0 z-10 transition-[width] duration-200 ease-out"
            style={{ width: `${detailPanelWidth}px` }}
          >
            <div {...detailResizeHandleProps} />
            <LogDetailPanel
              log={selectedLog}
              onClose={clearSelection}
              onNavigatePrev={selectPrev}
              onNavigateNext={selectNext}
              hasPrev={hasPrev}
              hasNext={hasNext}
            />
          </div>
        )}
      </div>
    </div>
  )
}
