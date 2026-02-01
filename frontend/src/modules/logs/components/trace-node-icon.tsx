// Node type indicator for trace visualization
// Displays: M (Middleware/Route), F (Function/Call), etc.

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

interface TraceNodeIconProps {
  type: 'route' | 'instance' | 'call' | 'receive' | 'response'
  status?: 'success' | 'error' | 'pending'
  tooltip?: string
}

const nodeConfig = {
  receive: { label: 'R', name: 'Received', className: 'bg-muted' },
  route: { label: 'M', name: 'Middleware', className: 'bg-muted' },
  instance: { label: 'I', name: 'Instance', className: 'bg-muted' },
  call: { label: 'F', name: 'Function', className: 'bg-muted' },
  response: { label: '✓', name: 'Response', className: 'bg-muted' },
} as const

const statusClassName = {
  success: 'border-success/50 text-success-foreground',
  error: 'border-destructive/50 text-destructive-foreground',
  pending: 'border-warning/50 text-warning-foreground',
}

export function TraceNodeIcon({ type, status, tooltip }: TraceNodeIconProps) {
  const config = nodeConfig[type]
  const content = (
    <span
      className={cn(
        'inline-flex size-5 items-center justify-center rounded border text-[10px] font-medium',
        config.className,
        status && statusClassName[status],
      )}
    >
      {config.label}
    </span>
  )

  if (tooltip) {
    return (
      <Tooltip>
        <TooltipTrigger render={content} />
        <TooltipContent>
          <p className="font-medium">{config.name}</p>
          <p className="text-muted-foreground">{tooltip}</p>
        </TooltipContent>
      </Tooltip>
    )
  }

  return content
}

interface TraceNodeSequenceProps {
  nodes: Array<{
    type: TraceNodeIconProps['type']
    status?: TraceNodeIconProps['status']
    tooltip?: string
  }>
}

export function TraceNodeSequence({ nodes }: TraceNodeSequenceProps) {
  return (
    <span className="inline-flex items-center gap-0.5">
      {nodes.map(node => (
        <TraceNodeIcon
          key={`${node.type}-${node.tooltip ?? 'default'}`}
          type={node.type}
          status={node.status}
          tooltip={node.tooltip}
        />
      ))}
    </span>
  )
}
