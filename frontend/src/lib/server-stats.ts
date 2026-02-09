// Shared server statistics aggregation utilities
import type { ServerInitStatus, ServerRuntimeStatus } from '@bindings/mcpv/internal/ui/types'

import { formatDuration, formatLatency, getElapsedMs } from './time'

export interface PoolStats {
  total: number
  ready: number
  busy: number
  starting: number
  failed: number
  draining: number
}

export interface MetricsSummary {
  totalCalls: number
  totalErrors: number
  avgResponseMs: number | null
  lastCallAgeMs: number | null
  startCount: number
}

export interface AggregatedStats {
  totalServers: number
  totalInstances: number
  readyInstances: number
  busyInstances: number
  startingInstances: number
  failedInstances: number
  drainingInstances: number
  suspendedServers: number
  totalCalls: number
  totalErrors: number
  avgDurationMs: number
  errorRate: number
  utilization: number
}

/**
 * Aggregates pool statistics from a single server runtime status
 */
export function getPoolStats(runtimeStatus: ServerRuntimeStatus): PoolStats {
  const { stats } = runtimeStatus
  return {
    total:
      stats.ready
      + stats.busy
      + stats.starting
      + stats.initializing
      + stats.handshaking
      + stats.draining
      + stats.failed,
    ready: stats.ready,
    busy: stats.busy,
    starting: stats.starting + stats.initializing + stats.handshaking,
    failed: stats.failed,
    draining: stats.draining,
  }
}

/**
 * Aggregates metrics summary from a single server runtime status
 */
export function getMetricsSummary(runtimeStatus: ServerRuntimeStatus): MetricsSummary {
  const { metrics } = runtimeStatus
  const avgResponseMs = metrics.totalCalls > 0 ? metrics.totalDurationMs / metrics.totalCalls : null
  const lastCallAgeMs = getElapsedMs(metrics.lastCallAt)

  return {
    totalCalls: metrics.totalCalls,
    totalErrors: metrics.totalErrors,
    avgResponseMs,
    lastCallAgeMs,
    startCount: metrics.startCount,
  }
}

/**
 * Aggregates statistics from multiple server runtime statuses
 */
export function aggregateStats(
  statuses: ServerRuntimeStatus[],
  initStatuses?: ServerInitStatus[],
): AggregatedStats {
  const result: AggregatedStats = {
    totalServers: statuses.length,
    totalInstances: 0,
    readyInstances: 0,
    busyInstances: 0,
    startingInstances: 0,
    failedInstances: 0,
    drainingInstances: 0,
    suspendedServers: 0,
    totalCalls: 0,
    totalErrors: 0,
    avgDurationMs: 0,
    errorRate: 0,
    utilization: 0,
  }

  if (initStatuses) {
    result.suspendedServers = initStatuses.filter(s => s.state === 'suspended').length
  }

  for (const status of statuses) {
    const { stats } = status
    result.totalInstances += stats.total
    result.readyInstances += stats.ready
    result.busyInstances += stats.busy
    result.startingInstances += stats.starting + stats.initializing + stats.handshaking
    result.failedInstances += stats.failed
    result.drainingInstances += stats.draining

    const { metrics } = status
    result.totalCalls += metrics.totalCalls
    result.totalErrors += metrics.totalErrors
    result.avgDurationMs += metrics.totalDurationMs
  }

  if (result.totalCalls > 0) {
    result.avgDurationMs /= result.totalCalls
    result.errorRate = (result.totalErrors / result.totalCalls) * 100
  }

  if (result.totalInstances > 0) {
    result.utilization = ((result.readyInstances + result.busyInstances) / result.totalInstances) * 100
  }

  return result
}

/**
 * Parse a timestamp string to milliseconds since epoch
 */
export function parseTimestamp(value: string): number | null {
  if (!value) {
    return null
  }
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) {
    return null
  }
  return parsed
}

/**
 * Get the effective start time for an instance (handshakedAt or spawnedAt)
 */
export function getInstanceStartedAt(inst: ServerRuntimeStatus['instances'][number]): number | null {
  const handshakedAt = parseTimestamp(inst.handshakedAt)
  if (handshakedAt !== null) {
    return handshakedAt
  }
  return parseTimestamp(inst.spawnedAt)
}

/**
 * Calculate the oldest uptime across all instances
 */
export function getOldestUptimeMs(instances: ServerRuntimeStatus['instances']): number | null {
  let oldestStartedAt: number | null = null
  for (const inst of instances) {
    const startedAt = getInstanceStartedAt(inst)
    if (startedAt === null) {
      continue
    }
    if (oldestStartedAt === null || startedAt < oldestStartedAt) {
      oldestStartedAt = startedAt
    }
  }
  if (oldestStartedAt === null) {
    return null
  }
  return Math.max(0, Date.now() - oldestStartedAt)
}

/**
 * Render a timeline string for an instance showing uptime, handshake time, and heartbeat
 */
export function formatInstanceTimeline(inst: ServerRuntimeStatus['instances'][number]): string | null {
  const parts: string[] = []
  const uptimeMs = getElapsedMs(inst.handshakedAt || inst.spawnedAt)
  if (uptimeMs !== null) {
    parts.push(`Up ${formatDuration(uptimeMs)}`)
  }

  const spawnedAt = parseTimestamp(inst.spawnedAt)
  const handshakedAt = parseTimestamp(inst.handshakedAt)
  if (spawnedAt !== null && handshakedAt !== null) {
    const handshakeMs = Math.max(0, handshakedAt - spawnedAt)
    parts.push(`Handshake ${formatLatency(handshakeMs)}`)
  }

  const heartbeatAgeMs = getElapsedMs(inst.lastHeartbeatAt)
  if (heartbeatAgeMs !== null) {
    parts.push(`Heartbeat ${formatDuration(heartbeatAgeMs)} ago`)
  }

  if (parts.length === 0) {
    return null
  }

  return parts.join(' · ')
}

/**
 * Format instance ID with truncation for display
 */
export function formatInstanceId(id: string): string {
  if (id.length <= 12) {
    return id
  }
  return `${id.slice(0, 8)}...${id.slice(-3)}`
}
