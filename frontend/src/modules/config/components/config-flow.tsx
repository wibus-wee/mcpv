// Input: Config hooks for profiles/callers, React Flow, UI primitives
// Output: ConfigFlow component - topology graph of profiles, callers, and servers
// Position: Visualization panel inside config module tabs

import type {
  ActiveCaller,
  ProfileDetail,
  ProfileSummary,
} from '@bindings/mcpd/internal/ui'
import {
  Background,
  BackgroundVariant,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { LayersIcon, ServerIcon, Share2Icon, UsersIcon } from 'lucide-react'

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

import { useActiveCallers } from '@/hooks/use-active-callers'
import { useCallers, useProfileDetails, useProfiles } from '../hooks'

type CallerNodeData = {
  name: string
  profileName?: string
}

type ProfileNodeData = {
  name: string
  serverCount: number
  isDefault: boolean
  isMissing: boolean
}

type ServerNodeData = {
  name: string
  protocolVersion: string
  tags: string[]
}

type CallerFlowNode = Node<CallerNodeData, 'caller'>
type ProfileFlowNode = Node<ProfileNodeData, 'profile'>
type ServerFlowNode = Node<ServerNodeData, 'server'>
type FlowNode = CallerFlowNode | ProfileFlowNode | ServerFlowNode

type LayoutResult = {
  nodes: FlowNode[]
  edges: Edge[]
  profileCount: number
  serverCount: number
  callerCount: number
}

const handleBaseClass =
  'size-2.5 border border-background bg-foreground/50 shadow-sm'

const CallerNode = ({ data }: NodeProps<CallerFlowNode>) => {
  return (
    <div className="min-w-[180px] rounded-xl border border-info/30 bg-info/5 px-3 py-2 shadow-xs">
      <Handle
        type="source"
        position={Position.Right}
        className={cn(handleBaseClass, 'bg-info')}
      />
      <div className="flex items-center gap-1.5 text-[0.65rem] font-medium uppercase tracking-wide text-info-foreground/70">
        <UsersIcon className="size-3" />
        Caller
      </div>
      <div className="mt-1 font-mono text-sm text-foreground">{data.name}</div>
      {data.profileName && (
        <Badge variant="outline" size="sm" className="mt-2 font-mono">
          {data.profileName}
        </Badge>
      )}
    </div>
  )
}

const ProfileNode = ({ data }: NodeProps<ProfileFlowNode>) => {
  const label = data.isMissing ? 'Missing Profile' : 'Profile'
  const handleTone = data.isMissing ? 'bg-warning' : 'bg-primary'

  return (
    <div
      className={cn(
        'min-w-[190px] rounded-xl border px-3 py-2 shadow-xs',
        data.isMissing
          ? 'border-warning/40 bg-warning/5'
          : 'border-primary/20 bg-primary/5',
      )}
    >
      <Handle
        type="target"
        position={Position.Left}
        className={cn(handleBaseClass, handleTone)}
      />
      <Handle
        type="source"
        position={Position.Right}
        className={cn(handleBaseClass, handleTone)}
      />
      <div className="flex items-center gap-1.5 text-[0.65rem] font-medium uppercase tracking-wide text-muted-foreground">
        <LayersIcon className="size-3" />
        {label}
      </div>
      <div className="mt-1 text-sm font-semibold text-foreground">{data.name}</div>
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        <Badge variant="secondary" size="sm">
          {data.serverCount} Server{data.serverCount === 1 ? '' : 's'}
        </Badge>
        {data.isDefault && !data.isMissing && (
          <Badge variant="success" size="sm">
            Default
          </Badge>
        )}
        {data.isMissing && (
          <Badge variant="warning" size="sm">
            Missing
          </Badge>
        )}
      </div>
    </div>
  )
}

const ServerNode = ({ data }: NodeProps<ServerFlowNode>) => {
  const protocolLabel =
    data.protocolVersion === 'default'
      ? 'Protocol default'
      : data.protocolVersion === 'mixed'
        ? 'Protocol mixed'
        : `Protocol ${data.protocolVersion}`

  return (
    <div className="min-w-[200px] rounded-xl border border-border/70 bg-muted/30 px-3 py-2 shadow-xs">
      <Handle
        type="target"
        position={Position.Left}
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

const nodeTypes = {
  caller: CallerNode,
  profile: ProfileNode,
  server: ServerNode,
}

const layoutConfig = {
  columns: {
    caller: 0,
    profile: 280,
    server: 560,
  },
  nodeGap: 96,
  serverGap: 84,
  clusterGap: 140,
  minClusterHeight: 120,
}

type AggregatedServer = {
  key: string
  name: string
  protocolVersions: Set<string>
  strategies: Set<string>
  sessionTTLSeconds: number
  sessionTTLMixed: boolean
  maxConcurrent: number
  exposeToolsCount: number
  profileNames: Set<string>
}

type ServerTagInput = {
  strategy: string
  strategyMixed: boolean
  sessionTTLSeconds: number
  sessionTTLMixed: boolean
  maxConcurrent: number
  exposeToolsCount: number
  profileCount: number
}

const buildServerTags = ({
  strategy,
  strategyMixed,
  sessionTTLSeconds,
  sessionTTLMixed,
  maxConcurrent,
  exposeToolsCount,
  profileCount,
}: ServerTagInput) => {
  const tags: string[] = []

  const strategyLabel = {
    stateless: 'Stateless',
    stateful: 'Stateful',
    persistent: 'Persistent',
    singleton: 'Singleton',
  }[strategy] ?? strategy

  if (strategyMixed) {
    tags.push('Strategy Mixed')
  } else if (strategy !== 'stateless') {
    tags.push(strategyLabel)
  }

  if (strategy === 'stateful') {
    if (sessionTTLMixed) {
      tags.push('Session TTL Mixed')
    } else if (sessionTTLSeconds > 0) {
      tags.push(`Session TTL ${sessionTTLSeconds}s`)
    } else {
      tags.push('Session TTL Off')
    }
  }

  if (maxConcurrent > 0) {
    tags.push(`Max ${maxConcurrent}`)
  }

  if (exposeToolsCount > 0) {
    tags.push(`Tools ${exposeToolsCount}`)
  }

  if (profileCount > 1) {
    tags.push(`Profiles ${profileCount}`)
  }

  return tags
}

const buildTopology = (
  profiles: ProfileSummary[],
  profileDetails: ProfileDetail[],
  callers: Record<string, string>,
  activeCallers: ActiveCaller[],
): LayoutResult => {
  const detailsByName = new Map(
    profileDetails.map(profile => [profile.name, profile]),
  )
  const profileNameSet = new Set(profiles.map(profile => profile.name))
  const activeCallerSet = new Set(activeCallers.map(caller => caller.caller))
  const callersByProfile = new Map<string, string[]>()
  const serversByKey = new Map<string, AggregatedServer>()

  for (const [caller, profileName] of Object.entries(callers)) {
    const bucket = callersByProfile.get(profileName) ?? []
    bucket.push(caller)
    callersByProfile.set(profileName, bucket)
  }

  const missingProfiles = Array.from(callersByProfile.keys()).filter(
    profileName => !profileNameSet.has(profileName),
  )

  const orderedProfiles = [
    ...profiles.map(profile => ({
      name: profile.name,
      isDefault: profile.isDefault,
      isMissing: false,
    })),
    ...missingProfiles.sort().map(profileName => ({
      name: profileName,
      isDefault: false,
      isMissing: true,
    })),
  ]

  const nodes: FlowNode[] = []
  const edges: Edge[] = []
  const profilePositions = new Map<string, number>()

  let cursorY = 0

  for (const profile of orderedProfiles) {
    const profileDetail = detailsByName.get(profile.name)
    const servers = profileDetail?.servers ?? []
    const callerList = (callersByProfile.get(profile.name) ?? []).slice().sort()
    const clusterSize = Math.max(callerList.length, 1)
    const clusterHeight = (clusterSize - 1) * layoutConfig.nodeGap
    const baseline = Math.max(clusterHeight, layoutConfig.minClusterHeight)
    const profileY = cursorY + baseline / 2
    profilePositions.set(profile.name, profileY)

    nodes.push({
      id: `profile:${profile.name}`,
      type: 'profile',
      position: {
        x: layoutConfig.columns.profile,
        y: profileY,
      },
      data: {
        name: profile.name,
        serverCount: servers.length,
        isDefault: profile.isDefault,
        isMissing: profile.isMissing,
      },
    })

    for (const server of servers) {
      const serverKey = server.specKey || server.name
      const protocolLabel = server.protocolVersion || 'default'
      const existing = serversByKey.get(serverKey)

      if (existing) {
        existing.strategies.add(server.strategy)
        if (existing.sessionTTLSeconds !== server.sessionTTLSeconds) {
          existing.sessionTTLMixed = true
        }
        existing.maxConcurrent = Math.max(
          existing.maxConcurrent,
          server.maxConcurrent,
        )
        existing.exposeToolsCount = Math.max(
          existing.exposeToolsCount,
          server.exposeTools.length,
        )
        existing.profileNames.add(profile.name)
        existing.protocolVersions.add(protocolLabel)
      } else {
        serversByKey.set(serverKey, {
          key: serverKey,
          name: server.name,
          protocolVersions: new Set([protocolLabel]),
          strategies: new Set([server.strategy]),
          sessionTTLSeconds: server.sessionTTLSeconds,
          sessionTTLMixed: false,
          maxConcurrent: server.maxConcurrent,
          exposeToolsCount: server.exposeTools.length,
          profileNames: new Set([profile.name]),
        })
      }

      edges.push({
        id: `edge:profile:${profile.name}->server:${serverKey}`,
        source: `profile:${profile.name}`,
        target: `server:${serverKey}`,
        type: 'smoothstep',
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: 'var(--chart-2)',
        },
        style: {
          stroke: 'var(--chart-2)',
          strokeWidth: 1.5,
          strokeOpacity: 0.6,
        },
      })
    }

    const callerStartY =
      profileY - ((callerList.length - 1) * layoutConfig.nodeGap) / 2

    callerList.forEach((caller, index) => {
      const nodeId = `caller:${caller}`
      const isActive = activeCallerSet.has(caller)

      nodes.push({
        id: nodeId,
        type: 'caller',
        position: {
          x: layoutConfig.columns.caller,
          y: callerStartY + index * layoutConfig.nodeGap,
        },
        data: {
          name: caller,
          profileName: profile.name,
        },
      })

      edges.push({
        id: `edge:${nodeId}->profile:${profile.name}`,
        source: nodeId,
        target: `profile:${profile.name}`,
        type: 'smoothstep',
        animated: isActive,
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: isActive ? 'var(--chart-4)' : 'var(--info)',
        },
        style: {
          stroke: isActive ? 'var(--chart-4)' : 'var(--info)',
          strokeWidth: isActive ? 2 : 1.4,
          strokeOpacity: isActive ? 0.9 : 0.55,
          strokeDasharray: isActive ? '6 4' : '4 4',
        },
      })
    })

    cursorY += baseline + layoutConfig.clusterGap
  }

  const serverEntries = Array.from(serversByKey.values()).map(entry => {
    const profileYs = Array.from(entry.profileNames)
      .map(name => profilePositions.get(name))
      .filter((value): value is number => value !== undefined)
    const desiredY =
      profileYs.length > 0
        ? profileYs.reduce((sum, value) => sum + value, 0) / profileYs.length
        : 0

    return {
      entry,
      desiredY,
    }
  })

  serverEntries.sort((a, b) => a.desiredY - b.desiredY)

  let lastServerY = -Infinity

  for (const { entry, desiredY } of serverEntries) {
    const resolvedY = Math.max(desiredY, lastServerY + layoutConfig.serverGap)
    lastServerY = resolvedY
    const protocolVersion =
      entry.protocolVersions.size === 1
        ? Array.from(entry.protocolVersions)[0]
        : 'mixed'

    nodes.push({
      id: `server:${entry.key}`,
      type: 'server',
      position: {
        x: layoutConfig.columns.server,
        y: resolvedY,
      },
      data: {
        name: entry.name,
        protocolVersion,
        tags: buildServerTags({
          strategy:
            entry.strategies.size === 1
              ? Array.from(entry.strategies)[0]
              : 'mixed',
          strategyMixed: entry.strategies.size > 1,
          sessionTTLSeconds: entry.sessionTTLSeconds,
          sessionTTLMixed: entry.sessionTTLMixed,
          maxConcurrent: entry.maxConcurrent,
          exposeToolsCount: entry.exposeToolsCount,
          profileCount: entry.profileNames.size,
        }),
      },
    })
  }

  const serverCount = serversByKey.size

  return {
    nodes,
    edges,
    profileCount: orderedProfiles.length,
    serverCount,
    callerCount: Object.keys(callers).length,
  }
}

const FlowSkeleton = () => {
  return (
    <div className="h-full rounded-xl border bg-card/60 p-6">
      <div className="flex items-center gap-3">
        <Skeleton className="h-5 w-32" />
        <Skeleton className="h-5 w-20" />
        <Skeleton className="h-5 w-20" />
      </div>
      <div className="mt-6 grid grid-cols-3 gap-4">
        {Array.from({ length: 6 }).map((_, index) => (
          <Skeleton key={index} className="h-20 w-full" />
        ))}
      </div>
    </div>
  )
}

const FlowEmpty = () => {
  return (
    <div className="flex h-full items-center justify-center rounded-xl border bg-card/60">
      <Empty className="py-16">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Share2Icon className="size-4" />
          </EmptyMedia>
          <EmptyTitle className="text-sm">No topology data</EmptyTitle>
          <EmptyDescription className="text-xs">
            Add profiles, servers, or callers to render the configuration map.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    </div>
  )
}

export const ConfigFlow = () => {
  const { data: profiles, isLoading: profilesLoading } = useProfiles()
  const { data: callers, isLoading: callersLoading } = useCallers()
  const { data: activeCallers, isLoading: activeCallersLoading } =
    useActiveCallers()
  const { data: profileDetails, isLoading: detailsLoading } =
    useProfileDetails(profiles)

  const isLoading =
    profilesLoading || callersLoading || detailsLoading || activeCallersLoading
  const { nodes, edges, profileCount, serverCount, callerCount } =
    buildTopology(
      profiles ?? [],
      profileDetails ?? [],
      callers ?? {},
      activeCallers ?? [],
    )

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div className="flex items-center gap-2">
            <Share2Icon className="size-4 text-muted-foreground" />
            <span className="text-sm font-medium">Topology</span>
          </div>
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-4 w-16" />
          </div>
        </div>
        <div className="flex-1 p-4">
          <FlowSkeleton />
        </div>
      </div>
    )
  }

  const hasData = profileCount > 0 || serverCount > 0 || callerCount > 0

  if (!hasData) {
    return (
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div className="flex items-center gap-2">
            <Share2Icon className="size-4 text-muted-foreground" />
            <span className="text-sm font-medium">Topology</span>
          </div>
        </div>
        <div className="flex-1 p-4">
          <FlowEmpty />
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b px-4 py-3">
        <div className="flex items-center gap-2">
          <Share2Icon className="size-4 text-muted-foreground" />
          <span className="text-sm font-medium">Topology</span>
        </div>
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <span className="size-2 rounded-full bg-info" />
            Callers {callerCount}
          </span>
          <span className="inline-flex items-center gap-1">
            <span className="size-2 rounded-full bg-primary" />
            Profiles {profileCount}
          </span>
          <span className="inline-flex items-center gap-1">
            <span className="size-2 rounded-full" style={{ background: 'var(--chart-2)' }} />
            Servers {serverCount}
          </span>
        </div>
      </div>
      <div className="flex-1 p-4">
        <div className="h-full rounded-xl border bg-card/60">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            className="h-full w-full"
            fitView
            fitViewOptions={{ padding: 0.2 }}
            nodesDraggable={false}
            nodesConnectable={false}
            zoomOnScroll
            panOnScroll
            minZoom={0.4}
            maxZoom={1.2}
          >
            <Background
              variant={BackgroundVariant.Dots}
              gap={20}
              size={1.5}
              color="var(--border)"
            />
          </ReactFlow>
        </div>
      </div>
    </div>
  )
}
