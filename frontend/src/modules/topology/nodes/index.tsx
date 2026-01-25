// Input: React Flow components, UI primitives, topology types
// Output: Node components and nodeTypes registry for React Flow
// Position: Node rendering layer for topology visualization

import { Handle, Position, type NodeProps } from '@xyflow/react'
import { MonitorIcon, ServerIcon, TagIcon } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { ServerStateBadge } from '@/components/custom/status-badge'

import type {
  ClientFlowNode,
  TagFlowNode,
  ServerFlowNode,
  InstanceFlowNode,
} from '../types'

export const handleBaseClass =
  'size-2.5 border border-background bg-foreground/50 shadow-sm'

export const ClientNode = ({ data, selected, isActive = false }: NodeProps<ClientFlowNode> & { isActive?: boolean }) => {
  return (
    <div
      className={cn(
        'min-w-[180px] rounded-xl border border-info/30 bg-info/5 px-3 py-2 shadow-xs transition-all',
        isActive && 'border-info/60',
        selected && 'border-2 border-info ring-2 ring-info/20 bg-info/20',
      )}
    >
      <Handle
        type="source"
        position={Position.Right}
        className={cn(handleBaseClass, 'bg-info')}
      />
      <div className={cn(
        'flex items-center gap-1.5 text-[0.65rem] font-medium uppercase tracking-wide',
        isActive ? 'text-info-foreground' : 'text-info-foreground/70',
      )}>
        <MonitorIcon className="size-3" />
        Client
      </div>
      <div className="mt-1 font-mono text-sm text-foreground">{data.name}</div>
      {data.pid !== undefined && (
        <div className="mt-1 text-xs text-muted-foreground font-mono">PID: {data.pid}</div>
      )}
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        <Badge variant="secondary" size="sm">
          {data.tagCount} Tag{data.tagCount === 1 ? '' : 's'}
        </Badge>
      </div>
    </div>
  )
}

export const TagNode = ({ data, selected }: NodeProps<TagFlowNode>) => {
  return (
    <div
      className={cn(
        'min-w-[190px] rounded-xl border px-3 py-2 shadow-xs transition-all',
        'border-primary/20 bg-primary/5',
        selected && 'border-2 border-primary ring-2 ring-primary/20 bg-primary/20',
      )}
    >
      <Handle
        type="target"
        position={Position.Left}
        className={cn(handleBaseClass, 'bg-primary')}
      />
      <Handle
        type="source"
        position={Position.Right}
        className={cn(handleBaseClass, 'bg-primary')}
      />
      <div className="flex items-center gap-1.5 text-[0.65rem] font-medium uppercase tracking-wide text-muted-foreground">
        <TagIcon className="size-3" />
        Tag
      </div>
      <div className="mt-1 text-sm font-semibold text-foreground">{data.name}</div>
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        <Badge variant="secondary" size="sm">
          {data.serverCount} Server{data.serverCount === 1 ? '' : 's'}
        </Badge>
        <Badge variant="outline" size="sm">
          {data.clientCount} Client{data.clientCount === 1 ? '' : 's'}
        </Badge>
      </div>
    </div>
  )
}

export const ServerNode = ({ data }: NodeProps<ServerFlowNode>) => {
  const protocolLabel =
    data.protocolVersion === 'default'
      ? 'Protocol default'
      : data.protocolVersion === 'mixed'
        ? 'Protocol mixed'
        : `Protocol ${data.protocolVersion}`

  return (
    <div className={cn('min-w-50 rounded-xl border border-border/70 bg-muted/30 px-3 py-2 shadow-xs')}>
      <Handle
        type="target"
        position={Position.Left}
        className={cn(handleBaseClass, 'bg-muted-foreground')}
      />
      <Handle
        type="source"
        position={Position.Right}
        className={cn(handleBaseClass, 'bg-muted-foreground')}
      />
      <div className="flex items-center gap-1.5 text-[0.65rem] font-medium uppercase tracking-wide text-muted-foreground">
        <ServerIcon className="size-3" />
        Server
      </div>
      <div className="mt-1 font-mono text-sm text-foreground">{data.name}</div>
      <div className="mt-1 text-xs text-muted-foreground">{protocolLabel}</div>
      {data.tags.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {data.tags.map(tag => (
            <Badge key={tag} variant="outline" size="sm">
              {tag}
            </Badge>
          ))}
        </div>
      )}
    </div>
  )
}

export const InstanceNode = ({ data, selected }: NodeProps<InstanceFlowNode>) => {
  const truncatedId = data.id.slice(-8)

  return (
    <div
      className={cn(
        'min-w-[140px] rounded-xl border border-border/50 bg-muted/20 px-2 py-1.5 shadow-xs transition-all',
        selected && 'border-2 border-primary ring-2 ring-primary/20 bg-muted/30',
      )}
    >
      <Handle
        type="target"
        position={Position.Left}
        className={cn(handleBaseClass, 'bg-border')}
      />
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-1.5 text-[0.65rem] font-medium uppercase tracking-wide text-muted-foreground">
          Instance
        </div>
        <ServerStateBadge state={data.state} size="sm" />
      </div>
      <div className="mt-1 font-mono text-xs text-foreground">{truncatedId}</div>
      {data.busyCount > 0 && (
        <div className="mt-1 text-[0.65rem] text-muted-foreground">Busy: {data.busyCount}</div>
      )}
    </div>
  )
}

export const nodeTypes = {
  client: ClientNode,
  tag: TagNode,
  server: ServerNode,
  instance: InstanceNode,
}
