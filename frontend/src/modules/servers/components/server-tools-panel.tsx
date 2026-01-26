// Input: serverName to filter tools, useToolsByServer hook data
// Output: ServerToolsPanel component with master-detail layout for browsing tools
// Position: Tab panel within servers page for Tools tab

import type { ToolEntry } from '@bindings/mcpd/internal/ui'
import { SearchIcon, WrenchIcon } from 'lucide-react'
import { AnimatePresence, m } from 'motion/react'
import { useMemo, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Spring } from '@/lib/spring'
import { getToolDisplayName } from '@/lib/tool-names'
import { cn } from '@/lib/utils'
import { ToolDetailPanel } from '@/modules/tools/components/tool-detail-panel'
import { useToolsByServer } from '@/modules/tools/hooks'

interface ServerToolsPanelProps {
  serverName: string | null
}

export function ServerToolsPanel({ serverName }: ServerToolsPanelProps) {
  const { serverMap, isLoading } = useToolsByServer()
  const [selectedTool, setSelectedTool] = useState<ToolEntry | null>(null)
  const [searchQuery, setSearchQuery] = useState('')

  // Find server by serverName (not by specKey which is the map key)
  let server = null
  for (const serverGroup of serverMap.values()) {
    if (serverGroup.serverName === serverName) {
      server = serverGroup
      break
    }
  }

  const tools = server?.tools ?? []

  const filteredTools = useMemo(() => {
    if (!searchQuery.trim()) return tools
    const query = searchQuery.toLowerCase()
    return tools.filter((tool) => {
      const displayName = getToolDisplayName(tool.name, serverName ?? undefined)
      const description = tool.toolJson?.description ?? ''
      return (
        displayName.toLowerCase().includes(query)
        || description.toLowerCase().includes(query)
      )
    })
  }, [tools, searchQuery, serverName])

  if (!serverName) {
    return (
      <Empty className="h-full">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <WrenchIcon className="size-4" />
          </EmptyMedia>
          <EmptyTitle>No Server Selected</EmptyTitle>
          <EmptyDescription>
            Select a server to browse its available tools
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  if (isLoading) {
    return (
      <div className="flex h-full">
        <div className="w-64 shrink-0 border-r p-3 space-y-2">
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
        <div className="flex-1 p-6">
          <Skeleton className="h-8 w-48 mb-4" />
          <Skeleton className="h-4 w-full mb-2" />
          <Skeleton className="h-4 w-3/4" />
        </div>
      </div>
    )
  }

  if (tools.length === 0) {
    return (
      <Empty className="h-full">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <WrenchIcon className="size-4" />
          </EmptyMedia>
          <EmptyTitle>No Tools Available</EmptyTitle>
          <EmptyDescription>
            This server has not registered any tools yet
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className="flex h-full">
      <div className="w-50 shrink-0 border-r flex flex-col">
        <div className="p-3 border-b">
          <div className="relative">
            <SearchIcon className="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
            <Input
              type="search"
              placeholder="Search tools..."
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              className="pl-8"
            />
          </div>
        </div>
        <ScrollArea className="flex-1">
          <div className="p-2 space-y-1">
            <AnimatePresence mode="popLayout">
              {filteredTools.map((tool) => {
                const displayName = getToolDisplayName(
                  tool.name,
                  serverName ?? undefined,
                )
                const isSelected = selectedTool?.name === tool.name

                return (
                  <m.div
                    key={tool.name}
                    layout
                    initial={{ opacity: 0, scale: 0.95 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, scale: 0.95 }}
                    transition={Spring.smooth(0.2)}
                  >
                    <Card
                      className={cn(
                        'p-2.5 cursor-pointer transition-colors shadow-none',
                        isSelected
                          ? 'bg-accent border-accent-foreground/20'
                          : 'hover:bg-muted/50',
                      )}
                      onClick={() => setSelectedTool(tool)}
                    >
                      <div className="flex items-start gap-2">
                        <WrenchIcon className="size-4 mt-0.5 shrink-0 text-muted-foreground" />
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-medium font-mono truncate">
                            {displayName}
                          </p>
                        </div>
                      </div>
                    </Card>
                  </m.div>
                )
              })}
            </AnimatePresence>
            {filteredTools.length === 0 && searchQuery && (
              <m.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                className="p-4 text-center text-sm text-muted-foreground"
              >
                No tools match "{searchQuery}"
              </m.div>
            )}
          </div>
        </ScrollArea>
        <div className="p-2 border-t">
          <Badge variant="secondary" className="w-full justify-center">
            {filteredTools.length} of {tools.length} tools
          </Badge>
        </div>
      </div>
      <div className="flex-1 min-w-0">
        <ToolDetailPanel tool={selectedTool} serverName={serverName} />
      </div>
    </div>
  )
}
