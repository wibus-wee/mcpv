// Input: ServerRuntimeStatus from bindings, runtime status hooks
// Output: Runtime status summary and instance detail components
// Position: Runtime status display component for server instances

import { RuntimeService } from '@bindings/mcpv/internal/ui/services'
import type { ServerInitStatus, ServerRuntimeStatus } from '@bindings/mcpv/internal/ui/types'
import { useState } from 'react'

import { ServerStateBadge, ServerStateCountBadge } from '@/components/custom/status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { formatInstanceId, formatInstanceTimeline, getOldestUptimeMs } from '@/lib/server-stats'
import { formatDuration, formatLatency, getElapsedMs, getRemainingSeconds } from '@/lib/time'
import { cn } from '@/lib/utils'

import { useRuntimeStatus, useServerInitStatus } from '../hooks'

interface ServerRuntimeIndicatorProps {
  specKey: string
  className?: string
}

export function ServerRuntimeIndicator({
  specKey,
  className,
}: ServerRuntimeIndicatorProps) {
  const { data: runtimeStatus } = useRuntimeStatus()
  const { data: initStatus } = useServerInitStatus()
  const serverStatus = (runtimeStatus as ServerRuntimeStatus[] | undefined)?.find(
    status => status.specKey === specKey,
  )
  const init = (initStatus as ServerInitStatus[] | undefined)?.find(
    status => status.specKey === specKey,
  )

  if (!init && !serverStatus) {
    return null
  }

  const summary = serverStatus ? buildInstanceSummary(serverStatus, init, 'compact') : null

  return (
    <div className={cn('flex items-center gap-2', className)}>
      {init && <InitBadge status={init} />}
      {summary && (
        <Badge
          variant={summary.variant}
          size="sm"
          className="font-mono tabular-nums"
          title={summary.title}
        >
          {summary.label}
        </Badge>
      )}
    </div>
  )
}

interface ServerRuntimeSummaryProps {
  specKey: string
  className?: string
}

export function ServerRuntimeSummary({ specKey, className }: ServerRuntimeSummaryProps) {
  const { data: runtimeStatus } = useRuntimeStatus()
  const { data: initStatus } = useServerInitStatus()
  const serverStatus = (runtimeStatus as ServerRuntimeStatus[] | undefined)?.find(
    status => status.specKey === specKey,
  )
  const init = (initStatus as ServerInitStatus[] | undefined)?.find(
    status => status.specKey === specKey,
  )

  if (!serverStatus && !init) {
    return null
  }

  if (!serverStatus && init) {
    return <InitOnlySummary status={init} className={className} />
  }

  if (!serverStatus) {
    return null
  }

  return (
    <ServerRuntimeDetails
      status={serverStatus}
      initStatus={init}
      className={className}
    />
  )
}

interface ServerRuntimeDetailsProps {
  status: ServerRuntimeStatus
  className?: string
  initStatus?: ServerInitStatus
}

export function ServerRuntimeDetails({
  status,
  className,
  initStatus,
}: ServerRuntimeDetailsProps) {
  const { instances, stats } = status
  const { metrics } = status
  const summary = buildInstanceSummary(status, initStatus, 'full')
  const uptimeMs = getOldestUptimeMs(instances)
  const restartCount = Math.max(0, metrics.startCount - 1)
  const avgResponseMs = metrics.totalCalls > 0
    ? metrics.totalDurationMs / metrics.totalCalls
    : null
  const lastCallAgeMs = getElapsedMs(metrics.lastCallAt)
  const showInitDetails = initStatus ? shouldShowInitDetails(initStatus) : false
  const showInitBadge = Boolean(initStatus) && !showInitDetails
  const stateCount
    = stats.ready
      + stats.busy
      + stats.starting
      + stats.initializing
      + stats.handshaking
      + stats.draining
      + stats.failed

  return (
    <div className={cn('space-y-3', className)}>
      <div className="flex flex-wrap items-center gap-2">
        {showInitBadge && initStatus && <InitBadge status={initStatus} />}
        {summary && (
          <Badge
            variant={summary.variant}
            size="sm"
            className="font-mono tabular-nums"
            title={summary.title}
          >
            {summary.label}
          </Badge>
        )}
      </div>

      {showInitDetails && initStatus && (
        <Card className="p-3">
          <InitStatusLine status={initStatus} />
        </Card>
      )}

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <MetricTile
          label="Uptime"
          value={uptimeMs === null ? '--' : formatDuration(uptimeMs)}
        />
        <MetricTile label="Restarts" value={`${restartCount}`} />
        <MetricTile
          label="Avg latency"
          value={avgResponseMs === null ? '--' : formatLatency(avgResponseMs)}
        />
        <MetricTile
          label="Last call"
          value={lastCallAgeMs === null ? '--' : `${formatDuration(lastCallAgeMs)} ago`}
        />
      </div>

      <div className="space-y-1">
        <span className="text-xs text-muted-foreground">Instance states</span>
        <div className="flex flex-wrap items-center gap-2">
          {stateCount > 0 ? (
            <>
              <ServerStateCountBadge state="ready" count={stats.ready} />
              <ServerStateCountBadge state="busy" count={stats.busy} />
              <ServerStateCountBadge state="starting" count={stats.starting} />
              <ServerStateCountBadge state="initializing" count={stats.initializing} />
              <ServerStateCountBadge state="handshaking" count={stats.handshaking} />
              <ServerStateCountBadge state="draining" count={stats.draining} />
              <ServerStateCountBadge state="failed" count={stats.failed} />
            </>
          ) : (
            <Badge variant="secondary" size="sm">
              No instances
            </Badge>
          )}
        </div>
      </div>

      {instances.length === 0 ? (
        <Card className="p-4">
          <p className="text-xs text-muted-foreground text-center">
            No active instances.
          </p>
        </Card>
      ) : (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">Instances</span>
            <Badge variant="secondary" size="sm" className="font-mono tabular-nums">
              {instances.length}
            </Badge>
          </div>
          <Card className="p-3">
            <div className="max-h-48 overflow-auto space-y-2 text-xs text-muted-foreground">
              {instances.map(inst => (
                <div key={inst.id} className="flex flex-wrap items-center gap-2">
                  <ServerStateBadge state={inst.state} size="sm" />
                  <span
                    className="font-mono text-foreground/80"
                    title={inst.id}
                  >
                    {formatInstanceId(inst.id)}
                  </span>
                  {formatInstanceTimeline(inst) && (
                    <span className="text-muted-foreground">
                      {formatInstanceTimeline(inst)}
                    </span>
                  )}
                </div>
              ))}
            </div>
          </Card>
        </div>
      )}
    </div>
  )
}

function InitOnlySummary({
  status,
  className,
}: {
  status: ServerInitStatus
  className?: string
}) {
  const showDetails = shouldShowInitDetails(status)
  const counts = showDetails ? '' : formatInitCounts(status)

  return (
    <div className={cn('space-y-2', className)}>
      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <InitBadge status={status} />
        {counts && <span>{counts}</span>}
      </div>
      {showDetails && (
        <Card className="p-3">
          <InitStatusLine status={status} showBadge={false} />
        </Card>
      )}
    </div>
  )
}

function MetricTile({ label, value }: { label: string, value: string }) {
  return (
    <div className="rounded-md border border-border/60 bg-card/40 px-3 py-2">
      <div className="text-[11px] uppercase tracking-wide text-muted-foreground">
        {label}
      </div>
      <div className="text-sm font-medium">{value}</div>
    </div>
  )
}

export function RuntimeStatusLegend({ className }: { className?: string }) {
  return (
    <div className={cn('flex flex-wrap items-center gap-2 text-xs', className)}>
      <ServerStateBadge state="ready" size="sm" />
      <ServerStateBadge state="busy" size="sm" />
      <ServerStateBadge state="starting" size="sm" />
      <ServerStateBadge state="initializing" size="sm" />
      <ServerStateBadge state="handshaking" size="sm" />
      <ServerStateBadge state="draining" size="sm" />
      <ServerStateBadge state="failed" size="sm" />
    </div>
  )
}

type SummaryMode = 'compact' | 'full'

type InstanceSummary = {
  label: string
  title?: string
  variant: 'secondary' | 'success' | 'warning' | 'error'
}

function buildInstanceSummary(
  status: ServerRuntimeStatus,
  initStatus?: ServerInitStatus,
  mode: SummaryMode = 'full',
): InstanceSummary | null {
  const { stats } = status
  const { ready } = stats
  const { total } = stats
  const { failed } = stats
  const { busy } = stats
  const starting = stats.starting + stats.initializing + stats.handshaking
  const { draining } = stats
  const target = initStatus?.minReady ?? 0

  let label = ''
  let title = ''
  if (mode === 'full') {
    if (target === 0 && total === 0) {
      label = 'No instances'
      title = 'No active instances'
    }
    else if (target > 0) {
      label = `Ready ${ready} of ${target} target`
      title = `Ready ${ready} of ${target} target · ${total} total`
    }
    else {
      label = `Ready ${ready} of ${total}`
      title = `Ready ${ready} of ${total} total`
    }
  }
  else {
    if (total === 0) {
      label = 'No instances'
    }
    else if (failed > 0) {
      label = 'Failed'
    }
    else if (draining > 0 && ready === 0) {
      label = 'Draining'
    }
    else if (target > 0 && ready < target) {
      label = 'Degraded'
    }
    else if (busy > 0) {
      label = 'Busy'
    }
    else if (starting > 0 && ready === 0) {
      label = 'Starting'
    }
    else {
      label = 'Ready'
    }
    if (target > 0) {
      title = `Ready ${ready} of ${target} target · ${total} total`
    }
    else if (total > 0) {
      title = `Ready ${ready} of ${total} total`
    }
    else {
      title = 'No active instances'
    }
  }

  let variant: InstanceSummary['variant'] = 'secondary'
  if (failed > 0) {
    variant = 'error'
  }
  else if (target > 0 && ready < target) {
    variant = 'warning'
  }
  else if (ready > 0) {
    variant = 'success'
  }

  return { label, title, variant }
}

function InitStatusLine({
  status,
  showBadge = true,
  className,
}: {
  status: ServerInitStatus
  showBadge?: boolean
  className?: string
}) {
  const [isRetrying, setIsRetrying] = useState(false)
  const [retryError, setRetryError] = useState<string | null>(null)
  const retryInfo = formatRetryInfo(status)
  const initCounts = formatInitCounts(status)

  const handleRetry = async () => {
    if (isRetrying) {
      return
    }
    setIsRetrying(true)
    setRetryError(null)
    try {
      await RuntimeService.RetryServerInit({ specKey: status.specKey })
    }
    catch (err) {
      setRetryError(err instanceof Error ? err.message : 'Retry failed')
    }
    finally {
      setIsRetrying(false)
    }
  }

  return (
    <div className={cn('flex flex-wrap items-center gap-2 text-xs', className)}>
      {showBadge && <InitBadge status={status} />}
      {initCounts && (
        <span className="text-muted-foreground">{initCounts}</span>
      )}
      {retryInfo && (
        <span className="text-muted-foreground">{retryInfo}</span>
      )}
      {status.state === 'suspended' && (
        <Button
          variant="outline"
          size="xs"
          onClick={handleRetry}
          disabled={isRetrying}
        >
          {isRetrying ? 'Retrying...' : 'Retry'}
        </Button>
      )}
      {status.lastError && (
        <span
          className="text-destructive cursor-help"
          title={status.lastError}
        >
          {status.lastError}
        </span>
      )}
      {retryError && (
        <span className="text-destructive">{retryError}</span>
      )}
    </div>
  )
}

function InitBadge({ status }: { status: ServerInitStatus }) {
  const variant = initVariant[status.state] || 'secondary'
  return (
    <Badge
      variant={variant}
      size="sm"
      className="font-medium"
      title={status.lastError || undefined}
    >
      {formatInitLabel(status)}
    </Badge>
  )
}

const initVariant: Record<string, 'secondary' | 'info' | 'success' | 'warning' | 'destructive'> = {
  pending: 'secondary',
  starting: 'info',
  ready: 'success',
  degraded: 'warning',
  failed: 'destructive',
  suspended: 'warning',
}

function formatInitLabel(status: ServerInitStatus) {
  switch (status.state) {
    case 'ready':
      return 'Init ready'
    case 'degraded':
      return 'Init degraded'
    case 'failed':
      return 'Init failed'
    case 'starting':
      return 'Init starting'
    case 'suspended':
      return 'Init suspended'
    default:
      return 'Init pending'
  }
}

function formatInitCounts(status: ServerInitStatus) {
  const parts: string[] = []
  if (status.minReady > 0) {
    parts.push(`Ready ${status.ready} of ${status.minReady} target`)
  }
  else {
    parts.push(`Ready ${status.ready}`)
  }
  if (status.failed > 0) {
    parts.push(`Failed ${status.failed}`)
  }
  return parts.join(' · ')
}

function shouldShowInitDetails(status: ServerInitStatus) {
  if (status.state !== 'ready') {
    return true
  }
  if (status.lastError) {
    return true
  }
  return status.retryCount > 0
}

function formatRetryInfo(status: ServerInitStatus) {
  const parts: string[] = []
  if (status.retryCount > 0) {
    parts.push(`Retries: ${status.retryCount}`)
  }
  if (status.nextRetryAt) {
    const remainingSeconds = getRemainingSeconds(status.nextRetryAt)
    parts.push(`Next in ${remainingSeconds}s`)
  }
  if (parts.length === 0) {
    return ''
  }
  return parts.join(' | ')
}
