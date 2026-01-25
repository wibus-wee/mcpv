// Input: Dashboard hooks, visualization components
// Output: ActivityInsights component with tool usage analytics
// Position: Dashboard analytics section showing tool activity and trends

import {
  BarChart3Icon,
  TrendingUpIcon,
  WrenchIcon,
  ZapIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useMemo } from 'react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Spring } from '@/lib/spring'

import { useTools } from '../hooks'
import { AnimatedNumber, Sparkline } from './sparkline'

interface ToolWithUsage {
  name: string
  description?: string
  callCount: number
  percentage: number
}

function TopToolRow({
  tool,
  index,
  maxCount,
}: {
  tool: ToolWithUsage
  index: number
  maxCount: number
}) {
  const barWidth = maxCount > 0 ? (tool.callCount / maxCount) * 100 : 0

  return (
    <m.div
      initial={{ opacity: 0, x: -10 }}
      animate={{ opacity: 1, x: 0 }}
      transition={Spring.smooth(0.3, index * 0.03)}
      className="group"
    >
      <div className="flex items-center justify-between gap-3 py-1.5">
        <Tooltip>
          <TooltipTrigger delay={200}>
            <span className="max-w-32 truncate text-sm font-mono">
              {tool.name}
            </span>
          </TooltipTrigger>
          <TooltipContent side="right" className="max-w-64">
            <p className="font-medium">{tool.name}</p>
            {tool.description && (
              <p className="text-xs text-muted-foreground">{tool.description}</p>
            )}
          </TooltipContent>
        </Tooltip>
        <div className="flex items-center gap-2">
          <div className="h-1.5 w-16 overflow-hidden rounded-full bg-muted/30">
            <m.div
              className="h-full rounded-full bg-primary/60"
              initial={{ width: 0 }}
              animate={{ width: `${barWidth}%` }}
              transition={{ duration: 0.5, delay: index * 0.05 }}
            />
          </div>
          <span className="w-8 text-right text-xs tabular-nums text-muted-foreground">
            {tool.callCount}
          </span>
        </div>
      </div>
    </m.div>
  )
}

function InsightCard({
  title,
  value,
  subtitle,
  icon: Icon,
  trend,
  sparkData,
  delay = 0,
}: {
  title: string
  value: number | string
  subtitle?: string
  icon: React.ComponentType<{ className?: string }>
  trend?: 'up' | 'down' | 'stable'
  sparkData?: number[]
  delay?: number
}) {
  const trendColors = {
    up: 'text-emerald-500',
    down: 'text-red-500',
    stable: 'text-muted-foreground',
  }

  return (
    <m.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.3, delay)}
      className="flex flex-col rounded-lg border bg-card p-3"
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-1.5 text-muted-foreground">
          <Icon className="size-3.5" />
          <span className="text-xs">{title}</span>
        </div>
        {trend && (
          <span className={trendColors[trend]}>
            {trend === 'up' && '↑'}
            {trend === 'down' && '↓'}
            {trend === 'stable' && '→'}
          </span>
        )}
      </div>
      <div className="mt-1 flex items-end justify-between">
        <div>
          <div className="text-xl font-semibold tabular-nums">
            {typeof value === 'number' ? <AnimatedNumber value={value} /> : value}
          </div>
          {subtitle && (
            <div className="text-xs text-muted-foreground">{subtitle}</div>
          )}
        </div>
        {sparkData && sparkData.length > 1 && (
          <Sparkline
            data={sparkData}
            width={60}
            height={20}
            strokeColor="var(--color-primary)"
            fillColor="var(--color-primary)"
            className="opacity-60"
          />
        )}
      </div>
    </m.div>
  )
}

export function ActivityInsights() {
  const { tools, isLoading: toolsLoading } = useTools()

  const toolsWithUsage = useMemo((): ToolWithUsage[] => {
    return tools.slice(0, 8).map((tool) => {
      let description: string | undefined
      try {
        const parsed = typeof tool.toolJson === 'string'
          ? JSON.parse(tool.toolJson)
          : tool.toolJson
        description = parsed?.description
      } catch {
        // ignore parse errors
      }
      return {
        name: tool.name,
        description,
        callCount: 0,
        percentage: 0,
      }
    })
  }, [tools])

  const maxCallCount = useMemo(() => {
    return Math.max(...toolsWithUsage.map(t => t.callCount), 1)
  }, [toolsWithUsage])

  const mockSparkData = useMemo(() =>
    Array.from({ length: 12 }, () => Math.random() * 100),
    [])

  if (toolsLoading) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <BarChart3Icon className="size-4" />
            Activity Insights
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-3 gap-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </div>
          <Skeleton className="h-32" />
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <BarChart3Icon className="size-4" />
            Activity Insights
          </CardTitle>
          <Badge variant="outline" size="sm">Live</Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-3 gap-3">
          <InsightCard
            title="Tools Available"
            value={tools.length}
            icon={WrenchIcon}
            sparkData={mockSparkData}
            delay={0}
          />
          <InsightCard
            title="Throughput"
            value="—"
            subtitle="calls/min"
            icon={TrendingUpIcon}
            delay={0.05}
          />
          <InsightCard
            title="Latency p50"
            value="—"
            subtitle="ms"
            icon={ZapIcon}
            delay={0.1}
          />
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <h4 className="text-xs font-medium text-muted-foreground">
              Available Tools
            </h4>
            <span className="text-xs text-muted-foreground">
              {tools.length} total
            </span>
          </div>

          {toolsWithUsage.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-4 text-center">
              <WrenchIcon className="mb-2 size-6 text-muted-foreground/30" />
              <p className="text-xs text-muted-foreground">No tools available</p>
            </div>
          ) : (
            <div className="space-y-0.5">
              {toolsWithUsage.map((tool, i) => (
                <TopToolRow
                  key={tool.name}
                  tool={tool}
                  index={i}
                  maxCount={maxCallCount}
                />
              ))}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
