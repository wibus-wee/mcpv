// Input: Server groups data, selection state, search query
// Output: ToolsSidebar component with collapsible server sections and selection
// Position: Left panel in master-detail tools layout

import { useEffect, useMemo, useState } from 'react'
import { m, AnimatePresence } from 'motion/react'
import { ChevronRightIcon, SearchIcon, ServerIcon, WrenchIcon } from 'lucide-react'

import type { ActiveCaller, ToolEntry } from '@bindings/mcpd/internal/ui'

import { CallerChipGroup } from '@/components/common/caller-chip-group'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useActiveCallers } from '@/hooks/use-active-callers'
import { ServerRuntimeIndicator } from '@/modules/config/components/server-runtime-status'
import { cn } from '@/lib/utils'
import { Spring } from '@/lib/spring'

import type { ServerGroup } from '../hooks'

interface ToolsSidebarProps {
  servers: ServerGroup[]
  selectedServerId: string | null
  selectedToolId: string | null
  onSelectServer: (serverId: string) => void
  onSelectTool: (tool: ToolEntry, server: ServerGroup) => void
  className?: string
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
    const schema = payload as { description?: string }
    return schema.description || ''
  } catch {
    return ''
  }
}

export function ToolsSidebar({
  servers,
  selectedServerId,
  selectedToolId,
  onSelectServer,
  onSelectTool,
  className
}: ToolsSidebarProps) {
  const [searchQuery, setSearchQuery] = useState('')
  const [expandedServers, setExpandedServers] = useState<Set<string>>(() => {
    return new Set(servers.map(s => s.id))
  })
  const { data: activeCallers } = useActiveCallers()

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

  const activeCallersByServer = useMemo(() => {
    const byServer = new Map<string, ActiveCaller[]>()
    const byProfile = new Map<string, ActiveCaller[]>()

    const activeCallerList = activeCallers ?? []
    activeCallerList.forEach(caller => {
      const list = byProfile.get(caller.profile) ?? []
      list.push(caller)
      byProfile.set(caller.profile, list)
    })

    servers.forEach(server => {
      const collected: ActiveCaller[] = []
      const seen = new Set<string>()

      server.profileNames.forEach(profileName => {
        const matches = byProfile.get(profileName) ?? []
        matches.forEach(caller => {
          const key = `${caller.caller}:${caller.pid}`
          if (seen.has(key)) {
            return
          }
          seen.add(key)
          collected.push(caller)
        })
      })

      byServer.set(server.id, collected)
    })

    return byServer
  }, [activeCallers, servers])

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

  useEffect(() => {
    if (!selectedServerId) {
      return
    }
    setExpandedServers(prev => {
      if (prev.has(selectedServerId)) {
        return prev
      }
      const next = new Set(prev)
      next.add(selectedServerId)
      return next
    })
  }, [selectedServerId])

  const handleServerClick = (serverId: string) => {
    toggleServer(serverId)
    onSelectServer(serverId)
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
                const isSelected = selectedServerId === server.id
                const activeForServer = activeCallersByServer.get(server.id) ?? []

                return (
                  <div key={server.id}>
                    <button
                      type="button"
                      onClick={() => handleServerClick(server.id)}
                      className={cn(
                        'w-full flex items-center gap-2 px-2 py-1.5 rounded-md',
                        'text-sm font-medium text-left transition-colors',
                        isSelected ? 'bg-muted/70 text-foreground' : 'hover:bg-muted/50',
                        'group',
                      )}
                    >
                      <m.div
                        animate={{ rotate: isExpanded ? 90 : 0 }}
                        transition={{ duration: 0.15 }}
                      >
                        <ChevronRightIcon className="size-4 text-muted-foreground" />
                      </m.div>
                      <ServerIcon className="size-4 text-muted-foreground" />
                      <span className="flex-1 min-w-0 truncate">{server.serverName}</span>
                      <ServerRuntimeIndicator specKey={server.specKey} />
                      <span className="text-xs text-muted-foreground tabular-nums">
                        {server.tools.length}
                      </span>
                    </button>
                    {activeForServer.length > 0 && (
                      <div className="ml-7 mb-1">
                        <CallerChipGroup callers={activeForServer} maxVisible={2} />
                      </div>
                    )}

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
