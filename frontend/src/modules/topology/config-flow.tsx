// Input: Config hooks for servers/clients, React Flow, topology submodules
// Output: ConfigFlow component - topology graph of clients, tags, and servers
// Position: Visualization panel inside config module tabs

import {
  Background,
  BackgroundVariant,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type Edge,
} from '@xyflow/react'
import { useCallback } from 'react'
import '@xyflow/react/dist/style.css'
import { Share2Icon } from 'lucide-react'

import { Skeleton } from '@/components/ui/skeleton'
import { useActiveClients } from '@/hooks/use-active-clients'
import { useRuntimeStatus, useServerDetails, useServers } from '../config/hooks'

import type { FlowNode } from './types'
import { nodeTypes } from './nodes'
import { buildTopology } from './layout'
import { FlowEmpty, FlowSkeleton } from './components'

const ConfigFlowInner = ({
  nodes,
  edges,
}: {
  nodes: FlowNode[]
  edges: Edge[]
}) => {
  const { fitView, getNodes } = useReactFlow()

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: FlowNode) => {
      const allNodes = getNodes() as FlowNode[]
      const relatedIds = new Set<string>([node.id])

      edges.forEach(edge => {
        if (edge.source === node.id) {
          relatedIds.add(edge.target)
        }
        if (edge.target === node.id) {
          relatedIds.add(edge.source)
        }
      })

      const relatedNodes = allNodes.filter(n => relatedIds.has(n.id))

      if (relatedNodes.length > 0) {
        fitView({
          nodes: relatedNodes,
          padding: 0.6,
          duration: 400,
          minZoom: 0.4,
          maxZoom: 1,
        })
      }
    },
    [edges, fitView, getNodes],
  )

  return (
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
      onNodeClick={onNodeClick}
    >
      <Background
        variant={BackgroundVariant.Dots}
        gap={20}
        size={1.5}
        color="var(--border)"
      />
    </ReactFlow>
  )
}

export const ConfigFlow = () => {
  const { data: servers, isLoading: serversLoading } = useServers()
  const { data: activeClients, isLoading: activeClientsLoading } =
    useActiveClients()
  const { data: serverDetails, isLoading: detailsLoading } =
    useServerDetails(servers)
  const { data: runtimeStatus, isLoading: runtimeStatusLoading } = useRuntimeStatus()

  const isLoading =
    serversLoading || detailsLoading || activeClientsLoading || runtimeStatusLoading
  const { nodes, edges, tagCount, serverCount, clientCount, instanceCount } =
    buildTopology(
      servers ?? [],
      serverDetails ?? [],
      activeClients ?? [],
      runtimeStatus ?? [],
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

  const hasData = tagCount > 0 || serverCount > 0 || clientCount > 0 || instanceCount > 0

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
      <div className="flex-1">
        <div className="h-full rounded-xl">
          <ReactFlowProvider>
            <ConfigFlowInner nodes={nodes} edges={edges} />
          </ReactFlowProvider>
        </div>
      </div>
    </div>
  )
}
