// Input: ServerRuntimeStatus from bindings, useRuntimeStatus hook
// Output: ServerRuntimeStatus component with color-coded instance state indicators
// Position: Runtime status display component for server instances

import type { ServerRuntimeStatus } from '@bindings/mcpd/internal/ui'

import { cn } from '@/lib/utils'

import { useRuntimeStatus } from '../hooks'

const stateColors: Record<string, string> = {
  ready: 'bg-success',
  busy: 'bg-warning',
  starting: 'bg-info',
  draining: 'bg-muted-foreground',
  stopped: 'bg-muted-foreground/50',
  failed: 'bg-destructive',
}

const stateLabels: Record<string, string> = {
  ready: 'Ready',
  busy: 'Busy',
  starting: 'Starting',
  draining: 'Draining',
  stopped: 'Stopped',
  failed: 'Failed',
}

function StateDot({ state }: { state: string }) {
  const colorClass = stateColors[state] || 'bg-muted-foreground'
  return (
    <span
      className={cn('size-2 rounded-full shrink-0', colorClass)}
      title={stateLabels[state] || state}
    />
  )
}

interface ServerRuntimeIndicatorProps {
  serverName: string
  className?: string
}

export function ServerRuntimeIndicator({
  serverName,
  className,
}: ServerRuntimeIndicatorProps) {
  const { data: runtimeStatus } = useRuntimeStatus()
  console.log('Runtime status data:', runtimeStatus)
  const serverStatus = runtimeStatus?.find((s) => s.serverName === serverName)

  if (!serverStatus || serverStatus.instances.length === 0) {
    return null
  }

  return (
    <div className={cn('flex items-center gap-1', className)}>
      {serverStatus.instances.slice(0, 5).map((inst) => (
        <StateDot key={inst.id} state={inst.state} />
      ))}
      {serverStatus.instances.length > 5 && (
        <span className="text-xs text-muted-foreground">
          +{serverStatus.instances.length - 5}
        </span>
      )}
    </div>
  )
}

interface ServerRuntimeDetailsProps {
  status: ServerRuntimeStatus
  className?: string
}

export function ServerRuntimeDetails({
  status,
  className,
}: ServerRuntimeDetailsProps) {
  const { instances, stats } = status

  if (instances.length === 0) {
    return (
      <div className={cn('text-xs text-muted-foreground', className)}>
        No active instances
      </div>
    )
  }

  return (
    <div className={cn('space-y-2', className)}>
      <div className="flex items-center gap-3 text-xs">
        <span className="text-muted-foreground">Instances:</span>
        <div className="flex items-center gap-2">
          {stats.ready > 0 && (
            <span className="flex items-center gap-1">
              <StateDot state="ready" />
              <span>{stats.ready}</span>
            </span>
          )}
          {stats.busy > 0 && (
            <span className="flex items-center gap-1">
              <StateDot state="busy" />
              <span>{stats.busy}</span>
            </span>
          )}
          {stats.starting > 0 && (
            <span className="flex items-center gap-1">
              <StateDot state="starting" />
              <span>{stats.starting}</span>
            </span>
          )}
          {stats.draining > 0 && (
            <span className="flex items-center gap-1">
              <StateDot state="draining" />
              <span>{stats.draining}</span>
            </span>
          )}
          {stats.failed > 0 && (
            <span className="flex items-center gap-1">
              <StateDot state="failed" />
              <span>{stats.failed}</span>
            </span>
          )}
        </div>
      </div>
    </div>
  )
}

export function RuntimeStatusLegend({ className }: { className?: string }) {
  return (
    <div className={cn('flex items-center gap-3 text-xs', className)}>
      <span className="flex items-center gap-1">
        <StateDot state="ready" />
        <span className="text-muted-foreground">Ready</span>
      </span>
      <span className="flex items-center gap-1">
        <StateDot state="busy" />
        <span className="text-muted-foreground">Busy</span>
      </span>
      <span className="flex items-center gap-1">
        <StateDot state="starting" />
        <span className="text-muted-foreground">Starting</span>
      </span>
      <span className="flex items-center gap-1">
        <StateDot state="draining" />
        <span className="text-muted-foreground">Draining</span>
      </span>
      <span className="flex items-center gap-1">
        <StateDot state="failed" />
        <span className="text-muted-foreground">Failed</span>
      </span>
    </div>
  )
}
