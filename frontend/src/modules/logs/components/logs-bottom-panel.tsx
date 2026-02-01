// Input: selected LogEntry, open state, resize handle props
// Output: LogsBottomPanel component with resizable details
// Position: Bottom panel in logs viewer

import { ChevronDownIcon, ChevronUpIcon } from 'lucide-react'

import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'

import type { LogEntry } from '../types'

interface LogsBottomPanelProps {
  log: LogEntry | null
  isOpen: boolean
  onOpenChange: (open: boolean) => void
  panelHeight?: number
  resizeHandleProps?: {
    onMouseDown: (e: React.MouseEvent) => void
    onDoubleClick: (e: React.MouseEvent) => void
    className: string
  }
  isDragging?: boolean
  detailPanelWidth?: number
  isDetailPanelOpen?: boolean
}

export function LogsBottomPanel({
  log,
  isOpen,
  onOpenChange,
  panelHeight = 220,
  resizeHandleProps,
  isDragging = false,
  detailPanelWidth = 0,
  isDetailPanelOpen = false,
}: LogsBottomPanelProps) {
  const hasError = log?.level === 'error'
  const hasWarning = log?.level === 'warn'
  const shouldShow = hasError || hasWarning

  // Auto show/hide based on log level
  if (!shouldShow && isOpen) {
    onOpenChange(false)
  }

  return (
    <div
      className={cn('relative', isDragging && 'select-none')}
      style={{
        marginRight: isDetailPanelOpen ? `${detailPanelWidth}px` : 0,
      }}
    >
      {isOpen && resizeHandleProps && <div {...resizeHandleProps} />}
      {/* Header */}
      <div className="flex h-10 items-center justify-between border-t bg-muted/30 px-4">
        <button
          type="button"
          className="flex items-center gap-2 text-sm font-medium"
          onClick={() => onOpenChange(!isOpen)}
        >
          <span>Console</span>
          {isOpen ? (
            <ChevronDownIcon className="size-4 text-muted-foreground" />
          ) : (
            <ChevronUpIcon className="size-4 text-muted-foreground" />
          )}
        </button>
      </div>

      {/* Content */}
      <div
        className="border-t bg-background"
        style={{
          height: isOpen ? panelHeight : 0,
          opacity: isOpen ? 1 : 0,
          overflow: 'hidden',
          pointerEvents: isOpen ? 'auto' : 'none',
        }}
      >
        <ScrollArea className="h-full">
          {log ? (
            <div
              className={cn(
                'p-4',
                hasError && 'bg-red-500/5',
                hasWarning && 'bg-amber-500/5',
              )}
            >
              <pre
                className={cn(
                  'whitespace-pre-wrap break-words font-mono text-xs leading-relaxed',
                  hasError && 'text-red-500',
                  hasWarning && 'text-amber-500',
                )}
              >
                {log.message}
              </pre>
            </div>
          ) : (
            <div className="flex h-full items-center justify-center p-4 text-sm text-muted-foreground">
              Select a log entry to view details
            </div>
          )}
        </ScrollArea>
      </div>
    </div>
  )
}
