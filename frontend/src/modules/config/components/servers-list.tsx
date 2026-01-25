// Input: ServerSummary array, selection state, runtime indicator
// Output: ServersList component - list view with tag filters
// Position: Left panel in config page master-detail layout

import type { ServerSummary } from '@bindings/mcpd/internal/ui'
import { FilterIcon, ServerIcon, TagIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useEffect, useMemo, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

import { ServerRuntimeIndicator } from './server-runtime-status'

interface ServersListProps {
  servers: ServerSummary[]
  selectedServer: string | null
  onSelect: (name: string) => void
  isLoading: boolean
  onRefresh: () => void
}

function ServersListSkeleton() {
  return (
    <div className="space-y-2">
      {Array.from({ length: 4 }).map((_, i) => (
        <Skeleton key={i} className="h-12 w-full rounded-md" />
      ))}
    </div>
  )
}

function ServersListEmpty() {
  return (
    <Empty className="py-8">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <ServerIcon className="size-4" />
        </EmptyMedia>
        <EmptyTitle className="text-sm">No servers</EmptyTitle>
        <EmptyDescription className="text-xs">
          Add MCP servers to start routing tools.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

function buildTagIndex(servers: ServerSummary[]) {
  const tagSet = new Set<string>()
  servers.forEach(server => {
    server.tags?.forEach(tag => tagSet.add(tag))
  })
  return Array.from(tagSet).sort((a, b) => a.localeCompare(b))
}

export function ServersList({
  servers,
  selectedServer,
  onSelect,
  isLoading,
  onRefresh,
}: ServersListProps) {
  const [selectedTags, setSelectedTags] = useState<string[]>([])
  const tags = useMemo(() => buildTagIndex(servers), [servers])
  const filteredServers = useMemo(() => {
    if (selectedTags.length === 0) return servers
    return servers.filter(server => {
      const serverTags = server.tags ?? []
      return selectedTags.some(tag => serverTags.includes(tag))
    })
  }, [servers, selectedTags])

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

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">Servers</span>
        <Button variant="ghost" size="xs" onClick={onRefresh}>
          Refresh
        </Button>
      </div>

      {tags.length > 0 && (
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

      <div className="space-y-1">
        {filteredServers.length === 0 ? (
          <div className="rounded-lg border border-dashed p-4 text-xs text-muted-foreground">
            No servers match the selected tags.
          </div>
        ) : (
          filteredServers.map((server, index) => {
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
                transition={{ duration: 0.2, delay: index * 0.03 }}
                onClick={() => onSelect(server.name)}
                className={cn(
                  'w-full rounded-lg border px-3 py-2 text-left transition-colors',
                  isActive
                    ? 'border-primary/60 bg-primary/10'
                    : 'border-border/60 hover:bg-muted/40',
                )}
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm font-medium">
                        {server.name}
                      </span>
                      {server.disabled && (
                        <Badge variant="warning" size="sm">
                          Disabled
                        </Badge>
                      )}
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
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
                  </div>
                  <ServerRuntimeIndicator specKey={server.specKey} />
                </div>
              </m.button>
            )
          })
        )}
      </div>
    </div>
  )
}
