// Input: ServerGroup data, runtime status summary, tool entries
// Output: ServerDetailPanel component showing server overview and tool list
// Position: Right panel in tools master-detail layout for server context

import { m } from 'motion/react'
import { ServerIcon, WrenchIcon } from 'lucide-react'

import type { ToolEntry } from '@bindings/mcpd/internal/ui'

import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'
import { ServerRuntimeSummary } from '@/modules/config/components/server-runtime-status'
import { useRuntimeStatus, useServerInitStatus } from '@/modules/config/hooks'

import type { ServerGroup } from '../hooks'

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

  const profileNames = [...server.profileNames].sort((a, b) => a.localeCompare(b))
  const toolList = [...server.tools].sort((a, b) => a.name.localeCompare(b.name))
  const isRuntimeLoading = runtimeStatus === undefined && initStatus === undefined
  const hasRuntimeData =
    runtimeStatus?.some(status => status.specKey === server.specKey) ||
    initStatus?.some(status => status.specKey === server.specKey)

  return (
    <ScrollArea className={cn('h-full', className)}>
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

        <div className="space-y-2">
          <h3 className="text-sm font-semibold">Profiles</h3>
          {profileNames.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {profileNames.map(profile => (
                <Badge key={profile} variant="secondary" size="sm">
                  {profile}
                </Badge>
              ))}
            </div>
          ) : (
            <Card className="p-4">
              <p className="text-xs text-muted-foreground text-center">
                This server is not assigned to any profile.
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
