// Input: Tools data from hooks, sidebar and detail panel components
// Output: ToolsGrid component with master-detail layout
// Position: Main container for tools page with Linear/Vercel-style UX

import { useState } from 'react'
import { m } from 'motion/react'
import { ServerOffIcon } from 'lucide-react'

import type { ToolEntry } from '@bindings/mcpd/internal/ui'

import { Skeleton } from '@/components/ui/skeleton'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

import { useToolsByServer } from '../hooks'
import { ToolsSidebar } from './tools-sidebar'
import { ToolDetailPanel } from './tool-detail-panel'

interface SelectedTool {
  tool: ToolEntry
  serverId: string
  serverName: string
}

interface ToolsGridProps {
  className?: string
}

export function ToolsGrid({ className }: ToolsGridProps) {
  const { servers, isLoading } = useToolsByServer()
  const [selectedTool, setSelectedTool] = useState<SelectedTool | null>(null)

  const handleSelectTool = (tool: ToolEntry, server: { id: string; serverName: string }) => {
    setSelectedTool({
      tool,
      serverId: server.id,
      serverName: server.serverName
    })
  }

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
        onSelectTool={handleSelectTool}
        className="w-72 shrink-0"
      />
      <ToolDetailPanel
        tool={selectedTool?.tool ?? null}
        serverName={selectedTool?.serverName}
        className="flex-1"
      />
    </m.div>
  )
}
