// Input: Server/client data from hooks, binding types
// Output: buildTopology function and layoutConfig for graph positioning
// Position: Layout computation layer for topology visualization

import type {
  ActiveClient,
  ServerDetail,
  ServerRuntimeStatus,
  ServerSummary,
} from '@bindings/mcpd/internal/ui'
import type { Edge } from '@xyflow/react'
import { MarkerType } from '@xyflow/react'

import type { FlowNode, LayoutResult } from './types'

export const layoutConfig = {
  columns: {
    client: 0,
    tag: 260,
    server: 520,
    instance: 780,
  },
  nodeGap: 96,
  tagGap: 140,
  serverGap: 96,
  instanceGap: 60,
}

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

export const buildTopology = (
  servers: ServerSummary[],
  serverDetails: ServerDetail[],
  activeClients: ActiveClient[],
  runtimeStatus: ServerRuntimeStatus[],
): LayoutResult => {
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

  const nodes: FlowNode[] = []
  const edges: Edge[] = []
  const tagPositions = new Map<string, number>()

  let tagCursor = 0
  allTags.forEach((tag) => {
    const y = tagCursor
    tagPositions.set(tag, y)
    tagCursor += layoutConfig.tagGap

    const serverCount = Array.from(serverEntries.values()).filter(entry =>
      entry.tags.includes(tag),
    ).length

    const clientCount = activeClients.filter((client) => {
      const resolved = resolveClientTags(client)
      if (resolved.mode !== 'tag') {
        return false
      }
      return resolved.tags.includes(tag)
    }).length

    nodes.push({
      id: `tag:${tag}`,
      type: 'tag',
      position: {
        x: layoutConfig.columns.tag,
        y,
      },
      data: {
        name: tag,
        serverCount,
        clientCount,
      },
    })
  })

  const clientEntries = activeClients.map((client) => {
    const resolved = resolveClientTags(client)
    const { tags } = resolved
    const tagYs = tags
      .map(tag => tagPositions.get(tag))
      .filter((value): value is number => value !== undefined)
    const desiredY = tagYs.length > 0
      ? tagYs.reduce((sum, value) => sum + value, 0) / tagYs.length
      : 0
    return {
      client,
      tags,
      desiredY,
      mode: resolved.mode,
      serverSpecKey: resolved.serverSpecKey,
    }
  })

  clientEntries.sort((a, b) => a.desiredY - b.desiredY)

  let lastClientY = -Infinity
  clientEntries.forEach(({ client, tags, desiredY, mode, serverSpecKey }) => {
    const resolvedY = Math.max(desiredY, lastClientY + layoutConfig.nodeGap)
    lastClientY = resolvedY

    const clientId = `client:${client.client}:${client.pid}`
    nodes.push({
      id: clientId,
      type: 'client',
      position: {
        x: layoutConfig.columns.client,
        y: resolvedY,
      },
      data: {
        name: client.client,
        pid: client.pid,
        tagCount: tags.length,
      },
    })

    if (mode === 'server' && serverSpecKey) {
      edges.push({
        id: `edge:${clientId}->server:${serverSpecKey}`,
        source: clientId,
        target: `server:${serverSpecKey}`,
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
      return
    }

    tags.forEach((tag) => {
      edges.push({
        id: `edge:${clientId}->tag:${tag}`,
        source: clientId,
        target: `tag:${tag}`,
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
  })

  const serverEntriesArray = Array.from(serverEntries.values()).map((entry) => {
    const tagYs = entry.tags
      .map(tag => tagPositions.get(tag))
      .filter((value): value is number => value !== undefined)
    const desiredY = tagYs.length > 0
      ? tagYs.reduce((sum, value) => sum + value, 0) / tagYs.length
      : 0
    return { entry, desiredY }
  })

  serverEntriesArray.sort((a, b) => a.desiredY - b.desiredY)

  const serverPositions = new Map<string, number>()
  let lastServerY = -Infinity

  serverEntriesArray.forEach(({ entry, desiredY }) => {
    const resolvedY = Math.max(desiredY, lastServerY + layoutConfig.serverGap)
    lastServerY = resolvedY

    nodes.push({
      id: `server:${entry.specKey}`,
      type: 'server',
      position: {
        x: layoutConfig.columns.server,
        y: resolvedY,
      },
      data: {
        name: entry.name,
        protocolVersion: entry.protocolVersion || 'default',
        tags: entry.tags.filter(tag => tag !== 'untagged'),
      },
    })
    serverPositions.set(entry.specKey, resolvedY)

    entry.tags.forEach((tag) => {
      edges.push({
        id: `edge:tag:${tag}->server:${entry.specKey}`,
        source: `tag:${tag}`,
        target: `server:${entry.specKey}`,
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

  let instanceCount = 0
  const runtimeStatusBySpecKey = new Map(
    runtimeStatus.map(status => [status.specKey, status]),
  )

  for (const [serverKey, serverStatus] of runtimeStatusBySpecKey.entries()) {
    const serverY = serverPositions.get(serverKey)
    if (serverY === undefined) continue

    const { instances } = serverStatus
    if (instances.length === 0) continue

    const instanceStartY
      = serverY - ((instances.length - 1) * layoutConfig.instanceGap) / 2

    instances.forEach((instance, index) => {
      instanceCount++
      const nodeId = `instance:${serverKey}:${instance.id}`

      nodes.push({
        id: nodeId,
        type: 'instance',
        position: {
          x: layoutConfig.columns.instance,
          y: instanceStartY + index * layoutConfig.instanceGap,
        },
        data: {
          id: instance.id,
          state: instance.state,
          busyCount: instance.busyCount,
        },
      })

      edges.push({
        id: `edge:server:${serverKey}->instance:${instance.id}`,
        source: `server:${serverKey}`,
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

  return {
    nodes,
    edges,
    tagCount: allTags.length,
    serverCount: serverEntries.size,
    clientCount: activeClients.length,
    instanceCount,
  }
}
