// Input: ServerSummary array, selection state, runtime status
// Output: Compact server list for master-detail layout left pane
// Position: Left panel in servers master-detail layout

import type { ServerSummary } from '@bindings/mcpv/internal/ui/types'
import { ServerIcon, WrenchIcon } from 'lucide-react'
import { AnimatePresence, m } from 'motion/react'
import { memo } from 'react'

import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

import type { ServerTab } from '../constants'
import { ServerRuntimeIndicator } from './server-runtime-status'

interface ServersMasterListProps {
  servers: ServerSummary[]
  selectedServer: string | null
  onSelectServer: (name: string) => void
  onSelectServerTab: (name: string, tab: ServerTab) => void
  isLoading: boolean
  toolCountMap: Map<string, number>
}

function ListSkeleton() {
  return (
    <div className="p-3 space-y-2">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-14 w-full rounded-lg" />
      ))}
    </div>
  )
}

function ListEmpty() {
  return (
    <div className="flex flex-col items-center justify-center h-full p-6 text-center">
      <ServerIcon className="size-8 text-muted-foreground/40 mb-3" />
      <p className="text-sm text-muted-foreground">No servers found</p>
    </div>
  )
}

export const ServersMasterList = memo(function ServersMasterList({
  servers,
  selectedServer,
  onSelectServer,
  onSelectServerTab,
  isLoading,
  toolCountMap,
}: ServersMasterListProps) {
  if (isLoading) {
    return <ListSkeleton />
  }

  if (servers.length === 0) {
    return <ListEmpty />
  }

  return (
    <ScrollArea className="h-full">
      <div className="p-2 space-y-1">
        <AnimatePresence initial={false} mode="popLayout">
          {servers.map((server, index) => {
            const isActive = server.name === selectedServer
            const toolCount = toolCountMap.get(server.specKey) ?? 0

            return (
              <m.button
                key={server.name}
                type="button"
                layout
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.95 }}
                transition={Spring.snappy(0.2, index * 0.015)}
                onClick={() => onSelectServer(server.name)}
                className={cn(
                  'w-full rounded-lg px-3 py-2.5 text-left transition-all duration-150',
                  'border border-transparent',
                  'focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1',
                  isActive
                    ? 'bg-accent border-accent-foreground/15 shadow-sm'
                    : 'hover:bg-muted/60',
                )}
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className={cn(
                        'text-sm font-medium truncate',
                        isActive && 'text-accent-foreground',
                      )}
                      >
                        {server.name}
                      </span>
                      {server.disabled && (
                        <Badge variant="warning" size="sm">Off</Badge>
                      )}
                    </div>
                    <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation()
                          onSelectServerTab(server.name, 'tools')
                        }}
                        className="flex items-center gap-1 hover:text-foreground transition-colors"
                      >
                        <WrenchIcon className="size-3" />
                        {toolCount}
                      </button>
                      <span className="uppercase text-[10px] font-mono opacity-60">
                        {server.transport}
                      </span>
                    </div>
                  </div>
                  <ServerRuntimeIndicator specKey={server.specKey} />
                </div>
              </m.button>
            )
          })}
        </AnimatePresence>
      </div>
    </ScrollArea>
  )
})
