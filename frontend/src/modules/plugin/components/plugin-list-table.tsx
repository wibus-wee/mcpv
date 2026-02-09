// Input: PluginListEntry array, edit handler
// Output: Data table with plugin list and actions
// Position: Main table component for plugins page

import type { PluginListEntry } from '@bindings/mcpv/internal/ui/types'
import { AlertCircleIcon, CheckCircleIcon, MapPinIcon, MinusCircleIcon, PencilIcon } from 'lucide-react'
import { useCallback, useState } from 'react'

import { UniversalEmptyState } from '@/components/common/universal-empty-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toastManager } from '@/components/ui/toast'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { useTogglePlugin } from '../hooks'
import { PluginCategoryBadge } from './plugin-category-badge'

interface PluginListTableProps {
  plugins: PluginListEntry[]
  onEditRequest?: (pluginName: string) => void
}

export function PluginListTable({ plugins, onEditRequest }: PluginListTableProps) {
  const [togglingPlugins, setTogglingPlugins] = useState<Set<string>>(() => new Set())
  const togglePlugin = useTogglePlugin()

  const handleToggle = useCallback(async (plugin: PluginListEntry, enabled: boolean) => {
    setTogglingPlugins(prev => new Set(prev).add(plugin.name))

    try {
      await togglePlugin(plugin.name, enabled)
      toastManager.add({
        title: `Plugin "${plugin.name}" ${enabled ? 'enabled' : 'disabled'}`,
        type: 'success',
      })
    }
    catch (error) {
      toastManager.add({
        title: 'Failed to toggle plugin',
        description: error instanceof Error
          ? error.message
          : `Failed to ${enabled ? 'enable' : 'disable'} plugin`,
        type: 'error',
      })
    }
    finally {
      setTogglingPlugins((prev) => {
        const next = new Set(prev)
        next.delete(plugin.name)
        return next
      })
    }
  }, [togglePlugin])

  if (plugins.length === 0) {
    return (
      <UniversalEmptyState
        icon={MapPinIcon}
        title="No plugins configured"
        description="Add plugins to your configuration to enable governance features."
      />
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Category</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Flows</TableHead>
          <TableHead className="text-right">Calls</TableHead>
          <TableHead className="text-right">Rejections</TableHead>
          <TableHead className="text-right">Latency (ms)</TableHead>
          <TableHead className="text-center">Required</TableHead>
          <TableHead className="text-center">Enabled</TableHead>
          <TableHead className="w-20" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {plugins.map((plugin) => {
          const isToggling = togglingPlugins.has(plugin.name)

          return (
            <tr
              key={plugin.name}
              data-slot="table-row"
              className={cn(
                'border-b transition-colors hover:bg-muted/50',
              )}
            >
              <TableCell className="font-medium">
                <div className="flex flex-col gap-0.5">
                  <span>{plugin.name}</span>
                  {plugin.commitHash && (
                    <span className="font-mono text-muted-foreground text-xs">
                      {plugin.commitHash.slice(0, 8)}
                    </span>
                  )}
                </div>
              </TableCell>
              <TableCell>
                <PluginCategoryBadge category={plugin.category} />
              </TableCell>
              <TableCell>
                <PluginStatusBadge status={plugin.status} error={plugin.statusError} />
              </TableCell>
              <TableCell>
                <div className="flex gap-1">
                  {plugin.flows.map(flow => (
                    <Badge
                      key={flow}
                      variant="outline"
                      size="sm"
                      className="capitalize"
                    >
                      {flow}
                    </Badge>
                  ))}
                </div>
              </TableCell>
              <TableCell className="text-right font-mono text-sm tabular-nums">
                {plugin.latestMetrics.callCount.toLocaleString()}
              </TableCell>
              <TableCell className="text-right font-mono text-sm tabular-nums">
                {plugin.latestMetrics.rejectionCount > 0
                  ? (
                      <span className="text-warning-foreground">
                        {plugin.latestMetrics.rejectionCount.toLocaleString()}
                      </span>
                    )
                  : (
                      <span className="text-muted-foreground">0</span>
                    )}
              </TableCell>
              <TableCell className="text-right font-mono text-sm tabular-nums">
                {plugin.latestMetrics.avgLatencyMs > 0
                  ? plugin.latestMetrics.avgLatencyMs.toFixed(2)
                  : (
                      <span className="text-muted-foreground">—</span>
                    )}
              </TableCell>
              <TableCell className="text-center">
                {plugin.required
                  ? (
                      <Badge variant="error" size="sm">
                        Required
                      </Badge>
                    )
                  : (
                      <Badge variant="outline" size="sm">
                        Optional
                      </Badge>
                    )}
              </TableCell>
              <TableCell className="text-center">
                <div className="flex items-center justify-center gap-2">
                  {isToggling && <Spinner className="size-4" />}
                  <Switch
                    checked={plugin.enabled}
                    onCheckedChange={checked => handleToggle(plugin, checked)}
                    disabled={isToggling}
                  />
                </div>
              </TableCell>
              <TableCell>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => onEditRequest?.(plugin.name)}
                >
                  <PencilIcon className="size-4" />
                </Button>
              </TableCell>
            </tr>
          )
        })}

      </TableBody>
    </Table>
  )
}

// PluginStatusBadge displays the runtime status of a plugin
function PluginStatusBadge({ status, error }: { status: string, error?: string }) {
  const statusConfig = {
    running: {
      icon: CheckCircleIcon,
      label: 'Running',
      variant: 'success' as const,
      className: 'text-success-foreground',
    },
    stopped: {
      icon: MinusCircleIcon,
      label: 'Stopped',
      variant: 'secondary' as const,
      className: 'text-muted-foreground',
    },
    error: {
      icon: AlertCircleIcon,
      label: 'Error',
      variant: 'error' as const,
      className: 'text-error-foreground',
    },
  }

  const config = statusConfig[status as keyof typeof statusConfig] || statusConfig.stopped
  const Icon = config.icon

  if (status === 'error' && error) {
    return (
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger>
            <Badge variant={config.variant} size="sm" className="cursor-help gap-1">
              <Icon className="size-3" />
              {config.label}
            </Badge>
          </TooltipTrigger>
          <TooltipContent>
            <p className="max-w-xs text-xs">{error}</p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    )
  }

  return (
    <Badge variant={config.variant} size="sm" className="gap-1">
      <Icon className="size-3" />
      {config.label}
    </Badge>
  )
}
