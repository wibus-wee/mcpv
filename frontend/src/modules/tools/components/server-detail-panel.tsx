// Input: ServerGroup data, runtime status summary, tool entries
// Input: motion animations, Wails runtime status bindings, tools data
// Output: ServerDetailPanel component showing server overview, runtime, and tools
// Position: Right panel in tools master-detail layout for server context

import { m } from 'motion/react'
import { ServerIcon, WrenchIcon } from 'lucide-react'

import type { StartCause, ToolEntry } from '@bindings/mcpd/internal/ui'

import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'
import { ServerRuntimeSummary } from '@/modules/config/components/server-runtime-status'
import { useRuntimeStatus, useServerInitStatus } from '@/modules/config/hooks'

import type { ServerGroup } from '../hooks'
import { formatRelativeTime } from '@/lib/time'

interface ServerDetailPanelProps {
  server: ServerGroup | null
  requestedServerId?: string | null
  onSelectTool: (tool: ToolEntry, server: ServerGroup) => void
  className?: string
}

type ToolSchema = {
  description?: string
}

function parseToolDescription(tool: ToolEntry): string {
  if (!tool.toolJson) {
    return ''
  }
  try {
    const payload =
      typeof tool.toolJson === 'string' ? JSON.parse(tool.toolJson) : tool.toolJson
    if (!payload || typeof payload !== 'object') {
      return ''
    }
    const schema = payload as ToolSchema
    return typeof schema.description === 'string' ? schema.description : ''
  } catch {
    return ''
  }
}

function formatStartReason(
  cause?: StartCause | null,
  activationMode?: string,
  minReady?: number,
): string {
  if (!cause?.reason) {
    return 'Unknown reason (no info)'
  }
  switch (cause.reason) {
    case 'bootstrap': {
      const policyLabel = resolvePolicyLabel(cause, activationMode, minReady)
      if (policyLabel !== '—') {
        return `Refresh tool metadata · ${policyLabel} keep-alive`
      }
      return 'Refresh tool metadata'
    }
    case 'tool_call':
      return 'Triggered by tool call'
    case 'client_activate':
      return 'Triggered by client activation'
    case 'policy_always_on':
      return 'always-on running'
    case 'policy_min_ready':
      return `minReady=${cause.policy?.minReady ?? 0} minimum ready`
    default:
      return `Unknown reason (${cause.reason})`
  }
}

function formatStartTriggerLines(cause?: StartCause | null): string[] {
  if (!cause) {
    return []
  }
  const lines = [] as string[]
  if (cause.client) {
    lines.push(`client: ${cause.client}`)
  }
  if (cause.toolName) {
    lines.push(`tool: ${cause.toolName}`)
  }
  return lines
}

function formatPolicyLabel(cause?: StartCause | null): string {
  if (!cause?.policy) {
    return '—'
  }
  if (cause.policy.activationMode === 'always-on') {
    return 'always-on'
  }
  if (cause.policy.minReady > 0) {
    return `minReady=${cause.policy.minReady}`
  }
  return '—'
}

function resolvePolicyLabel(
  cause: StartCause | null | undefined,
  activationMode?: string,
  minReady?: number,
): string {
  if (cause?.policy) {
    return formatPolicyLabel(cause)
  }
  if (activationMode === 'always-on') {
    return 'always-on'
  }
  if (minReady && minReady > 0) {
    return `minReady=${minReady}`
  }
  return '—'
}


function resolveStartCause(
  cause: StartCause | null | undefined,
  activationMode?: string,
  minReady?: number,
): StartCause | null {
  if (cause?.reason) {
    return cause
  }
  if (!activationMode && !minReady) {
    return null
  }
  if (activationMode === 'always-on') {
    return {
      reason: 'policy_always_on',
      timestamp: '',
      policy: {
        activationMode,
        minReady: minReady ?? 0,
      },
    }
  }
  if (minReady && minReady > 0) {
    return {
      reason: 'policy_min_ready',
      timestamp: '',
      policy: {
        activationMode: activationMode ?? 'on-demand',
        minReady,
      },
    }
  }
  return null
}

export function ServerDetailPanel({
  server,
  requestedServerId,
  onSelectTool,
  className,
}: ServerDetailPanelProps) {
  const { data: runtimeStatus } = useRuntimeStatus()
  const { data: initStatus } = useServerInitStatus()

  if (!server) {
    const title = requestedServerId ? 'Server not found' : 'Select a server'
    const description = requestedServerId
      ? 'This server is no longer available. Pick another server from the list.'
      : 'Choose a server on the left to see runtime details and tools.'

    return (
      <div className={cn('flex flex-col items-center justify-center h-full text-muted-foreground', className)}>
        <ServerIcon className="size-12 mb-4 opacity-20" />
        <p className="text-sm font-medium">{title}</p>
        <p className="text-xs mt-1">{description}</p>
      </div>
    )
  }

  const toolList = [...server.tools].sort((a, b) => a.name.localeCompare(b.name))
  const specDetail = server.specDetail
  const isRuntimeLoading = runtimeStatus === undefined && initStatus === undefined
  const runtimeEntry = runtimeStatus?.find(status => status.specKey === server.specKey)
  const instanceStatuses = runtimeEntry?.instances ?? []
  const sortedInstances = [...instanceStatuses].sort((a, b) =>
    a.id.localeCompare(b.id),
  )
  const hasRuntimeData =
    runtimeStatus?.some(status => status.specKey === server.specKey) ||
    initStatus?.some(status => status.specKey === server.specKey)

  return (
    <ScrollArea className={cn('h-full w-full', className)}>
      <m.div
        key={server.id}
        initial={{ opacity: 0, x: 20 }}
        animate={{ opacity: 1, x: 0 }}
        transition={Spring.smooth(0.3)}
        className="p-6 space-y-6 min-w-0 w-full"
      >
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h2 className="text-xl font-semibold truncate">{server.serverName}</h2>
            <p className="text-xs text-muted-foreground mt-1">
              Spec key <code className="font-mono">{server.specKey}</code>
            </p>
          </div>
          <Badge variant="outline" size="sm" className="shrink-0">
            {server.tools.length} tools
          </Badge>
        </div>

        <div className="space-y-2">
          <h3 className="text-sm font-semibold">Runtime</h3>
          {isRuntimeLoading ? (
            <Card className="p-4">
              <p className="text-xs text-muted-foreground text-center">
                Loading runtime status...
              </p>
            </Card>
          ) : hasRuntimeData ? (
            <ServerRuntimeSummary specKey={server.specKey} />
          ) : (
            <Card className="p-4">
              <p className="text-xs text-muted-foreground text-center">
                Runtime data has not been reported yet.
              </p>
            </Card>
          )}
        </div>

        <div className="space-y-2 w-full">
          <h3 className="text-sm font-semibold">Why it&apos;s on</h3>
          {sortedInstances.length > 0 ? (
            <Card className="p-0 overflow-x-auto max-w-full">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Instance</TableHead>
                    <TableHead>Cause</TableHead>
                    <TableHead>Trigger</TableHead>
                    <TableHead>Policy</TableHead>
                    <TableHead>Time</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sortedInstances.map(instance => {
                    const resolvedCause = resolveStartCause(
                      instance.lastStartCause,
                      specDetail?.activationMode,
                      specDetail?.minReady,
                    )
                    const triggerLines = formatStartTriggerLines(resolvedCause)
                    const relativeTime = formatRelativeTime(resolvedCause?.timestamp)
                    const policyLabel = resolvePolicyLabel(
                      resolvedCause,
                      specDetail?.activationMode,
                      specDetail?.minReady,
                    )
                    return (
                      <TableRow key={instance.id}>
                        <TableCell className="font-mono text-xs">
                          {instance.id}
                        </TableCell>
                        <TableCell className="text-xs">
                          {formatStartReason(
                            resolvedCause,
                            specDetail?.activationMode,
                            specDetail?.minReady,
                          )}
                        </TableCell>
                        <TableCell className="text-xs">
                          {triggerLines.length > 0 ? (
                            <div className="space-y-1">
                              {triggerLines.map(line => (
                                <p key={line} className="text-xs text-muted-foreground">
                                  {line}
                                </p>
                              ))}
                            </div>
                          ) : (
                            <span className="text-xs text-muted-foreground">—</span>
                          )}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {policyLabel}
                        </TableCell>
                        <TableCell
                          className="text-xs text-muted-foreground"
                          title={resolvedCause?.timestamp || ''}
                        >
                          {relativeTime}
                        </TableCell>
                      </TableRow>
                    )
                  })}


                </TableBody>
              </Table>
            </Card>
          ) : (
            <Card className="p-4">
              <p className="text-xs text-muted-foreground text-center">
                No runtime instances reported yet.
              </p>
            </Card>
          )}
        </div>

        <div className="space-y-2">
          <h3 className="text-sm font-semibold">Tags</h3>
          {server.tags.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {server.tags.map(tag => (
                <Badge key={tag} variant="secondary" size="sm">
                  {tag}
                </Badge>
              ))}
            </div>
          ) : (
            <Card className="p-4">
              <p className="text-xs text-muted-foreground text-center">
                This server does not have any tags.
              </p>
            </Card>
          )}
        </div>

        <div className="space-y-2">
          <h3 className="text-sm font-semibold">Tools</h3>
          {toolList.length > 0 ? (
            <div className="space-y-2">
              {toolList.map(tool => {
                const description = parseToolDescription(tool)
                const isCached = tool.source === 'cache'
                const cachedLabel = tool.cachedAt
                  ? `Cached ${formatRelativeTime(tool.cachedAt)}`
                  : 'Cached metadata'

                return (
                  <button
                    key={tool.name}
                    type="button"
                    onClick={() => onSelectTool(tool, server)}
                    className={cn(
                      'w-full text-left rounded-md border border-border/60 bg-card/40',
                      'px-3 py-2 transition-colors hover:bg-muted/60'
                    )}
                  >
                    <div className="flex items-center gap-2">
                      <WrenchIcon className="size-3.5 text-muted-foreground" />
                      <span className="font-mono text-xs text-foreground/90">
                        {tool.name}
                      </span>
                      {isCached && (
                        <Badge
                          variant="outline"
                          size="sm"
                          className="ml-auto"
                          title={cachedLabel}
                        >
                          cached
                        </Badge>
                      )}
                    </div>
                    {description && (
                      <p className="text-xs text-muted-foreground mt-1.5 leading-relaxed">
                        {description}
                      </p>
                    )}
                  </button>
                )
              })}
            </div>
          ) : (
            <Card className="p-4">
              <p className="text-xs text-muted-foreground text-center">
                No tools are reported for this server.
              </p>
            </Card>
          )}
        </div>
      </m.div>
    </ScrollArea>
  )
}
