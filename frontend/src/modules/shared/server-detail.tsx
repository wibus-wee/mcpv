import type { ServerDetail } from '@bindings/mcpv/internal/ui/types'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

export function buildCommandSummary(server: ServerDetail) {
  if (server.transport === 'streamable_http') {
    return server.http?.endpoint ?? '--'
  }
  return server.cmd.join(' ')
}

interface DetailRowProps {
  label: string
  value?: ReactNode
  mono?: boolean
}

export function DetailRow({ label, value, mono }: DetailRowProps) {
  return (
    <div className="flex items-start justify-between gap-4 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <div className={cn('text-right', mono && 'font-mono text-xs')}>{value}</div>
    </div>
  )
}
