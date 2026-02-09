// Input: React Flow types, UI binding types
// Output: Type definitions for topology flow nodes and layout
// Position: Shared types for topology module

import type {
  ActiveClient,
  ServerDetail,
  ServerRuntimeStatus,
  ServerSummary,
} from '@bindings/mcpv/internal/ui/types'
import type { Node } from '@xyflow/react'

export type ClientNodeData = {
  name: string
  pid?: number
  tagCount: number
}

export type TagNodeData = {
  name: string
  serverCount: number
  clientCount: number
}

export type ServerNodeData = {
  name: string
  protocolVersion: string
  tags: string[]
}

export type InstanceNodeData = {
  id: string
  state: string
  busyCount: number
}

export type ClientFlowNode = Node<ClientNodeData, 'client'>
export type TagFlowNode = Node<TagNodeData, 'tag'>
export type ServerFlowNode = Node<ServerNodeData, 'server'>
export type InstanceFlowNode = Node<InstanceNodeData, 'instance'>
export type FlowNode = ClientFlowNode | TagFlowNode | ServerFlowNode | InstanceFlowNode

export type LayoutResult = {
  nodes: FlowNode[]
  edges: import('@xyflow/react').Edge[]
  tagCount: number
  serverCount: number
  clientCount: number
  instanceCount: number
}

export type BuildTopologyInput = {
  servers: ServerSummary[]
  serverDetails: ServerDetail[]
  activeClients: ActiveClient[]
  runtimeStatus: ServerRuntimeStatus[]
}
