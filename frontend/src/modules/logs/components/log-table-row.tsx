// Input: LogEntry row data and selection handlers
// Output: LogTableRow component rendering a single log row
// Position: Log list row in logs module table view

import { AlertCircleIcon, AlertTriangleIcon, CheckCircleIcon, InfoIcon, WorkflowIcon, ZapIcon } from 'lucide-react'
import { memo, useCallback, useId } from 'react'

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import type { LogEntry } from '../types'
import { formatInlineFields, formatInlineMessage, formatTime } from '../utils'

interface LogTableRowProps {
  log: LogEntry
  isSelected?: boolean
  onSelectLog: (id: string | null) => void
  columnsClassName: string
}

// Status code color mapping (similar to HTTP status codes)
function getStatusColor(level: LogEntry['level']) {
  switch (level) {
    case 'error':
      return 'text-red-500'
    case 'warn':
      return 'text-amber-500'
    case 'info':
      return 'text-emerald-500'
    default:
      return 'text-muted-foreground'
  }
}

// Status icon mapping
function getLevelIcon(level: LogEntry['level']) {
  switch (level) {
    case 'error':
      return <AlertCircleIcon className="size-4" />
    case 'warn':
      return <AlertTriangleIcon className="size-4" />
    case 'info':
      return <CheckCircleIcon className="size-4" />
    default:
      return <InfoIcon className="size-4" />
  }
}

// Get status code-like display
function getLevelCode(level: LogEntry['level']) {
  switch (level) {
    case 'error':
      return '500'
    case 'warn':
      return '400'
    case 'info':
      return '200'
    default:
      return '---'
  }
}

export const logTableColumnsClassName
  = 'grid grid-cols-[140px_64px_140px_270px_minmax(0,1fr)] items-center'

export const LogTableRow = memo(function LogTableRow({
  log,
  isSelected,
  onSelectLog,
  columnsClassName,
}: LogTableRowProps) {
  // Build trace nodes from log for display
  const traceNodes = buildTraceIcons(log)
  const baseId = useId()

  const hasError = log.level === 'error'
  const hasWarning = log.level === 'warn'
  const messageText = formatInlineMessage(log.message)
  const inlineFields = formatInlineFields(log.fields)
  const message = inlineFields
    ? (messageText ? `${messageText} | ${inlineFields}` : inlineFields)
    : messageText
  const handleClick = useCallback(() => {
    onSelectLog(isSelected ? null : log.id)
  }, [isSelected, log.id, onSelectLog])

  return (
    <div
      className={cn(
        'group h-9 w-full cursor-pointer border-b border-border/40 px-4 text-sm transition-colors hover:bg-muted/50',
        columnsClassName,
        isSelected && 'bg-accent',
        hasError && 'bg-red-500/5 hover:bg-red-500/10',
        hasWarning && 'bg-amber-500/5 hover:bg-amber-500/10',
      )}
      onClick={handleClick}
    >
      {/* Time */}
      <div className="font-mono text-xs text-muted-foreground tabular-nums">
        {formatTime(log.timestamp)}
      </div>

      {/* Status */}
      <div className="flex items-center justify-center gap-2">
        {(log.level === 'error' || log.level === 'warn') && (
          <span className={cn(getStatusColor(log.level), '-ml-6')}>
            {getLevelIcon(log.level)}
          </span>
        )}
        <span className={cn('font-mono text-xs font-medium', getStatusColor(log.level))}>
          {getLevelCode(log.level)}
        </span>
      </div>

      {/* Host / Server */}
      <div className="truncate text-xs text-muted-foreground">
        {log.serverType || log.source}
      </div>

      {/* Request / Message with trace icons */}
      <div className="flex min-w-0 items-center gap-2">
        {/* Trace icons */}
        {traceNodes.length > 0 && (
          <div className="flex shrink-0 items-center gap-0.5">
            {traceNodes.map(node => (
              <TraceIcon key={`${baseId}-${node.type}`} type={node.type} tooltip={node.tooltip} />
            ))}
          </div>
        )}

        {/* Request label */}
        <span className="truncate font-mono text-xs">
          {log.logger || log.serverType || 'request'}
        </span>
      </div>

      {/* Messages */}
      <div className="min-w-0 truncate text-xs">
        <span className={cn('font-mono', hasError && 'text-red-500')}>
          {truncateMessage(message, 160)}
        </span>
      </div>
    </div>
  )
})

// Trace icon component
interface TraceIconProps {
  type: 'M' | 'F' | 'R'
  tooltip?: string
}

function TraceIcon({ type, tooltip }: TraceIconProps) {
  const icon = type === 'M' ? <WorkflowIcon className="size-3.5" /> : <ZapIcon className="size-3.5" />

  const content = (
    <span className="inline-flex size-5 items-center justify-center rounded border border-border p-1">
      {icon}
    </span>
  )

  if (tooltip) {
    return (
      <Tooltip>
        <TooltipTrigger delay={0} className="cursor-default">
          {content}
        </TooltipTrigger>
        <TooltipContent side="top">
          <p className="text-xs">{tooltip}</p>
        </TooltipContent>
      </Tooltip>
    )
  }

  return content
}

// Build trace icons from log entry
function buildTraceIcons(log: LogEntry) {
  const icons: Array<{ type: 'M' | 'F' | 'R', tooltip?: string }> = []

  // M = Middleware (routing)
  if (log.fields.event?.toString().includes('route') || log.source === 'core') {
    icons.push({ type: 'M', tooltip: 'Middleware' })
  }

  // F = Function (downstream/call)
  if (log.source === 'downstream' || log.fields.instanceID) {
    icons.push({ type: 'F', tooltip: log.serverType || 'Function' })
  }

  return icons
}

function truncateMessage(message: string, maxLength: number) {
  if (message.length <= maxLength) return message
  return `${message.slice(0, maxLength)}...`
}
