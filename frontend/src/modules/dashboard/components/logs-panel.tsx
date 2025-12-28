// Input: Card, Badge, Button, ScrollArea, Switch, Select components, logs hook
// Output: LogsPanel component displaying real-time logs
// Position: Dashboard logs section with filtering

import {
  AlertCircleIcon,
  AlertTriangleIcon,
  BugIcon,
  InfoIcon,
  RefreshCwIcon,
  ScrollTextIcon,
  TrashIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useMemo, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { useCoreState } from '@/hooks/use-core-state'
import type { LogEntry } from '@/hooks/use-logs'
import { useLogs } from '@/hooks/use-logs'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

const levelConfig = {
  debug: {
    icon: BugIcon,
    color: 'text-muted-foreground',
    badge: 'secondary' as const,
  },
  info: {
    icon: InfoIcon,
    color: 'text-info',
    badge: 'info' as const,
  },
  warn: {
    icon: AlertTriangleIcon,
    color: 'text-warning',
    badge: 'warning' as const,
  },
  error: {
    icon: AlertCircleIcon,
    color: 'text-destructive',
    badge: 'error' as const,
  },
}

function LogItem({ log }: { log: LogEntry }) {
  const config = levelConfig[log.level] ?? levelConfig.info
  const Icon = config.icon

  return (
    <div className="flex items-start gap-3 py-2 px-3 hover:bg-muted/50 rounded-md transition-colors">
      <Icon className={cn('size-4 mt-0.5 shrink-0', config.color)} />
      <div className="flex-1 min-w-0 space-y-1">
        <div className="flex items-center gap-2">
          <Badge variant={config.badge} size="sm">
            {log.level}
          </Badge>
          {log.source && (
            <span className="text-muted-foreground text-xs font-mono">
              {log.source}
            </span>
          )}
          <span className="text-muted-foreground text-xs ml-auto">
            {log.timestamp.toLocaleTimeString()}
          </span>
        </div>
        <p className="text-sm break-words">{log.message}</p>
        <div className="text-xs font-mono text-muted-foreground">
          {log.source}
        </div>
      </div>
    </div>
  )
}

export function LogsPanel() {
  const { logs, mutate } = useLogs()
  const { coreStatus } = useCoreState()
  const [levelFilter, setLevelFilter] = useState<string>('all')
  const [autoScroll, setAutoScroll] = useState(true)

  const filteredLogs = useMemo(() => {
    if (levelFilter === 'all') return logs
    return logs.filter(log => log.level === levelFilter)
  }, [logs, levelFilter])

  const clearLogs = () => {
    mutate([], { revalidate: false })
  }

  const forceRefresh = () => {
    console.log('[LogsPanel] Force refresh triggered, current core status:', coreStatus)
    // This will trigger the log stream to restart in root-provider
    window.location.reload()
  }

  const isConnected = coreStatus === 'running' && logs.length > 0
  const isDisconnected = coreStatus === 'stopped' || coreStatus === 'error'
  const isWaiting = coreStatus === 'running' && logs.length === 0

  return (
    <m.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.4, 0.2)}
    >
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <ScrollTextIcon className="size-5" />
              Logs
              <Badge variant="secondary" size="sm">
                {logs.length}
              </Badge>
              {isConnected && (
                <Badge variant="success" size="sm" className="ml-1">
                  Connected
                </Badge>
              )}
              {isDisconnected && (
                <Badge variant="error" size="sm" className="ml-1">
                  Disconnected
                </Badge>
              )}
              {isWaiting && (
                <Badge variant="warning" size="sm" className="ml-1">
                  Waiting...
                </Badge>
              )}
            </CardTitle>
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2">
                <Checkbox
                  id="auto-scroll"
                  checked={autoScroll}
                  onCheckedChange={checked => setAutoScroll(checked === true)}
                />
                <Label htmlFor="auto-scroll" className="text-sm">
                  Auto-scroll
                </Label>
              </div>
              <Select value={levelFilter} onValueChange={setLevelFilter}>
                <SelectTrigger size="sm" className="w-32">
                  <SelectValue placeholder="Filter level" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All levels</SelectItem>
                  <SelectItem value="debug">Debug</SelectItem>
                  <SelectItem value="info">Info</SelectItem>
                  <SelectItem value="warn">Warning</SelectItem>
                  <SelectItem value="error">Error</SelectItem>
                </SelectContent>
              </Select>
              {isWaiting && (
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={forceRefresh}
                  title="Refresh page to reconnect"
                >
                  <RefreshCwIcon className="size-4" />
                </Button>
              )}
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={clearLogs}
              >
                <TrashIcon className="size-4" />
              </Button>
            </div>
          </div>
        </CardHeader>
        <Separator />
        <CardContent className="p-0">
          <ScrollArea className="h-80">
            {filteredLogs.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full py-12 text-muted-foreground">
                <ScrollTextIcon className="size-8 mb-2 opacity-50" />
                {isDisconnected ? (
                  <>
                    <p className="text-sm font-medium">Core is not running</p>
                    <p className="text-xs">Start the core to see logs</p>
                  </>
                ) : isWaiting ? (
                  <>
                    <p className="text-sm font-medium">Waiting for logs...</p>
                    <p className="text-xs">Check browser console for debug info</p>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={forceRefresh}
                      className="mt-2"
                    >
                      <RefreshCwIcon className="size-3 mr-1" />
                      Refresh Page
                    </Button>
                  </>
                ) : (
                  <>
                    <p className="text-sm">No logs yet</p>
                    <p className="text-xs">Logs will appear here when the core is running</p>
                  </>
                )}
              </div>
            ) : (
              <div className="divide-y">
                {filteredLogs.map(log => (
                  <LogItem key={log.id} log={log} />
                ))}
              </div>
            )}
          </ScrollArea>
        </CardContent>
      </Card>
    </m.div>
  )
}
