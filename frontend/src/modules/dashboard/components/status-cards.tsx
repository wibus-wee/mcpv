// Input: Card, Badge, Progress, Tooltip components, jotai atoms, lucide icons
// Output: StatusCards component displaying core status metrics
// Position: Dashboard status overview section

import { useAtomValue } from 'jotai'
import {
  ActivityIcon,
  ClockIcon,
  FileTextIcon,
  ServerIcon,
  WrenchIcon,
} from 'lucide-react'
import { m } from 'motion/react'

import { coreStatusAtom } from '@/atoms/core'
import { promptsCountAtom, resourcesCountAtom, toolsCountAtom } from '@/atoms/dashboard'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Spring } from '@/lib/spring'

import { useCoreState } from '../hooks'

interface StatCardProps {
  title: string
  value: number | string
  icon: React.ReactNode
  description?: string
  delay?: number
  loading?: boolean
}

function StatCard({ title, value, icon, description, delay = 0, loading }: StatCardProps) {
  return (
    <m.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.3, delay)}
    >
      <Card>
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
            <Skeleton className="h-5 w-12" />
          ) : (
            <div className="text-lg font-semibold">{value}</div>
          )}
        </CardContent>
      </Card>
    </m.div>
  )
}

export function StatusCards() {
  const coreStatus = useAtomValue(coreStatusAtom)
  const toolsCount = useAtomValue(toolsCountAtom)
  const resourcesCount = useAtomValue(resourcesCountAtom)
  const promptsCount = useAtomValue(promptsCountAtom)

  const { data: coreState, isLoading } = useCoreState()

  const statusBadgeVariant = {
    running: 'success' as const,
    starting: 'warning' as const,
    stopped: 'secondary' as const,
    error: 'error' as const,
  }

  const formatUptime = (ms: number) => {
    const totalSeconds = Math.floor(ms / 1000)
    if (totalSeconds <= 0) return '0s'

    const seconds = totalSeconds % 60
    const totalMinutes = Math.floor(totalSeconds / 60)
    const minutes = totalMinutes % 60
    const hours = Math.floor(totalMinutes / 60)
    const days = Math.floor(hours / 24)
    const remHours = hours % 24

    if (totalSeconds < 60) return `${totalSeconds}s`
    if (totalSeconds < 3600) return `${totalMinutes}m ${seconds}s`
    if (totalSeconds < 86400) return `${hours}h ${minutes}m`
    return `${days}d ${remHours}h`
  }

  return (
    <div className="grid gap-2 md:grid-cols-3 lg:grid-cols-5">
      <m.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={Spring.smooth(0.3)}
      >
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-1">
            <CardTitle className="text-muted-foreground text-xs font-medium">
              Core Status
            </CardTitle>
            <ServerIcon className="size-3.5 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-1.5">
              {isLoading ? (
                <Skeleton className="h-5 w-16" />
              ) : (
                <>
                  <Badge variant={statusBadgeVariant[coreStatus]} size="default" className='mt-2.5'>
                    {coreStatus.charAt(0).toUpperCase() + coreStatus.slice(1)}
                  </Badge>
                  {coreStatus === 'starting' && (
                    <Progress value={null} className="h-1 w-12" />
                  )}
                </>
              )}
            </div>
          </CardContent>
        </Card>
      </m.div>

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
              <Skeleton className="h-5 w-12" />
            ) : (
              <div className="text-lg font-semibold">
                {coreState?.uptime ? formatUptime(coreState.uptime) : '--'}
              </div>
            )}
          </CardContent>
        </Card>
      </m.div>

      <StatCard
        title="Tools"
        value={toolsCount}
        icon={<WrenchIcon className="size-3.5" />}
        description="Available MCP tools"
        delay={0.06}
      />

      <StatCard
        title="Resources"
        value={resourcesCount}
        icon={<FileTextIcon className="size-3.5" />}
        description="Available resources"
        delay={0.09}
      />

      <StatCard
        title="Prompts"
        value={promptsCount}
        icon={<ActivityIcon className="size-3.5" />}
        description="Available prompt templates"
        delay={0.12}
      />
    </div>
  )
}
