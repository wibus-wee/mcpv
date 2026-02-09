// Input: ServerSummary array, selection state, loading flag
// Output: ServersList component - unified list with search, tag filters, collapsible sections
// Position: Left panel in servers page layout

import type { ServerSummary } from '@bindings/mcpv/internal/ui/types'
import { FilterIcon, SearchIcon, ServerIcon, TagIcon } from 'lucide-react'
import { AnimatePresence, m } from 'motion/react'
import { useEffect, useMemo, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'
import { ServerEmptyState } from '@/modules/servers/components/server-empty-state'
import { ServerRuntimeIndicator } from '@/modules/servers/components/server-runtime-status'
import { useFilteredServers } from '@/modules/servers/hooks'

interface ServersListProps {
  servers: ServerSummary[]
  selectedServer: string | null
  onSelect: (name: string) => void
  isLoading: boolean
}

function ServersListSkeleton() {
  return (
    <div className="space-y-2 p-3">
      {Array.from({ length: 5 }).map((_, i) => (
        <Skeleton key={i} className="h-12 w-full rounded-md" />
      ))}
    </div>
  )
}

function ServersListEmpty() {
  return (
    <ServerEmptyState
      title="No servers"
      description="Add MCP servers to start routing tools."
    />
  )
}

function buildTagIndex(servers: ServerSummary[]) {
  const tagSet = new Set<string>()
  servers.forEach((server) => {
    server.tags?.forEach(tag => tagSet.add(tag))
  })
  return Array.from(tagSet).sort((a, b) => a.localeCompare(b))
}

export function ServersList({
  servers,
  selectedServer,
  onSelect,
  isLoading,
}: ServersListProps) {
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedTags, setSelectedTags] = useState<string[]>([])

  const tags = useMemo(() => buildTagIndex(servers), [servers])

  const filteredServers = useFilteredServers(servers, searchQuery, selectedTags)

  useEffect(() => {
    if (selectedTags.length === 0) return
    setSelectedTags(current => current.filter(tag => tags.includes(tag)))
  }, [tags, selectedTags.length])

  if (isLoading) {
    return <ServersListSkeleton />
  }

  if (servers.length === 0) {
    return <ServersListEmpty />
  }

  const hasFilters = tags.length > 0

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border bg-background/50 p-3 space-y-3">
        <div className="relative">
          <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
          <Input
            placeholder={`Search ${servers.length} servers...`}
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="pl-9 h-9 bg-background border-muted-foreground/20 focus:border-primary/50"
          />
        </div>

        {hasFilters && (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <FilterIcon className="size-3.5" />
              Filter by tag
            </div>
            <ToggleGroup
              multiple
              value={selectedTags}
              onValueChange={values => setSelectedTags(values as string[])}
              className="flex flex-wrap gap-1.5"
            >
              {tags.map(tag => (
                <ToggleGroupItem key={tag} value={tag} size="sm" variant="outline">
                  <TagIcon className="size-3" />
                  {tag}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
        )}
      </div>

      <ScrollArea className="flex-1">
        <div className="p-2">
          {filteredServers.length === 0 ? (
            <div className="rounded-lg border border-dashed p-4 text-xs text-muted-foreground text-center">
              {searchQuery || selectedTags.length > 0
                ? 'No servers match your filters.'
                : 'No servers available.'}
            </div>
          ) : (
            <div className="space-y-1">
              <AnimatePresence initial={false}>
                {filteredServers.map((server, index) => {
                  const isActive = server.name === selectedServer
                  const serverTags = server.tags ?? []
                  const displayTags = serverTags.slice(0, 2)
                  const extraTags = serverTags.length - displayTags.length

                  return (
                    <m.button
                      key={server.name}
                      type="button"
                      initial={{ opacity: 0, y: 6 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -6 }}
                      transition={Spring.snappy(0.25, index * 0.02)}
                      onClick={() => onSelect(server.name)}
                      className={cn(
                        'w-full rounded-lg border px-3 py-2 text-left transition-colors',
                        isActive
                          ? 'border-primary/60 bg-primary/10'
                          : 'border-border/60 hover:bg-muted/40',
                      )}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <ServerIcon className="size-4 shrink-0 text-muted-foreground" />
                            <span className="truncate text-sm font-medium">
                              {server.name}
                            </span>
                            {server.disabled && (
                              <Badge variant="warning" size="sm">
                                Disabled
                              </Badge>
                            )}
                          </div>
                          {displayTags.length > 0 && (
                            <div className="mt-1 ml-6 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                              {displayTags.map(tag => (
                                <Badge key={`${server.name}-${tag}`} variant="outline" size="sm">
                                  {tag}
                                </Badge>
                              ))}
                              {extraTags > 0 && (
                                <span className="text-[10px] text-muted-foreground">
                                  +{extraTags} more
                                </span>
                              )}
                            </div>
                          )}
                        </div>
                        <ServerRuntimeIndicator specKey={server.specKey} />
                      </div>
                    </m.button>
                  )
                })}
              </AnimatePresence>
            </div>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
