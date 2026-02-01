// Input: LogEntry data, navigation handlers, UI primitives
// Output: LogDetailPanel component showing log metadata and trace
// Position: Right-side detail panel in logs module

import {
  CheckIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  CopyIcon,
  GlobeIcon,
  ShieldIcon,
  XIcon,
} from 'lucide-react'
import { useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'

import type { LogEntry } from '../types'
import { formatDateTime, formatDuration, getVisibleFields } from '../utils'

interface LogDetailPanelProps {
  log: LogEntry | null
  onClose: () => void
  onNavigatePrev?: () => void
  onNavigateNext?: () => void
  hasPrev?: boolean
  hasNext?: boolean
}

export function LogDetailPanel({
  log,
  onClose,
  onNavigatePrev,
  onNavigateNext,
  hasPrev,
  hasNext,
}: LogDetailPanelProps) {
  const [copiedField, setCopiedField] = useState<string | null>(null)

  if (!log) return null

  const handleCopy = async (key: string, value: string) => {
    await navigator.clipboard.writeText(value)
    setCopiedField(key)
    setTimeout(() => setCopiedField(null), 2000)
  }

  const visibleFields = getVisibleFields(log.fields)
  const duration = typeof log.fields.duration_ms === 'number' ? log.fields.duration_ms : null

  return (
    <div className="flex h-full w-full flex-col border-l bg-background">
      {/* Header */}
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex items-center gap-2">
          <Badge variant={log.level === 'error' ? 'error' : 'success'} size="sm">
            {getLevelCode(log.level)}
          </Badge>
          <span className="font-mono text-sm">{log.logger || log.serverType || '/'}</span>
        </div>

        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon-xs"
            onClick={onNavigatePrev}
            disabled={!hasPrev}
          >
            <ChevronUpIcon className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon-xs"
            onClick={onNavigateNext}
            disabled={!hasNext}
          >
            <ChevronDownIcon className="size-4" />
          </Button>
          <Separator orientation="vertical" className="mx-1 h-4" />
          <Button variant="ghost" size="icon-xs" onClick={onClose}>
            <XIcon className="size-4" />
          </Button>
        </div>
      </div>

      {/* Content */}
      <ScrollArea className="flex-1">
        <div className="p-4">
          {/* Request started */}
          <div className="mb-4 text-xs text-muted-foreground">
            <span className="mr-2">○</span>
            Request started
            <span className="ml-2 tabular-nums">{formatDateTime(log.timestamp)}</span>
          </div>

          {/* General Metadata */}
          <MetadataSection title="">
            <MetadataRow
              label="Request ID"
              value={log.id}
              onCopy={handleCopy}
              isCopied={copiedField === 'id'}
            />
            <MetadataRow
              label="Path"
              value={log.logger || '/'}
              onCopy={handleCopy}
              isCopied={copiedField === 'path'}
            />
            <MetadataRow
              label="Host"
              value={log.serverType || 'localhost'}
              onCopy={handleCopy}
              isCopied={copiedField === 'host'}
            />
            <MetadataRow
              label="Source"
              value={log.source}
              onCopy={handleCopy}
              isCopied={copiedField === 'source'}
            />
            {log.stream && (
              <MetadataRow
                label="Stream"
                value={log.stream}
                onCopy={handleCopy}
                isCopied={copiedField === 'stream'}
              />
            )}
          </MetadataSection>

          {/* Search Params / Fields as tags */}
          {visibleFields.length > 0 && (
            <>
              <Separator className="my-4" />
              <div className="mb-2 text-xs text-muted-foreground">Fields</div>
              <div className="flex flex-wrap gap-1.5">
                {visibleFields.map(([key, value]) => (
                  <Badge key={key} variant="outline" size="sm" className="font-mono">
                    {key}
                    <span className="ml-1 text-muted-foreground">
                      {truncateValue(String(value), 20)}
                    </span>
                  </Badge>
                ))}
              </div>
            </>
          )}

          <Separator className="my-4" />

          {/* Execution Trace */}
          <ExecutionTrace log={log} duration={duration} />
        </div>
      </ScrollArea>
    </div>
  )
}

// Metadata section wrapper
function MetadataSection({
  title,
  children,
}: {
  title: string
  children: React.ReactNode
}) {
  return (
    <div className="rounded-lg border bg-card">
      {title && (
        <div className="border-b px-3 py-2 text-xs font-medium text-muted-foreground">
          {title}
        </div>
      )}
      <div className="divide-y">{children}</div>
    </div>
  )
}

// Single metadata row
function MetadataRow({
  label,
  value,
  onCopy,
  isCopied,
}: {
  label: string
  value: string
  onCopy?: (key: string, value: string) => void
  isCopied?: boolean
}) {
  return (
    <div className="group flex items-center justify-between gap-2 px-3 py-2">
      <span className="shrink-0 text-xs text-muted-foreground">{label}</span>
      <div className="flex min-w-0 items-center gap-1">
        <span className="truncate font-mono text-xs">{value}</span>
        {onCopy && (
          <Button
            variant="ghost"
            size="icon-xs"
            className="size-5 shrink-0 opacity-0 group-hover:opacity-100"
            onClick={() => onCopy(label, value)}
          >
            {isCopied ? (
              <CheckIcon className="size-3" />
            ) : (
              <CopyIcon className="size-3" />
            )}
          </Button>
        )}
      </div>
    </div>
  )
}

// Execution trace timeline
function ExecutionTrace({ log, duration }: { log: LogEntry, duration: number | null }) {
  return (
    <div className="space-y-3">
      {/* Received */}
      <TraceStep
        icon={<GlobeIcon className="size-3.5" />}
        label="Received"
        sublabel="localhost"
      />

      <TraceStep
        icon={<ShieldIcon className="size-3.5" />}
        label="Firewall"
        value="Allowed"
      />

      {log.source === 'core' ? (
        <TraceCard
          icon="M"
          label="Middleware"
          status={log.level === 'error' ? 'error' : 'success'}
          statusCode={log.level === 'error' ? 500 : 200}
        />
      ) : null}

      {/* Function Invocation */}
      {(log.source === 'downstream' || log.fields.instanceID) ? (
        <TraceCard
          icon="F"
          label="Function Invocation"
          status={log.level === 'error' ? 'error' : 'success'}
          statusCode={log.level === 'error' ? 500 : 200}
        >
          <div className="mt-2 space-y-1.5 text-xs">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Route</span>
              <Badge variant="outline" size="sm">/{log.serverType || 'function'}</Badge>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Runtime</span>
              <span className="flex items-center gap-1">
                <span className="size-2 rounded-full bg-emerald-500" />
                MCP Server
              </span>
            </div>
            {duration !== null && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Execution Duration</span>
                <span className="tabular-nums">{formatDuration(duration)}</span>
              </div>
            )}
            {log.fields.instanceID != null && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Instance ID</span>
                <span className="font-mono">{String(log.fields.instanceID).slice(0, 8)}</span>
              </div>
            )}
          </div>
        </TraceCard>
      ) : null}

      {/* Response */}
      <div className="flex items-center gap-2 text-xs">
        <span className={cn(
          'size-2 rounded-full',
          log.level === 'error' ? 'bg-red-500' : 'bg-emerald-500',
        )}
        />
        <span>
          Response finished
          {duration !== null && (
            <span className="ml-1 text-muted-foreground">in {formatDuration(duration)}</span>
          )}
        </span>
      </div>
    </div>
  )
}

// Simple trace step
function TraceStep({
  icon,
  label,
  sublabel,
  value,
}: {
  icon: React.ReactNode
  label: string
  sublabel?: string
  value?: string
}) {
  return (
    <div className="flex items-center gap-3 text-xs">
      <div className="flex size-6 items-center justify-center rounded border bg-muted">
        {icon}
      </div>
      <div className="flex flex-1 items-center justify-between">
        <span>
          {label}
          {sublabel && (
            <span className="ml-1 text-muted-foreground">({sublabel})</span>
          )}
        </span>
        {value && <span className="text-muted-foreground">{value}</span>}
      </div>
    </div>
  )
}

// Trace card with more details
function TraceCard({
  icon,
  label,
  status,
  statusCode,
  children,
}: {
  icon: string
  label: string
  status: 'success' | 'error'
  statusCode: number
  children?: React.ReactNode
}) {
  return (
    <div className="rounded-lg border bg-card">
      <div className="flex items-center justify-between px-3 py-2">
        <div className="flex items-center gap-2">
          <span className="inline-flex size-5 items-center justify-center rounded border bg-muted text-[10px] font-medium">
            {icon}
          </span>
          <span className="text-sm font-medium">{label}</span>
        </div>
        <Badge
          variant={status === 'error' ? 'error' : 'success'}
          size="sm"
        >
          {statusCode}
        </Badge>
      </div>
      {children && <div className="border-t px-3 py-2">{children}</div>}
    </div>
  )
}

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

function truncateValue(value: string, maxLength: number) {
  if (value.length <= maxLength) return value
  return `${value.slice(0, maxLength)}...`
}
