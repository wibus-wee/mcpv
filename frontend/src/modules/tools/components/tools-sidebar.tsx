// Input: Server groups data, selection state, search query
// Output: ToolsSidebar component with collapsible server sections
// Position: Left panel in master-detail tools layout

import { useState, useMemo } from 'react'
import { m, AnimatePresence } from 'motion/react'
import { ChevronRightIcon, SearchIcon, ServerIcon, WrenchIcon } from 'lucide-react'

import type { ToolEntry } from '@bindings/mcpd/internal/ui'

import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ServerRuntimeIndicator } from '@/modules/config/components/server-runtime-status'
import { cn } from '@/lib/utils'
import { Spring } from '@/lib/spring'

interface ServerGroup {
  id: string
  specKey: string
  serverName: string
  tools: ToolEntry[]
  profileNames: string[]
  hasToolData: boolean
}

interface ToolsSidebarProps {
  servers: ServerGroup[]
  selectedToolId: string | null
  onSelectTool: (tool: ToolEntry, server: ServerGroup) => void
  className?: string
}

function parseToolDescription(tool: ToolEntry): string {
  try {
    const schema = JSON.parse(tool.toolJson as string)
    return schema.description || ''
  } catch {
    return ''
  }
}

export function ToolsSidebar({
  servers,
  selectedToolId,
  onSelectTool,
  className
}: ToolsSidebarProps) {
  const [searchQuery, setSearchQuery] = useState('')
  const [expandedServers, setExpandedServers] = useState<Set<string>>(() => {
    return new Set(servers.map(s => s.id))
  })

  const filteredServers = useMemo(() => {
    if (!searchQuery.trim()) return servers

    const query = searchQuery.toLowerCase()
    return servers
      .map(server => {
        const matchingTools = server.tools.filter(tool => {
          const desc = parseToolDescription(tool)
          return (
            tool.name.toLowerCase().includes(query) ||
            desc.toLowerCase().includes(query)
          )
        })

        const serverMatches = server.serverName.toLowerCase().includes(query)

        if (serverMatches) {
          return server
        }

        if (matchingTools.length > 0) {
          return { ...server, tools: matchingTools }
        }

        return null
      })
      .filter((s): s is ServerGroup => s !== null)
  }, [servers, searchQuery])

  const toggleServer = (serverId: string) => {
    setExpandedServers(prev => {
      const next = new Set(prev)
      if (next.has(serverId)) {
        next.delete(serverId)
      } else {
        next.add(serverId)
      }
      return next
    })
  }

  const totalTools = servers.reduce((acc, s) => acc + s.tools.length, 0)

  return (
    <div className={cn('flex flex-col h-full border-r border-border', className)}>
      <div className="p-3 border-b border-border">
        <div className="relative">
          <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
          <Input
            placeholder={`Search ${totalTools} tools...`}
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="pl-9 h-9 bg-muted/50"
          />
        </div>
      </div>

      <ScrollArea className="flex-1">
        <div className="p-2">
          {filteredServers.length === 0 ? (
            <div className="px-3 py-8 text-center text-sm text-muted-foreground">
              {searchQuery ? 'No tools match your search' : 'No servers configured'}
            </div>
          ) : (
            <div className="space-y-1">
              {filteredServers.map(server => {
                const isExpanded = expandedServers.has(server.id)

                return (
                  <div key={server.id}>
                    <button
                      type="button"
                      onClick={() => toggleServer(server.id)}
                      className={cn(
                        'w-full flex items-center gap-2 px-2 py-1.5 rounded-md',
                        'text-sm font-medium text-left',
                        'hover:bg-muted/50 transition-colors',
                        'group'
                      )}
                    >
                      <m.div
                        animate={{ rotate: isExpanded ? 90 : 0 }}
                        transition={{ duration: 0.15 }}
                      >
                        <ChevronRightIcon className="size-4 text-muted-foreground" />
                      </m.div>
                      <ServerIcon className="size-4 text-muted-foreground" />
                      <span className="flex-1 truncate">{server.serverName}</span>
                      <ServerRuntimeIndicator specKey={server.specKey} />
                      <span className="text-xs text-muted-foreground tabular-nums">
                        {server.tools.length}
                      </span>
                    </button>

                    <AnimatePresence initial={false}>
                      {isExpanded && server.tools.length > 0 && (
                        <m.div
                          initial={{ height: 0, opacity: 0 }}
                          animate={{ height: 'auto', opacity: 1 }}
                          exit={{ height: 0, opacity: 0 }}
                          transition={Spring.snappy(0.2)}
                          className="overflow-hidden"
                        >
                          <div className="ml-4 pl-2 border-l border-border/50 space-y-0.5 py-1">
                            {server.tools.map(tool => {
                              const isSelected = selectedToolId === `${server.id}:${tool.name}`

                              return (
                                <button
                                  key={tool.name}
                                  type="button"
                                  onClick={() => onSelectTool(tool, server)}
                                  className={cn(
                                    'w-full flex items-center gap-2 px-2 py-1.5 rounded-md',
                                    'text-sm text-left transition-colors',
                                    isSelected
                                      ? 'bg-primary/10 text-primary'
                                      : 'hover:bg-muted/50 text-foreground/80'
                                  )}
                                >
                                  <WrenchIcon className={cn(
                                    'size-3.5 shrink-0',
                                    isSelected ? 'text-primary' : 'text-muted-foreground'
                                  )} />
                                  <span className="truncate font-mono text-xs">
                                    {tool.name}
                                  </span>
                                </button>
                              )
                            })}
                          </div>
                        </m.div>
                      )}
                    </AnimatePresence>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
