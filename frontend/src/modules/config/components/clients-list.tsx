// Input: ActiveClient array
// Output: ClientsList component - active client view
// Position: Clients tab content

import type { ActiveClient } from '@bindings/mcpd/internal/ui'
import { m } from 'motion/react'
import { MonitorIcon, TagIcon } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { formatRelativeTime } from '@/lib/time'

interface ClientsListProps {
  clients: ActiveClient[]
  isLoading: boolean
}

function ClientsListSkeleton() {
  return (
    <div className="space-y-2">
      {Array.from({ length: 4 }).map((_, i) => (
        <Skeleton key={i} className="h-14 w-full rounded-md" />
      ))}
    </div>
  )
}

function ClientsListEmpty() {
  return (
    <Empty className="py-16">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <MonitorIcon className="size-4" />
        </EmptyMedia>
        <EmptyTitle className="text-sm">No active clients</EmptyTitle>
        <EmptyDescription className="text-xs">
          Clients appear here once they connect with a tag or server name.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

function ClientAvatar({ name }: { name: string }) {
  return (
    <div className="flex size-8 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
      {name.slice(0, 1).toUpperCase()}
    </div>
  )
}

export function ClientsList({ clients, isLoading }: ClientsListProps) {
  if (isLoading) {
    return <ClientsListSkeleton />
  }

  if (clients.length === 0) {
    return <ClientsListEmpty />
  }

  return (
    <div className="space-y-2">
      {clients.map((client, index) => (
        <m.div
          key={`${client.client}-${client.pid}`}
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2, delay: index * 0.03 }}
          className="rounded-lg border bg-background p-3 shadow-xs"
        >
          <div className="flex items-start gap-3">
            <ClientAvatar name={client.client} />
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">
                    {client.client}
                  </span>
                  <Badge variant="outline" size="sm" className="font-mono text-[10px]">
                    PID {client.pid}
                  </Badge>
                </div>
                <span className="text-xs text-muted-foreground">
                  {client.lastHeartbeat ? formatRelativeTime(client.lastHeartbeat) : 'Just now'}
                </span>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <div className="flex items-center gap-1 text-xs text-muted-foreground">
                  <TagIcon className="size-3" />
                  Tags
                </div>
                <div className={cn('flex flex-wrap gap-1.5', client.tags?.length ? '' : 'text-muted-foreground')}>
                  {(client.tags ?? []).length > 0 ? (
                    client.tags.map(tag => (
                      <Badge key={`${client.client}-${tag}`} variant="secondary" size="sm">
                        {tag}
                      </Badge>
                    ))
                  ) : (
                    <span className="text-xs">No tags</span>
                  )}
                </div>
              </div>
            </div>
          </div>
        </m.div>
      ))}
    </div>
  )
}
