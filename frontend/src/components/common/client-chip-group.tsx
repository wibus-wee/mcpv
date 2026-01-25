// Input: active clients, icons, class utility
// Output: client chip group for contextual client display
// Position: shared UI component for client lists

import { MousePointer2Icon } from 'lucide-react'

import type { ActiveClient } from '@bindings/mcpd/internal/ui'

import { cn } from '@/lib/utils'

interface ClientChipGroupProps {
  clients: ActiveClient[]
  maxVisible?: number
  showPid?: boolean
  emptyText?: string
  className?: string
}

export const ClientChipGroup = ({
  clients,
  maxVisible = 2,
  showPid = false,
  emptyText,
  className,
}: ClientChipGroupProps) => {
  const visibleClients = clients.slice(0, maxVisible)
  const extraCount = Math.max(clients.length - visibleClients.length, 0)

  if (clients.length === 0) {
    if (!emptyText) {
      return null
    }
    return <span className={cn('text-[0.7rem] text-muted-foreground', className)}>{emptyText}</span>
  }

  return (
    <div className={cn('flex flex-wrap items-center gap-1', className)}>
      {visibleClients.map(entry => (
        <span
          key={`${entry.client}:${entry.pid}`}
          className="inline-flex items-center gap-1 rounded-full bg-background/80 px-2 py-0.5 font-mono text-[0.7rem] text-foreground shadow-xs"
          title={showPid ? `${entry.client} (PID: ${entry.pid})` : entry.client}
        >
          <MousePointer2Icon className="size-3 text-info" />
          {entry.client}
          {showPid && (
            <span className="text-muted-foreground">(PID: {entry.pid})</span>
          )}
        </span>
      ))}
      {extraCount > 0 && (
        <span className="rounded-full bg-background/70 px-2 py-0.5 text-[0.7rem] text-muted-foreground">
          +{extraCount}
        </span>
      )}
    </div>
  )
}
