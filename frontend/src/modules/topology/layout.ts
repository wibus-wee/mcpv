// Input: Server/client data from hooks, binding types, elkjs for auto-layout
// Output: buildTopology function using elkjs for hierarchical graph positioning
// Position: Layout computation layer for topology visualization

import type {
  ActiveClient,
  ServerDetail,
  ServerRuntimeStatus,
  ServerSummary,
} from '@bindings/mcpv/internal/ui/types'
import type { Edge } from '@xyflow/react'
import { MarkerType } from '@xyflow/react'
import ELK from 'elkjs/lib/elk.bundled.js'

import type { FlowNode, LayoutResult } from './types'

// Node dimensions for elk layout calculation
const nodeDimensions = {
  client: { width: 220, height: 100 },
  tag: { width: 200, height: 90 },
  server: { width: 220, height: 110 },
  instance: { width: 160, height: 70 },
}

const elk = new ELK()

const scaleSpacing = (base: number, scale: number, min: number) =>
  Math.max(min, Math.round(base * scale)).toString()

const getLayoutOptions = (scale: number) => ({
  'elk.algorithm': 'layered',
  'elk.direction': 'RIGHT',
  'elk.layered.considerModelOrder': 'true',
  'elk.spacing.nodeNode': scaleSpacing(48, scale, 28),
  'elk.layered.spacing.nodeNodeBetweenLayers': scaleSpacing(140, scale, 80),
  'elk.spacing.edgeNode': scaleSpacing(26, scale, 16),
  'elk.padding': '[top=40,left=40,bottom=40,right=40]',
})

type ServerEntry = {
  specKey: string
  name: string
  protocolVersion: string
  tags: string[]
}

const normalizeTags = (tags: string[] | undefined) => {
  if (!tags || tags.length === 0) return ['untagged']
  return tags
}

export const buildTopology = async (
  servers: ServerSummary[],
  serverDetails: ServerDetail[],
  activeClients: ActiveClient[],
  runtimeStatus: ServerRuntimeStatus[],
): Promise<LayoutResult> => {
  const serverEntries = new Map<string, ServerEntry>()
  const serverNameIndex = new Map<string, ServerEntry>()

  servers.forEach((summary) => {
    if (!summary.specKey) return
    const entry = {
      specKey: summary.specKey,
      name: summary.name,
      protocolVersion: 'default',
      tags: normalizeTags(summary.tags),
    }
    serverEntries.set(summary.specKey, entry)
    serverNameIndex.set(summary.name, entry)
  })

  serverDetails.forEach((detail) => {
    const existing = serverEntries.get(detail.specKey)
    const entry = {
      specKey: detail.specKey,
      name: detail.name,
      protocolVersion: detail.protocolVersion || existing?.protocolVersion || 'default',
      tags: normalizeTags(detail.tags ?? existing?.tags),
    }
    serverEntries.set(detail.specKey, entry)
    serverNameIndex.set(detail.name, entry)
  })

  runtimeStatus.forEach((status) => {
    if (!status.specKey) return
    if (!serverEntries.has(status.specKey)) {
      const entry = {
        specKey: status.specKey,
        name: status.serverName || status.specKey,
        protocolVersion: 'default',
        tags: ['untagged'],
      }
      serverEntries.set(status.specKey, entry)
      serverNameIndex.set(entry.name, entry)
    }
  })

  const tagSet = new Set(
    Array.from(serverEntries.values()).flatMap(entry => entry.tags),
  )
  activeClients.forEach((client) => {
    client.tags?.forEach(tag => tagSet.add(tag))
  })

  const allTags = Array.from(tagSet).sort((a, b) => a.localeCompare(b))

  const resolveClientTags = (client: ActiveClient) => {
    const serverName = client.server?.trim()
    if (serverName) {
      const entry = serverNameIndex.get(serverName)
      const tags = entry?.tags.filter(tag => tag !== 'untagged') ?? []
      return {
        mode: 'server' as const,
        tags,
        serverSpecKey: entry?.specKey ?? '',
      }
    }

    const tags = client.tags && client.tags.length > 0 ? client.tags : allTags
    return {
      mode: 'tag' as const,
      tags,
      serverSpecKey: '',
    }
  }

  // Build ELK graph structure
  const elkNodes: any[] = []
  const elkEdges: any[] = []
  const nodeDataMap = new Map<string, FlowNode['data'] & { type: FlowNode['type'] }>()
  const edgeList: Edge[] = []

  const totalNodeEstimate = activeClients.length
    + allTags.length
    + serverEntries.size
    + runtimeStatus.reduce((sum, status) => sum + status.instances.length, 0)

  const densityScale
    = totalNodeEstimate > 140
      ? 0.65
      : totalNodeEstimate > 90
        ? 0.8
        : 1

  // Add tag nodes
  allTags.forEach((tag) => {
    const serverCount = Array.from(serverEntries.values()).filter(entry =>
      entry.tags.includes(tag),
    ).length

    const clientCount = activeClients.filter((client) => {
      const resolved = resolveClientTags(client)
      if (resolved.mode !== 'tag') return false
      return resolved.tags.includes(tag)
    }).length

    const nodeId = `tag:${tag}`
    elkNodes.push({
      id: nodeId,
      width: nodeDimensions.tag.width,
      height: nodeDimensions.tag.height,
    })
    nodeDataMap.set(nodeId, {
      type: 'tag',
      name: tag,
      serverCount,
      clientCount,
    })
  })

  // Add client nodes and edges
  const orderedClients = activeClients
    .map((client) => {
      const resolved = resolveClientTags(client)
      const sortKey = resolved.mode === 'server'
        ? `server:${resolved.serverSpecKey}`
        : `tag:${resolved.tags.slice().sort((a, b) => a.localeCompare(b)).join(',')}`
      return { client, resolved, sortKey }
    })
    .sort((a, b) => {
      const keyCompare = a.sortKey.localeCompare(b.sortKey)
      if (keyCompare !== 0) return keyCompare
      const nameCompare = a.client.client.localeCompare(b.client.client)
      if (nameCompare !== 0) return nameCompare
      return (a.client.pid ?? 0) - (b.client.pid ?? 0)
    })

  orderedClients.forEach(({ client, resolved }) => {
    const clientId = `client:${client.client}:${client.pid}`

    elkNodes.push({
      id: clientId,
      width: nodeDimensions.client.width,
      height: nodeDimensions.client.height,
    })
    nodeDataMap.set(clientId, {
      type: 'client',
      name: client.client,
      pid: client.pid,
      tagCount: resolved.tags.length,
    })

    if (resolved.mode === 'server' && resolved.serverSpecKey) {
      // Direct client -> server edge
      const serverId = `server:${resolved.serverSpecKey}`
      elkEdges.push({
        id: `edge:${clientId}->server:${resolved.serverSpecKey}`,
        sources: [clientId],
        targets: [serverId],
      })
      edgeList.push({
        id: `edge:${clientId}->server:${resolved.serverSpecKey}`,
        source: clientId,
        target: serverId,
        type: 'smoothstep',
        animated: true,
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: 'var(--chart-3)',
        },
        style: {
          stroke: 'var(--chart-3)',
          strokeWidth: 1.6,
          strokeOpacity: 0.7,
        },
      })
    }
    else {
      // Client -> tag edges
      resolved.tags.forEach((tag) => {
        const tagId = `tag:${tag}`
        elkEdges.push({
          id: `edge:${clientId}->tag:${tag}`,
          sources: [clientId],
          targets: [tagId],
        })
        edgeList.push({
          id: `edge:${clientId}->tag:${tag}`,
          source: clientId,
          target: tagId,
          type: 'smoothstep',
          animated: true,
          markerEnd: {
            type: MarkerType.ArrowClosed,
            color: 'var(--chart-4)',
          },
          style: {
            stroke: 'var(--chart-4)',
            strokeWidth: 1.5,
            strokeOpacity: 0.6,
            strokeDasharray: '6 4',
          },
        })
      })
    }
  })

  // Add server nodes and edges
  Array.from(serverEntries.values())
    .sort((a, b) => a.name.localeCompare(b.name))
    .forEach((entry) => {
      const serverId = `server:${entry.specKey}`

      elkNodes.push({
        id: serverId,
        width: nodeDimensions.server.width,
        height: nodeDimensions.server.height,
      })
      nodeDataMap.set(serverId, {
        type: 'server',
        name: entry.name,
        protocolVersion: entry.protocolVersion || 'default',
        tags: entry.tags.filter(tag => tag !== 'untagged'),
      })

      // Tag -> server edges
      entry.tags.forEach((tag) => {
        const tagId = `tag:${tag}`
        elkEdges.push({
          id: `edge:tag:${tag}->server:${entry.specKey}`,
          sources: [tagId],
          targets: [serverId],
        })
        edgeList.push({
          id: `edge:tag:${tag}->server:${entry.specKey}`,
          source: tagId,
          target: serverId,
          type: 'smoothstep',
          markerEnd: {
            type: MarkerType.ArrowClosed,
            color: 'var(--chart-2)',
          },
          style: {
            stroke: 'var(--chart-2)',
            strokeWidth: 1.4,
            strokeOpacity: 0.6,
          },
        })
      })
    })

  // Add instance nodes and edges
  let instanceCount = 0
  const runtimeStatusBySpecKey = new Map(
    runtimeStatus.map(status => [status.specKey, status]),
  )

  const orderedRuntimeStatus = Array.from(runtimeStatusBySpecKey.entries())
    .sort(([a], [b]) => a.localeCompare(b))

  for (const [serverKey, serverStatus] of orderedRuntimeStatus) {
    const serverId = `server:${serverKey}`
    const serverExists = elkNodes.some(n => n.id === serverId)
    if (!serverExists) continue

    const { instances } = serverStatus
    instances
      .slice()
      .sort((a, b) => a.id.localeCompare(b.id))
      .forEach((instance) => {
        instanceCount++
        const nodeId = `instance:${serverKey}:${instance.id}`

        elkNodes.push({
          id: nodeId,
          width: nodeDimensions.instance.width,
          height: nodeDimensions.instance.height,
        })
        nodeDataMap.set(nodeId, {
          type: 'instance',
          id: instance.id,
          state: instance.state,
          busyCount: instance.busyCount,
        })

        elkEdges.push({
          id: `edge:server:${serverKey}->instance:${instance.id}`,
          sources: [serverId],
          targets: [nodeId],
        })
        edgeList.push({
          id: `edge:server:${serverKey}->instance:${instance.id}`,
          source: serverId,
          target: nodeId,
          type: 'smoothstep',
          markerEnd: {
            type: MarkerType.ArrowClosed,
            color: 'var(--border)',
          },
          style: {
            stroke: 'var(--border)',
            strokeWidth: 1,
            strokeOpacity: 0.5,
          },
        })
      })
  }

  // Run ELK layout
  const graph = {
    id: 'root',
    layoutOptions: getLayoutOptions(densityScale),
    children: elkNodes,
    edges: elkEdges,
  }

  const layoutedGraph = await elk.layout(graph)

  // Build final nodes with computed positions
  const nodes: FlowNode[] = []
  layoutedGraph.children?.forEach((node) => {
    const data = nodeDataMap.get(node.id)
    if (!data) return

    const { type, ...rest } = data

    nodes.push({
      id: node.id,
      type: type as FlowNode['type'],
      position: { x: node.x ?? 0, y: node.y ?? 0 },
      data: rest,
    } as FlowNode)
  })

  return {
    nodes,
    edges: edgeList,
    tagCount: allTags.length,
    serverCount: serverEntries.size,
    clientCount: activeClients.length,
    instanceCount,
  }
}
