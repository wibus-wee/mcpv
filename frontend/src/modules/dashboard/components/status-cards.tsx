// Input: Card, Badge, Progress, Tooltip components, dashboard data hooks, lucide icons
// Output: StatusCards component displaying core status metrics with animations
// Position: Dashboard status overview section

import {
  CheckCircle2Icon,
  ClockIcon,
  FileTextIcon,
  LayersIcon,
  Loader2Icon,
  MonitorIcon,
  ServerIcon,
  WrenchIcon,
  XCircleIcon,
} from 'lucide-react'
import { m } from 'motion/react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useActiveClients } from '@/hooks/use-active-clients'
import { useCoreState } from '@/hooks/use-core-state'
import { Spring } from '@/lib/spring'
import { formatDuration } from '@/lib/time'

import { useServers } from '@/modules/config/hooks'
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

function CoreStatusCard() {
  const { coreStatus, data: isLoading } = useCoreState()

  const statusConfig = {
    running: {
      variant: 'success' as const,
      icon: CheckCircle2Icon,
      label: 'Running',
      dotColor: 'bg-emerald-500',
    },
    starting: {
      variant: 'warning' as const,
      icon: Loader2Icon,
      label: 'Starting',
      dotColor: 'bg-amber-500',
    },
    stopped: {
      variant: 'secondary' as const,
      icon: XCircleIcon,
      label: 'Stopped',
      dotColor: 'bg-slate-400',
    },
    stopping: {
      variant: 'warning' as const,
      icon: Loader2Icon,
      label: 'Stopping',
      dotColor: 'bg-amber-500',
    },
    error: {
      variant: 'error' as const,
      icon: XCircleIcon,
      label: 'Error',
      dotColor: 'bg-red-500',
    },
  }

  const config = statusConfig[coreStatus]

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

function UptimeCard() {
  const { data: coreState, isLoading } = useCoreState()

  const uptimeFormatted = coreState?.uptime ? formatDuration(coreState.uptime) : '--'

  const getUptimeHint = () => {
    if (!coreState?.uptime) return null
    const hours = coreState.uptime / 3600
    if (hours >= 24) return 'Stable'
    if (hours >= 1) return 'Running well'
    return null
  }

  const hint = getUptimeHint()

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
              {hint && (
                <span className="text-xs text-emerald-500">{hint}</span>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </m.div>
  )
}

export function StatusCards() {
  const { data: servers, isLoading: serversLoading } = useServers()
  const { data: clients, isLoading: clientsLoading } = useActiveClients()
  const { tools, isLoading: toolsLoading } = useTools()
  const { resources, isLoading: resourcesLoading } = useResources()

  return (
    <div className="grid gap-3 md:grid-cols-3 lg:grid-cols-6">
      <CoreStatusCard />
      <UptimeCard />

      <StatCard
        title="Servers"
        value={servers?.length ?? 0}
        icon={<LayersIcon className="size-3.5" />}
        description="Configured MCP servers"
        delay={0.06}
        loading={serversLoading}
      />

      <StatCard
        title="Active Clients"
        value={clients?.length ?? 0}
        icon={<MonitorIcon className="size-3.5" />}
        description="Clients currently connected"
        delay={0.09}
        loading={clientsLoading}
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
  )
}
