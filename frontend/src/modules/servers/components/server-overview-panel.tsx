// Input: serverName, config hooks, runtime status hooks, active clients
// Output: Server overview panel showing health, stats
// Position: Overview panel component for server module

import { useAtom } from 'jotai'
import {
  ActivityIcon,
  ClockIcon,
  ServerIcon,
  WrenchIcon,
  ZapIcon,
} from 'lucide-react'
import { m } from 'motion/react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { ScrollArea } from '@/components/ui/scroll-area'
import { getMetricsSummary, getPoolStats } from '@/lib/server-stats'
import { Spring } from '@/lib/spring'
import {
  formatDuration,
  formatLatency,
} from '@/lib/time'
import { cn } from '@/lib/utils'
import { ServerInstancesTable } from '@/modules/servers/components/server-instances-table'
import { ServerRuntimeSummary } from '@/modules/servers/components/server-runtime-status'
import { useRuntimeStatus } from '@/modules/servers/hooks'

import { selectedServerAtom } from '../atoms'

interface ServerOverviewPanelProps {
  className?: string
}

function ServerStatCard({
  icon: Icon,
  label,
  value,
  subValue,
  variant = 'default',
}: {
  icon: React.ElementType
  label: string
  value: React.ReactNode
  subValue?: string
  variant?: 'default' | 'success' | 'warning' | 'error'
}) {
  const variantClasses = {
    default: 'text-muted-foreground',
    success: 'text-success',
    warning: 'text-warning',
    error: 'text-destructive',
  }

  return (
    <Card className="p-3">
      <div className="flex items-start gap-3">
        <div className={cn('rounded-md bg-muted p-2', variantClasses[variant])}>
          <Icon className="size-4" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="text-sm font-semibold tabular-nums">{value}</p>
          {subValue && (
            <p className="text-xs text-muted-foreground">{subValue}</p>
          )}
        </div>
      </div>
    </Card>
  )
}

function EmptyState() {
  return (
    <Empty className="py-16">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <ServerIcon className="size-4" />
        </EmptyMedia>
        <EmptyTitle className="text-sm">Select a server</EmptyTitle>
        <EmptyDescription className="text-xs">
          Choose a server from the list to view its health and status.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function ServerOverviewPanel({
  className,
}: ServerOverviewPanelProps) {
  const [server] = useAtom(selectedServerAtom)
  const { data: runtimeStatus } = useRuntimeStatus()

  if (!server) {
    return <EmptyState />
  }

  const serverRuntimeStatus = runtimeStatus?.find(
    status => status.specKey === server.specKey,
  )
  const poolStats = serverRuntimeStatus ? getPoolStats(serverRuntimeStatus) : null
  const metricsSummary = serverRuntimeStatus
    ? getMetricsSummary(serverRuntimeStatus)
    : null
  const tags = server.tags ?? []

  const instanceStatuses = serverRuntimeStatus?.instances ?? []
  const sortedInstances = [...instanceStatuses].sort((a, b) =>
    a.id.localeCompare(b.id),
  )
  const specDetail = server

  return (
    <ScrollArea className={cn('h-full', className)}>
      <m.div
        key={server.name}
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={Spring.smooth(0.3)}
        className="space-y-6"
      >
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-lg font-semibold">{server.name}</h2>
              <Badge
                variant={server.disabled ? 'warning' : 'success'}
                size="sm"
              >
                {server.disabled ? 'Disabled' : 'Enabled'}
              </Badge>
              <Badge variant="outline" size="sm" className="uppercase">
                {server.transport}
              </Badge>
            </div>
            <p className="text-xs text-muted-foreground">
              Spec key <code className="font-mono">{server.specKey}</code>
            </p>
          </div>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <ActivityIcon className="size-4" />
              Runtime Status
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ServerRuntimeSummary specKey={server.specKey} />
          </CardContent>
        </Card>

        {poolStats && (
          <div className="space-y-2">
            <h3 className="flex items-center gap-2 text-sm font-semibold">
              <ZapIcon className="size-4" />
              Pool Stats
            </h3>
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
              <ServerStatCard
                icon={ServerIcon}
                label="Total Instances"
                value={poolStats.total}
              />
              <ServerStatCard
                icon={ActivityIcon}
                label="Ready"
                value={poolStats.ready}
                variant={poolStats.ready > 0 ? 'success' : 'default'}
              />
              <ServerStatCard
                icon={WrenchIcon}
                label="Busy"
                value={poolStats.busy}
                variant={poolStats.busy > 0 ? 'warning' : 'default'}
              />
              {poolStats.starting > 0 && (
                <ServerStatCard
                  icon={ClockIcon}
                  label="Starting"
                  value={poolStats.starting}
                />
              )}
              {poolStats.failed > 0 && (
                <ServerStatCard
                  icon={ZapIcon}
                  label="Failed"
                  value={poolStats.failed}
                  variant="error"
                />
              )}
            </div>
          </div>
        )}

        {metricsSummary && (
          <div className="space-y-2">
            <h3 className="flex items-center gap-2 text-sm font-semibold">
              <ClockIcon className="size-4" />
              Recent Metrics
            </h3>
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
              <ServerStatCard
                icon={WrenchIcon}
                label="Total Calls"
                value={metricsSummary.totalCalls}
              />
              <ServerStatCard
                icon={ZapIcon}
                label="Errors"
                value={metricsSummary.totalErrors}
                variant={metricsSummary.totalErrors > 0 ? 'error' : 'default'}
              />
              <ServerStatCard
                icon={ClockIcon}
                label="Avg Latency"
                value={
                  metricsSummary.avgResponseMs !== null
                    ? formatLatency(metricsSummary.avgResponseMs)
                    : '--'
                }
              />
              <ServerStatCard
                icon={ActivityIcon}
                label="Last Call"
                value={
                  metricsSummary.lastCallAgeMs !== null
                    ? `${formatDuration(metricsSummary.lastCallAgeMs)} ago`
                    : '--'
                }
              />
            </div>
          </div>
        )}

        {sortedInstances.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm">
                <ZapIcon className="size-4" />
                Why it&apos;s on
              </CardTitle>
            </CardHeader>
            <CardContent className="p-1 -mt-2">
              <ServerInstancesTable instances={sortedInstances} specDetail={specDetail} />
            </CardContent>
          </Card>
        )}

        <div className="space-y-2">
          <h3 className="text-sm font-semibold">Tags</h3>
          {tags.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {tags.map(tag => (
                <Badge key={tag} variant="secondary" size="sm">
                  {tag}
                </Badge>
              ))}
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">
              No tags configured for this server.
            </p>
          )}
        </div>
      </m.div>
    </ScrollArea>
  )
}
