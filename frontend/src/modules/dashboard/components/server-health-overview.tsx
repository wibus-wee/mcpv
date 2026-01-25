// Input: RuntimeService bindings, SWR, visualization components
// Output: ServerHealthOverview component displaying pool health and metrics
// Position: Primary dashboard visualization for server pool status

import { Link } from '@tanstack/react-router'
import {
  ActivityIcon,
  AlertTriangleIcon,
  CheckCircle2Icon,
  ExternalLinkIcon,
  ServerIcon,
  ZapIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useMemo } from 'react'
import useSWR from 'swr'

import { RuntimeService, ServerInitStatus, ServerRuntimeStatus } from '@bindings/mcpd/internal/ui'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Spring } from '@/lib/spring'
import { swrPresets } from '@/lib/swr-config'

import { AnimatedNumber, StackedBar } from './sparkline'

function useRuntimeStatus() {
  return useSWR<ServerRuntimeStatus[]>(
    'runtime-status',
    () => RuntimeService.GetRuntimeStatus(),
    swrPresets.fastRealtime,
  )
}

function useServerInitStatus() {
  return useSWR<ServerInitStatus[]>(
    'server-init-status',
    () => RuntimeService.GetServerInitStatus(),
    swrPresets.fastRealtime,
  )
}

interface AggregatedStats {
  totalServers: number
  totalInstances: number
  readyInstances: number
  busyInstances: number
  startingInstances: number
  failedInstances: number
  drainingInstances: number
  suspendedServers: number
  totalCalls: number
  totalErrors: number
  avgDurationMs: number
  errorRate: number
  utilization: number
}

function aggregateStats(
  statuses: ServerRuntimeStatus[],
  initStatuses?: ServerInitStatus[],
): AggregatedStats {
  const result: AggregatedStats = {
    totalServers: statuses.length,
    totalInstances: 0,
    readyInstances: 0,
    busyInstances: 0,
    startingInstances: 0,
    failedInstances: 0,
    drainingInstances: 0,
    suspendedServers: 0,
    totalCalls: 0,
    totalErrors: 0,
    avgDurationMs: 0,
    errorRate: 0,
    utilization: 0,
  }

  if (initStatuses) {
    result.suspendedServers = initStatuses.filter(s => s.state === 'suspended').length
  }

  for (const status of statuses) {
    const stats = status.stats
    result.totalInstances += stats.total
    result.readyInstances += stats.ready
    result.busyInstances += stats.busy
    result.startingInstances += stats.starting + stats.initializing + stats.handshaking
    result.failedInstances += stats.failed
    result.drainingInstances += stats.draining

    const metrics = status.metrics
    result.totalCalls += metrics.totalCalls
    result.totalErrors += metrics.totalErrors
    result.avgDurationMs += metrics.totalDurationMs
  }

  if (result.totalCalls > 0) {
    result.avgDurationMs = result.avgDurationMs / result.totalCalls
    result.errorRate = (result.totalErrors / result.totalCalls) * 100
  }

  if (result.totalInstances > 0) {
    result.utilization = (result.busyInstances / result.totalInstances) * 100
  }

  return result
}

function HealthVerdict({ stats }: { stats: AggregatedStats }) {
  const { utilization, errorRate, failedInstances, suspendedServers, totalServers } = stats

  const getVerdict = () => {
    if (suspendedServers > 0) {
      return {
        status: 'warning' as const,
        icon: AlertTriangleIcon,
        message: `${suspendedServers} server${suspendedServers > 1 ? 's' : ''} suspended`,
        color: 'text-amber-500',
      }
    }
    if (failedInstances > 0) {
      return {
        status: 'warning' as const,
        icon: AlertTriangleIcon,
        message: `${failedInstances} failed instance${failedInstances > 1 ? 's' : ''} detected`,
        color: 'text-amber-500',
      }
    }
    if (errorRate > 5) {
      return {
        status: 'warning' as const,
        icon: AlertTriangleIcon,
        message: `High error rate: ${errorRate.toFixed(1)}%`,
        color: 'text-amber-500',
      }
    }
    if (utilization > 85) {
      return {
        status: 'warning' as const,
        icon: ZapIcon,
        message: 'High utilization — consider scaling',
        color: 'text-amber-500',
      }
    }
    return {
      status: 'healthy' as const,
      icon: CheckCircle2Icon,
      message: `All ${totalServers} server${totalServers !== 1 ? 's' : ''} healthy`,
      color: 'text-emerald-500',
    }
  }

  const verdict = getVerdict()
  const Icon = verdict.icon
  const showLink = verdict.status === 'warning'

  return (
    <m.div
      initial={{ opacity: 0, y: 5 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.3)}
      className="flex items-center justify-between gap-2 rounded-lg bg-muted/30 px-3 py-2"
    >
      <div className="flex items-center gap-2">
        <Icon className={`size-4 ${verdict.color}`} />
        <span className="text-sm text-muted-foreground">{verdict.message}</span>
      </div>
      {showLink && (
        <Link
          to="/tools"
          search={{ server: undefined }}
          className="flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          View details
          <ExternalLinkIcon className="size-3" />
        </Link>
      )}
    </m.div>
  )
}

function MetricTile({
  label,
  value,
  icon: Icon,
  suffix,
  delay = 0,
}: {
  label: string
  value: number
  icon: React.ComponentType<{ className?: string }>
  suffix?: string
  delay?: number
}) {
  return (
    <m.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.3, delay)}
      className="flex flex-col gap-1"
    >
      <div className="flex items-center gap-1.5 text-muted-foreground">
        <Icon className="size-3" />
        <span className="text-xs">{label}</span>
      </div>
      <div className="flex items-baseline gap-1">
        <span className="text-lg font-semibold tabular-nums">
          <AnimatedNumber value={value} />
        </span>
        {suffix && <span className="text-xs text-muted-foreground">{suffix}</span>}
      </div>
    </m.div>
  )
}

export function ServerHealthOverview() {
  const { data: statuses, isLoading } = useRuntimeStatus()
  const { data: initStatuses } = useServerInitStatus()

  const stats = useMemo(() => {
    if (!statuses) return null
    return aggregateStats(statuses, initStatuses)
  }, [statuses, initStatuses])

  const poolSegments = useMemo(() => {
    if (!stats) return []
    return [
      { value: stats.readyInstances, color: 'bg-emerald-500', label: 'Ready' },
      { value: stats.busyInstances, color: 'bg-blue-500', label: 'Busy' },
      { value: stats.startingInstances, color: 'bg-amber-500', label: 'Starting' },
      { value: stats.drainingInstances, color: 'bg-slate-400', label: 'Draining' },
      { value: stats.failedInstances, color: 'bg-red-500', label: 'Failed' },
    ]
  }, [stats])

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            Server Health
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-24 w-full" />
        </CardContent>
      </Card>
    )
  }

  if (!stats || stats.totalServers === 0) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            Server Health
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col items-center justify-center py-6 text-center">
            <ServerIcon className="mb-2 size-8 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">No servers running</p>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader className='pb-3'>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            Server Health
            <Badge variant="secondary" size="sm">{stats.totalServers}</Badge>
          </CardTitle>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <HealthVerdict stats={stats} />

        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">Pool Distribution</span>
            <span className="text-muted-foreground">{stats.totalInstances} instances</span>
          </div>
          <StackedBar segments={poolSegments} height={10} />
          <div className="flex flex-wrap gap-3 text-xs">
            {poolSegments.filter(s => s.value > 0).map((segment) => (
              <Tooltip key={segment.label}>
                <TooltipTrigger>
                  <div className="flex items-center gap-1.5">
                    <span className={`size-2 rounded-full ${segment.color}`} />
                    <span className="text-muted-foreground">{segment.label}</span>
                    <span className="font-medium">{segment.value}</span>
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  {segment.label}: {segment.value} instance{segment.value !== 1 ? 's' : ''}
                </TooltipContent>
              </Tooltip>
            ))}
          </div>
        </div>

        <div className="grid grid-cols-3 gap-4 border-t pt-4">
          <MetricTile
            label="Total Calls"
            value={stats.totalCalls}
            icon={ActivityIcon}
            delay={0}
          />
          <MetricTile
            label="Errors"
            value={stats.totalErrors}
            icon={AlertTriangleIcon}
            delay={0.05}
          />
          <MetricTile
            label="Avg Duration"
            value={Math.round(stats.avgDurationMs)}
            icon={ZapIcon}
            suffix="ms"
            delay={0.1}
          />
        </div>
      </CardContent>
    </Card>
  )
}
