// Input: Card, Badge, ToggleGroup, tooltip components, core status view hook, dashboard data hooks
// Output: StatusCards component displaying core status metrics with view selector
// Position: Dashboard status overview section

import {
  ClockIcon,
  FileTextIcon,
  LayersIcon,
  Loader2Icon,
  ServerIcon,
  WrenchIcon,
} from 'lucide-react'
import { m } from 'motion/react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { CoreStatusView } from '@/hooks/use-core-status-view'
import { useCoreStatusViewState } from '@/hooks/use-core-status-view'
import { Spring } from '@/lib/spring'
import { formatDuration } from '@/lib/time'
import { useServers } from '@/modules/servers/hooks'
import { coreStatusConfig, type CoreStatus } from '@/modules/shared/core-status'

import { useResources, useTools } from '../hooks'
import { AnimatedNumber } from './sparkline'

interface StatCardProps {
  title: string
  value: number | string
  icon: React.ReactNode
  description?: string
  delay?: number
  loading?: boolean
  animate?: boolean
}

function StatCard({
  title,
  value,
  icon,
  description,
  delay = 0,
  loading,
  animate = true,
}: StatCardProps) {
  return (
    <m.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.3, delay)}
    >
      <Card className="relative overflow-hidden">
        <CardHeader className="flex flex-row items-center justify-between pb-1">
          <CardTitle className="text-muted-foreground text-xs font-medium">
            {title}
          </CardTitle>
          <Tooltip>
            <TooltipTrigger>
              <span className="text-muted-foreground">{icon}</span>
            </TooltipTrigger>
            {description && (
              <TooltipContent>{description}</TooltipContent>
            )}
          </Tooltip>
        </CardHeader>
        <CardContent>
          {loading ? (
            <Skeleton className="h-6 w-12" />
          ) : (
            <div className="text-xl font-semibold tabular-nums">
              {typeof value === 'number' && animate ? (
                <AnimatedNumber value={value} />
              ) : (
                value
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </m.div>
  )
}

function CoreStatusCard({
  coreStatus,
  isLoading,
}: {
  coreStatus: CoreStatus
  isLoading: boolean
}) {
  const config = coreStatusConfig[coreStatus]

  return (
    <m.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.3)}
    >
      <Card className="relative overflow-hidden">
        <CardHeader className="flex flex-row items-center justify-between pb-1">
          <CardTitle className="text-muted-foreground text-xs font-medium">
            Core Status
          </CardTitle>
          <ServerIcon className="size-3.5 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-6 w-16" />
          ) : (
            <div className="flex items-center gap-2 mt-2.5">
              <Badge variant={config.variant} size="default" className="gap-1.5">
                {(coreStatus === 'starting' || coreStatus === 'stopping') && (
                  <Loader2Icon className="size-3 animate-spin" />
                )}
                {config.label}
              </Badge>
            </div>
          )}
        </CardContent>
      </Card>
    </m.div>
  )
}

function UptimeCard({
  uptime,
  isLoading,
}: {
  uptime?: number
  isLoading: boolean
}) {
  const uptimeFormatted = uptime ? formatDuration(uptime) : '--'

  return (
    <m.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.3, 0.03)}
    >
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-1">
          <CardTitle className="text-muted-foreground text-xs font-medium">
            Uptime
          </CardTitle>
          <ClockIcon className="size-3.5 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-6 w-16" />
          ) : (
            <div className="flex items-baseline gap-2">
              <span className="text-xl font-semibold tabular-nums">
                {uptimeFormatted}
              </span>
            </div>
          )}
        </CardContent>
      </Card>
    </m.div>
  )
}

export function StatusCards() {
  const { data: servers, isLoading: serversLoading } = useServers()
  const { tools, isLoading: toolsLoading } = useTools()
  const { resources, isLoading: resourcesLoading } = useResources()
  const {
    coreStatus,
    data: coreState,
    isLoading: coreLoading,
    view,
    setView,
    isRemoteAvailable,
  } = useCoreStatusViewState()

  const handleViewChange = (next: string | null) => {
    if (!next) return
    void setView(next as CoreStatusView)
  }

  return (
    <div className="space-y-3">
      {isRemoteAvailable && (
        <div className="flex items-center justify-between gap-3">
          <span className="text-xs font-medium text-muted-foreground">
            Core status view
          </span>
          <ToggleGroup
            type="single"
            value={view}
            onValueChange={handleViewChange}
            aria-label="Core status view"
          >
            <ToggleGroupItem value="local" size="sm" variant="outline">
              Local
            </ToggleGroupItem>
            <ToggleGroupItem value="remote" size="sm" variant="outline">
              Remote
            </ToggleGroupItem>
          </ToggleGroup>
        </div>
      )}
      <div className="grid gap-3 md:grid-cols-3 lg:grid-cols-5">
        <CoreStatusCard coreStatus={coreStatus} isLoading={coreLoading} />
        <UptimeCard uptime={coreState?.uptime} isLoading={coreLoading} />

        <StatCard
          title="Servers"
          value={servers?.length ?? 0}
          icon={<LayersIcon className="size-3.5" />}
          description="Configured MCP servers"
          delay={0.06}
          loading={serversLoading}
        />

        <StatCard
          title="Tools"
          value={tools.length}
          icon={<WrenchIcon className="size-3.5" />}
          description="Available MCP tools"
          delay={0.12}
          loading={toolsLoading}
        />

        <StatCard
          title="Resources"
          value={resources.length}
          icon={<FileTextIcon className="size-3.5" />}
          description="Available resources"
          delay={0.15}
          loading={resourcesLoading}
        />
      </div>
    </div>
  )
}
