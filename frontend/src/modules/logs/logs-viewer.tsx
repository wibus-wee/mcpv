// Input: log stream data, log viewer hooks, resizable panel hooks
// Output: LogsViewer component with table, detail panel, and bottom panel
// Position: Logs module main view for /logs route

import { useSetAtom } from 'jotai'
import { useCallback, useDeferredValue, useEffect, useState } from 'react'

import { logStreamTokenAtom } from '@/atoms/logs'
import { useCoreState } from '@/hooks/use-core-state'
import { useLogs } from '@/hooks/use-logs'
import { useResizable } from '@/hooks/use-resizable'
import { cn } from '@/lib/utils'

import {
  LogDetailPanel,
  LogsBottomPanel,
  LogTableRow,
  LogToolbar,
} from './components'
import { logTableColumnsClassName } from './components/log-table-row'
import { useLogViewer } from './hooks'

export function LogsViewer() {
  const { logs, mutate } = useLogs()
  const { coreStatus } = useCoreState()
  const bumpLogStreamToken = useSetAtom(logStreamTokenAtom)
  const [isBottomPanelOpen, setIsBottomPanelOpen] = useState(false)
  const [isBottomPanelDismissed, setIsBottomPanelDismissed] = useState(false)

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

  // Connection status
  const connectionStatus = (() => {
    if (coreStatus === 'stopped' || coreStatus === 'error') return 'disconnected'
    if (coreStatus === 'running' && logs.length > 0) return 'connected'
    return 'waiting'
  })() as 'connected' | 'disconnected' | 'waiting'

  // Actions
  const handleClear = useCallback(() => {
    mutate([], { revalidate: false })
  }, [mutate])

  const handleRefresh = useCallback(() => {
    bumpLogStreamToken(value => value + 1)
  }, [bumpLogStreamToken])

  const handleSelectLog = useCallback(
    (id: string | null) => {
      selectLog(id)
    },
    [selectLog],
  )

  const deferredLogs = useDeferredValue(filteredLogs)
  const deferredLogCounts = useDeferredValue(logCounts)
  const deferredServerOptions = useDeferredValue(serverOptions)

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

  const handleBottomPanelToggle = useCallback(
    (open: boolean) => {
      setIsBottomPanelOpen(open)
      setIsBottomPanelDismissed(!open)
    },
    [],
  )

  return (
    <div className="flex h-full flex-col overflow-hidden bg-background">
      {/* Toolbar */}
      <LogToolbar
        filters={filters}
        onFiltersChange={setFilters}
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
