// Input: Card, Badge, Button, Checkbox, Select components, logs hook
// Output: LogsPanel component displaying real-time logs
// Position: Dashboard logs section with filtering

import { useSetAtom } from 'jotai'
import { RefreshCwIcon, ScrollTextIcon, TrashIcon } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'

import { logStreamTokenAtom } from '@/atoms/logs'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { useCoreState } from '@/hooks/use-core-state'
import type { LogEntry, LogSource } from '@/hooks/use-logs'
import { useLogs } from '@/hooks/use-logs'
import { cn } from '@/lib/utils'

const levelClassName: Record<LogEntry['level'], string> = {
  debug: 'text-muted-foreground',
  info: 'text-info',
  warn: 'text-warning',
  error: 'text-destructive',
}

const hiddenFieldKeys = new Set(['log_source', 'logger', 'serverType', 'stream', 'timestamp'])
const logRowSize = 32

type LogSegment = {
  text: string
  className: string
}

type LogRowData = {
  log: LogEntry
  segments: LogSegment[]
  lineLength: number
}

const formatFieldValue = (value: unknown) => {
  if (value === null) return 'null'
  if (value === undefined) return 'undefined'
  if (value instanceof Date) return value.toISOString()
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    }
    catch {
      return String(value)
    }
  }
  return String(value)
}

const formatInlineFields = (fields: Record<string, unknown>) => {
  const entries = Object.entries(fields).filter(([key]) => !hiddenFieldKeys.has(key))
  if (entries.length === 0) return ''
  return entries
    .map(([key, value]) => `${key}=${formatFieldValue(value)}`)
    .join(' ')
}

const formatInlineMessage = (message: string) => message.replace(/\n/g, '\\n')

const getLogSegments = (log: LogEntry): LogSegment[] => {
  const segments: LogSegment[] = [
    { text: log.timestamp.toLocaleTimeString(), className: 'text-muted-foreground' },
    { text: log.level.toUpperCase(), className: levelClassName[log.level] },
    { text: log.source, className: 'text-muted-foreground' },
  ]

  if (log.serverType) {
    segments.push({ text: `server=${log.serverType}`, className: 'text-muted-foreground' })
  }
  if (log.stream) {
    segments.push({ text: `stream=${log.stream}`, className: 'text-muted-foreground' })
  }
  if (log.logger) {
    segments.push({ text: `logger=${log.logger}`, className: 'text-muted-foreground' })
  }

  const inlineMessage = formatInlineMessage(log.message)
  segments.push({ text: inlineMessage, className: 'text-foreground' })

  const inlineFields = formatInlineFields(log.fields)
  if (inlineFields) {
    segments.push({ text: inlineFields, className: 'text-muted-foreground' })
  }

  return segments
}

const getSegmentsLength = (segments: LogSegment[]) => {
  if (segments.length === 0) return 0
  return segments.reduce((sum, segment) => sum + segment.text.length, 0) + (segments.length - 1)
}

function LogRow({ segments }: { segments: LogSegment[] }) {
  return (
    <div className="w-full border-b border-border/50 px-3 py-1 text-xs font-mono whitespace-pre leading-6">
      {segments.map((segment, index) => (
        <span key={`${segment.text}-${index}`} className={cn(segment.className, index === 1 && 'uppercase')}>
          {index === 0 ? segment.text : ` ${segment.text}`}
        </span>
      ))}
    </div>
  )
}

export function LogsPanel() {
  const { logs, mutate } = useLogs()
  const { coreStatus } = useCoreState()
  const [levelFilter, setLevelFilter] = useState<string>('all')
  const [sourceFilter, setSourceFilter] = useState<LogSource | 'all'>('all')
  const [serverFilter, setServerFilter] = useState<string>('all')
  const [autoScroll, setAutoScroll] = useState(true)
  const bumpLogStreamToken = useSetAtom(logStreamTokenAtom)
  const parentRef = useRef<HTMLDivElement | null>(null)
  const levelLabels: Record<string, string> = {
    all: 'All levels',
    debug: 'Debug',
    info: 'Info',
    warn: 'Warning',
    error: 'Error',
  }

  const sourceLabels: Record<string, string> = {
    all: 'All sources',
    core: 'Core',
    downstream: 'Downstream',
    ui: 'Wails UI',
    unknown: 'Unknown',
  }

  const serverOptions = useMemo(() => {
    return Array.from(
      new Set(
        logs
          .map(log => log.serverType)
          .filter((server): server is string => typeof server === 'string'),
      ),
    ).sort()
  }, [logs])

  const filteredLogs = useMemo(() => {
    return logs.filter((log) => {
      if (levelFilter !== 'all' && log.level !== levelFilter) {
        return false
      }
      if (sourceFilter !== 'all' && log.source !== sourceFilter) {
        return false
      }
      if (serverFilter !== 'all' && log.serverType !== serverFilter) {
        return false
      }
      return true
    })
  }, [levelFilter, logs, serverFilter, sourceFilter])

  const orderedLogs = useMemo(() => filteredLogs.slice().reverse(), [filteredLogs])
  const orderedLogRows = useMemo(() => {
    return orderedLogs.map((log): LogRowData => {
      const segments = getLogSegments(log)
      return {
        log,
        segments,
        lineLength: getSegmentsLength(segments),
      }
    })
  }, [orderedLogs])

  const maxLineLength = useMemo(() => {
    let max = 0
    for (const row of orderedLogRows) {
      if (row.lineLength > max) {
        max = row.lineLength
      }
    }
    return max
  }, [orderedLogRows])
  const listWidth = maxLineLength > 0 ? `${maxLineLength}ch` : '100%'

  const clearLogs = () => {
    mutate([], { revalidate: false })
  }


  const forceRefresh = () => {
    bumpLogStreamToken(value => value + 1)
  }

  const isConnected = coreStatus === 'running' && logs.length > 0
  const isDisconnected = coreStatus === 'stopped' || coreStatus === 'error'
  const isWaiting = coreStatus === 'running' && logs.length === 0
  const showServerFilter = serverOptions.length > 0
    && (sourceFilter === 'all' || sourceFilter === 'downstream')

  const virtualizer = useVirtualizer({
    count: orderedLogRows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => logRowSize,
    overscan: 12,
  })

  useEffect(() => {
    if (!autoScroll || orderedLogRows.length === 0) {
      return
    }
    // Scroll to the end (latest log)
    virtualizer.scrollToIndex(orderedLogRows.length - 1, {
      align: 'end',
      behavior: 'auto',
    })
  }, [autoScroll, orderedLogRows.length, virtualizer])

  useEffect(() => {
    if (sourceFilter !== 'downstream' && sourceFilter !== 'all' && serverFilter !== 'all') {
      setServerFilter('all')
    }
  }, [sourceFilter, serverFilter])

  const renderEmptyState = () => {
    if (isDisconnected) {
      return (
        <>
          <p className="text-sm font-medium">Core is not running</p>
          <p className="text-xs">Start the core to see logs</p>
        </>
      )
    }
    if (isWaiting) {
      return (
        <>
          <p className="text-sm font-medium">Waiting for logs...</p>
          <Button
            variant="ghost"
            size="sm"
            onClick={forceRefresh}
            className="mt-2"
          >
            <RefreshCwIcon className="size-3 mr-1" />
            Restart Log Stream
          </Button>
        </>
      )
    }
    return (
      <>
        <p className="text-sm">No logs yet</p>
        <p className="text-xs">Logs will appear here when the core is running</p>
      </>
    )
  }

  return (
    <div className="h-full">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <ScrollTextIcon className="size-5" />
              Logs
              <Badge variant="secondary" size="sm">
                {logs.length}
              </Badge>
              {isConnected && (
                <Badge variant="success" size="sm" className="ml-1">
                  Connected
                </Badge>
              )}
              {isDisconnected && (
                <Badge variant="error" size="sm" className="ml-1">
                  Disconnected
                </Badge>
              )}
              {isWaiting && (
                <Badge variant="warning" size="sm" className="ml-1">
                  Waiting...
                </Badge>
              )}
            </CardTitle>
            <div className="flex flex-wrap items-center gap-3">
              <div className="flex items-center gap-2">
                <Checkbox
                  id="auto-scroll"
                  checked={autoScroll}
                  onCheckedChange={checked => setAutoScroll(checked === true)}
                />
                <Label htmlFor="auto-scroll" className="text-sm">
                  Auto-scroll
                </Label>
              </div>
              <Select value={levelFilter} onValueChange={value => setLevelFilter(value || "all")}>
                <SelectTrigger size="sm" className="w-32">
                  <SelectValue>
                    {value =>
                      value
                        ? levelLabels[String(value)] ?? String(value)
                        : 'Filter level'}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All levels</SelectItem>
                  <SelectItem value="debug">Debug</SelectItem>
                  <SelectItem value="info">Info</SelectItem>
                  <SelectItem value="warn">Warning</SelectItem>
                  <SelectItem value="error">Error</SelectItem>
                </SelectContent>
              </Select>
              <Select
                value={sourceFilter}
                onValueChange={value => setSourceFilter(value as LogSource | 'all')}
              >
                <SelectTrigger size="sm" className="w-36">
                  <SelectValue>
                    {value =>
                      value
                        ? sourceLabels[String(value)] ?? String(value)
                        : 'Filter source'}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All sources</SelectItem>
                  <SelectItem value="core">Core</SelectItem>
                  <SelectItem value="downstream">Downstream</SelectItem>
                  <SelectItem value="ui">Wails UI</SelectItem>
                  <SelectItem value="unknown">Unknown</SelectItem>
                </SelectContent>
              </Select>
              {showServerFilter && (
                <Select value={serverFilter} onValueChange={(value) => value && setServerFilter(value)}>
                  <SelectTrigger size="sm" className="w-40">
                    <SelectValue>
                      {value => (value ? String(value) : 'Filter server')}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All servers</SelectItem>
                    {serverOptions.map(server => (
                      <SelectItem key={server} value={server}>
                        {server}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
              {isWaiting && (
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={forceRefresh}
                  title="Restart log stream"
                >
                  <RefreshCwIcon className="size-4" />
                </Button>
              )}
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={clearLogs}
              >
                <TrashIcon className="size-4" />
              </Button>
            </div>
          </div>
        </CardHeader>
        <Separator />
        <CardContent className="p-0">
          <div ref={parentRef} className="h-80 overflow-auto">
            {orderedLogRows.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full py-12 text-muted-foreground">
                <ScrollTextIcon className="size-8 mb-2 opacity-50" />
                {renderEmptyState()}
              </div>
            ) : (
              <div
                style={{
                  height: `${virtualizer.getTotalSize()}px`,
                  width: listWidth,
                  minWidth: '100%',
                  position: 'relative',
                }}
              >
                {virtualizer.getVirtualItems().map((virtualItem) => {
                  const row = orderedLogRows[virtualItem.index]
                  return (
                    <div
                      key={virtualItem.key}
                      data-index={virtualItem.index}
                      style={{
                        position: 'absolute',
                        top: 0,
                        left: 0,
                        width: '100%',
                        transform: `translateY(${virtualItem.start}px)`,
                      }}
                    >
                      <LogRow segments={row.segments} />
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
