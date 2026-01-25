// Input: Active clients hook, icons, class utility
// Output: Active clients indicator component for headers
// Position: Shared header indicator for active client registrations

import { MousePointer2Icon } from 'lucide-react'

import { useActiveClients } from '@/hooks/use-active-clients'
import { cn } from '@/lib/utils'

const maxVisibleClients = 2

export const ActiveClientsIndicator = ({ className }: { className?: string }) => {
  const { data: activeClients } = useActiveClients()
  const entries = activeClients ?? []
  const hasActive = entries.length > 0
  const visibleClients = entries.slice(0, maxVisibleClients)
  const extraCount = Math.max(entries.length - visibleClients.length, 0)
  const title = hasActive
    ? entries.map(entry => `${entry.client} (PID: ${entry.pid})`).join(', ')
    : 'No active clients'

  return (
    <div
      className={cn(
        'flex items-center gap-2 rounded-full border border-border/60 bg-muted/30 px-2.5 py-1 text-xs',
        className,
      )}
      title={title}
    >
      <span
        className={cn(
          'size-2 rounded-full',
          hasActive ? 'bg-success animate-pulse' : 'bg-muted',
        )}
      />
      <span className="text-muted-foreground">Active Clients</span>
      {hasActive ? (
        <div className="flex items-center gap-1">
          {visibleClients.map(entry => (
            <span
              key={`${entry.client}:${entry.pid}`}
              className="inline-flex items-center gap-1 rounded-full bg-background/80 px-2 py-0.5 font-mono text-[0.7rem] text-foreground shadow-xs"
            >
              <MousePointer2Icon className="size-3 text-info" />
              {entry.client}
              <span className="text-muted-foreground">(PID: {entry.pid})</span>
            </span>
          ))}
          {extraCount > 0 && (
            <span className="rounded-full bg-background/70 px-2 py-0.5 text-[0.7rem] text-muted-foreground">
              +{extraCount}
            </span>
          )}
        </div>
      ) : (
        <span className="text-muted-foreground">None</span>
      )}
    </div>
  )
}
