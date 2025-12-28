// Input: Server data, runtime status, click handler
// Output: ServerCard component with glass effect
// Position: Base component for server display in tools grid

import { ServerIcon } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { ServerRuntimeIndicator } from '@/modules/config/components/server-runtime-status'
import { cn } from '@/lib/utils'

interface ServerCardProps {
  specKey: string
  serverName: string
  toolCount: number
  className?: string
  onClick: () => void
}

export function ServerCard({
  specKey,
  serverName,
  toolCount,
  className,
  onClick
}: ServerCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'relative overflow-hidden rounded-xl border border-border/50',
        'bg-background/80 backdrop-blur-sm',
        'transition-all duration-300',
        'hover:border-primary/50 hover:shadow-[0_0_20px_rgba(var(--primary-rgb),0.15)]',
        'cursor-pointer p-6 flex flex-col items-center gap-3 text-center',
        className
      )}
    >
      <ServerIcon className="size-8 text-muted-foreground" />
      <h3 className="text-lg font-semibold">{serverName}</h3>
      <ServerRuntimeIndicator specKey={specKey} />
      <Badge variant="secondary">
        {toolCount} {toolCount === 1 ? 'tool' : 'tools'}
      </Badge>
      {toolCount === 0 && (
        <p className="text-xs text-muted-foreground">No tools available</p>
      )}
    </button>
  )
}
