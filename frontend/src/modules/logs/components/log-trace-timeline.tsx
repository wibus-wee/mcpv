// Execution trace timeline for detail drawer
// Vercel style: vertical timeline with nodes for each execution phase

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

import type { LogEntry } from '../types'
import { formatDuration, formatTime } from '../utils'

interface LogTraceTimelineProps {
  log: LogEntry
}

interface TraceStep {
  id: string
  type: 'receive' | 'route' | 'instance' | 'call' | 'response'
  label: string
  sublabel?: string
  time?: Date
  duration?: number
  status: 'success' | 'error' | 'pending'
  details?: Array<{ key: string, value: string }>
}

export function LogTraceTimeline({ log }: LogTraceTimelineProps) {
  const steps = buildTraceSteps(log)

  if (steps.length === 0) {
    return (
      <div className="py-4 text-center text-sm text-muted-foreground">
        No trace information available
      </div>
    )
  }

  return (
    <div className="relative pl-6">
      {/* Vertical line */}
      <div className="absolute bottom-4 left-2.5 top-4 w-px bg-border" />

      {/* Steps */}
      <div className="space-y-4">
        {steps.map((step, index) => (
          <TraceStepNode
            key={step.id}
            step={step}
            isFirst={index === 0}
            isLast={index === steps.length - 1}
          />
        ))}
      </div>
    </div>
  )
}

interface TraceStepNodeProps {
  step: TraceStep
  isFirst?: boolean
  isLast?: boolean
}

function TraceStepNode({ step, isFirst: _isFirst, isLast: _isLast }: TraceStepNodeProps) {
  const statusColors = {
    success: 'bg-success border-success/50',
    error: 'bg-destructive border-destructive/50',
    pending: 'bg-warning border-warning/50',
  }

  return (
    <div className="relative">
      {/* Node dot */}
      <div
        className={cn(
          'absolute -left-6 top-1 size-2.5 rounded-full border-2',
          statusColors[step.status],
        )}
      />

      {/* Content */}
      <div className="rounded-lg border bg-card p-3">
        {/* Header */}
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <span className="font-medium text-sm">{step.label}</span>
            {step.sublabel && (
              <Badge variant="outline" size="sm">
                {step.sublabel}
              </Badge>
            )}
          </div>
          {step.duration !== undefined && (
            <span className="text-xs text-muted-foreground tabular-nums">
              {formatDuration(step.duration)}
            </span>
          )}
        </div>

        {/* Time */}
        {step.time && (
          <p className="mt-1 text-xs text-muted-foreground">
            {formatTime(step.time)}
          </p>
        )}

        {/* Details */}
        {step.details && step.details.length > 0 && (
          <div className="mt-2 space-y-1 border-t pt-2">
            {step.details.map(({ key, value }) => (
              <div
                key={key}
                className="flex items-center justify-between gap-2 text-xs"
              >
                <span className="text-muted-foreground">{key}</span>
                <span className="truncate font-mono">{value}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

/**
 * Build trace steps from log entry fields
 */
function buildTraceSteps(log: LogEntry): TraceStep[] {
  const steps: TraceStep[] = []
  const { fields } = log

  // Step 1: Receive (always present)
  steps.push({
    id: 'receive',
    type: 'receive',
    label: 'Request Received',
    time: log.timestamp,
    status: 'success',
  })

  // Step 2: Route (if routing info present)
  if (fields.event?.toString().includes('route')) {
    steps.push({
      id: 'route',
      type: 'route',
      label: 'Route',
      sublabel: log.serverType,
      status: fields.event === 'route_error' ? 'error' : 'success',
      duration: typeof fields.duration_ms === 'number' ? fields.duration_ms : undefined,
    })
  }

  // Step 3: Instance (if instance info present)
  if (fields.instanceID) {
    steps.push({
      id: 'instance',
      type: 'instance',
      label: 'Instance',
      sublabel: String(fields.state || ''),
      status: log.level === 'error' ? 'error' : 'success',
      details: [
        { key: 'Instance ID', value: String(fields.instanceID) },
        ...(fields.state ? [{ key: 'State', value: String(fields.state) }] : []),
      ],
    })
  }

  // Step 4: Function call (for downstream logs)
  if (log.source === 'downstream') {
    steps.push({
      id: 'call',
      type: 'call',
      label: 'Function Invocation',
      sublabel: log.serverType,
      status: log.level === 'error' ? 'error' : 'success',
      details: log.stream ? [{ key: 'Stream', value: log.stream }] : undefined,
    })
  }

  // Step 5: Response (if duration present, implies completion)
  if (fields.duration_ms !== undefined) {
    steps.push({
      id: 'response',
      type: 'response',
      label: log.level === 'error' ? 'Error Response' : 'Response',
      status: log.level === 'error' ? 'error' : 'success',
      duration: typeof fields.duration_ms === 'number' ? fields.duration_ms : undefined,
    })
  }

  return steps
}
