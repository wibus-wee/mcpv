// Input: Tools data from hooks, selection state, sidebar and detail panels
// Output: ToolsGrid component with master-detail layout and server detail support
// Position: Main container for tools page with Linear/Vercel-style UX

import { useEffect, useState } from 'react'
import { m } from 'motion/react'
import { ServerOffIcon } from 'lucide-react'

import type { ToolEntry } from '@bindings/mcpd/internal/ui'

import { Skeleton } from '@/components/ui/skeleton'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

import { useToolsByServer, type ServerGroup } from '../hooks'
import { ServerDetailPanel } from './server-detail-panel'
import { ToolsSidebar } from './tools-sidebar'
import { ToolDetailPanel } from './tool-detail-panel'

interface SelectedTool {
  tool: ToolEntry
  serverId: string
  serverName: string
}

interface ToolsGridProps {
  selectedServerId?: string
  onSelectServer: (serverId: string | null) => void
  className?: string
}

export function ToolsGrid({
  selectedServerId,
  onSelectServer,
  className,
}: ToolsGridProps) {
  const { servers, serverMap, isLoading } = useToolsByServer()
  const [selectedTool, setSelectedTool] = useState<SelectedTool | null>(null)

  const handleSelectTool = (tool: ToolEntry, server: ServerGroup) => {
    setSelectedTool({
      tool,
      serverId: server.id,
      serverName: server.serverName,
    })
    onSelectServer(server.id)
  }

  const handleSelectServer = (serverId: string) => {
    setSelectedTool(null)
    onSelectServer(serverId)
  }

  useEffect(() => {
    if (!selectedTool) {
      return
    }
    if (!selectedServerId || selectedTool.serverId !== selectedServerId) {
      setSelectedTool(null)
    }
  }, [selectedServerId, selectedTool])

  if (isLoading) {
    return (
      <div className={cn('flex flex-1 border-t border-border overflow-hidden', className)}>
        <div className="w-72 border-r border-border p-3 space-y-2">
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
        </div>
        <div className="flex-1 p-6 space-y-4">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="h-4 w-full max-w-md" />
          <Skeleton className="h-32 w-full" />
        </div>
      </div>
    )
  }

  if (servers.length === 0) {
    return (
      <div className={cn('flex flex-col items-center justify-center flex-1 gap-4', className)}>
        <ServerOffIcon className="size-16 text-muted-foreground" />
        <div className="text-center">
          <h3 className="text-lg font-semibold">No servers configured</h3>
          <p className="text-sm text-muted-foreground">
            Add MCP servers to your configuration to see tools
          </p>
        </div>
      </div>
    )
  }

  const selectedToolId = selectedTool
    ? `${selectedTool.serverId}:${selectedTool.tool.name}`
    : null
  const selectedServer = selectedServerId
    ? serverMap.get(selectedServerId) ?? null
    : null
  const shouldShowServerPanel = !selectedTool
  const requestedServerId = shouldShowServerPanel ? selectedServerId ?? null : null

  return (
    <m.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={Spring.smooth(0.3)}
      className={cn(
        'flex flex-1 overflow-hidden bg-background border-t',
        className
      )}
    >
      <ToolsSidebar
        servers={servers}
        selectedToolId={selectedToolId}
        selectedServerId={selectedServerId ?? null}
        onSelectServer={handleSelectServer}
        onSelectTool={handleSelectTool}
        className="w-72 shrink-0"
      />
      {selectedTool ? (
        <ToolDetailPanel
          tool={selectedTool.tool}
          serverName={selectedTool.serverName}
          className="flex-1"
        />
      ) : (
        <ServerDetailPanel
          server={selectedServer}
          requestedServerId={requestedServerId}
          onSelectTool={handleSelectTool}
          className="flex-1"
        />
      )}
    </m.div>
  )
}
