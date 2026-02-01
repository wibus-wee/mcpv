// Log utility functions
// Handles formatting, filtering, and data transformation

import type { LogEntry, LogFilters, LogRowData, LogSegment } from '../types'

// Hidden fields that shouldn't be displayed in inline view
const HIDDEN_FIELD_KEYS = new Set([
  'log_source',
  'logger',
  'serverType',
  'stream',
  'timestamp',
])

/**
 * Format a field value for display
 */
export function formatFieldValue(value: unknown): string {
  if (value === null) return 'null'
  if (value === undefined) return 'undefined'
  if (value instanceof Date) return value.toISOString()
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    }
    catch {
      return String(value)
    }
  }
  return String(value)
}

/**
 * Format timestamp to time string with milliseconds
 */
export function formatTime(date: Date): string {
  return date.toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    fractionalSecondDigits: 3,
  })
}

/**
 * Format timestamp to full date time string
 */
export function formatDateTime(date: Date): string {
  return date.toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    fractionalSecondDigits: 3,
    hour12: false,
  })
}

/**
 * Format duration in milliseconds to human readable string
 */
export function formatDuration(ms: number): string {
  if (ms < 1) return '<1ms'
  if (ms < 1000) return `${Math.round(ms)}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`
  return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`
}

/**
 * Format inline message - escape newlines for single-line display
 */
export function formatInlineMessage(message: string): string {
  return message.replaceAll('\n', '↵ ')
}

/**
 * Format fields as key=value pairs for inline display
 */
export function formatInlineFields(fields: Record<string, unknown>): string {
  const entries = Object.entries(fields).filter(
    ([key]) => !HIDDEN_FIELD_KEYS.has(key),
  )
  if (entries.length === 0) return ''
  return entries
    .map(([key, value]) => `${key}=${formatFieldValue(value)}`)
    .join(' ')
}

/**
 * Build log segments for row display
 */
export function buildLogSegments(log: LogEntry): LogSegment[] {
  const segments: LogSegment[] = [
    { text: formatTime(log.timestamp), type: 'time' },
    { text: log.level.toUpperCase(), type: 'level' },
    { text: log.source, type: 'source' },
  ]

  if (log.serverType) {
    segments.push({ text: log.serverType, type: 'server' })
  }

  const message = formatInlineMessage(log.message)
  segments.push({
    text: message,
    type: log.level === 'error' ? 'error' : 'message',
  })

  const inlineFields = formatInlineFields(log.fields)
  if (inlineFields) {
    segments.push({ text: inlineFields, type: 'field' })
  }

  return segments
}

/**
 * Create processed log row data
 */
export function createLogRowData(log: LogEntry): LogRowData {
  return {
    log,
    segments: buildLogSegments(log),
  }
}

/**
 * Filter logs based on filter criteria
 */
export function filterLogs(logs: LogEntry[], filters: LogFilters): LogEntry[] {
  return logs.filter((log) => {
    // Level filter
    if (filters.level !== 'all' && log.level !== filters.level) {
      return false
    }

    // Source filter
    if (filters.source !== 'all' && log.source !== filters.source) {
      return false
    }

    // Server filter
    if (filters.server !== 'all' && log.serverType !== filters.server) {
      return false
    }

    // Search filter
    if (filters.search) {
      const searchLower = filters.search.toLowerCase()
      const matchesMessage = log.message.toLowerCase().includes(searchLower)
      const matchesServer = log.serverType
        ?.toLowerCase()
        .includes(searchLower)
      const matchesLogger = log.logger?.toLowerCase().includes(searchLower)
      const matchesFields = Object.values(log.fields).some(value =>
        formatFieldValue(value).toLowerCase().includes(searchLower),
      )

      if (!matchesMessage && !matchesServer && !matchesLogger && !matchesFields) {
        return false
      }
    }

    return true
  })
}

/**
 * Extract unique server names from logs
 */
export function extractServerNames(logs: LogEntry[]): string[] {
  const servers = new Set<string>()
  for (const log of logs) {
    if (log.serverType) {
      servers.add(log.serverType)
    }
  }
  return Array.from(servers).sort()
}

/**
 * Count logs by level
 */
export function countByLevel(
  logs: LogEntry[],
): Record<LogEntry['level'], number> {
  const counts: Record<LogEntry['level'], number> = {
    debug: 0,
    info: 0,
    warn: 0,
    error: 0,
  }
  for (const log of logs) {
    counts[log.level]++
  }
  return counts
}

/**
 * Check if log has error-level content
 */
export function isErrorLog(log: LogEntry): boolean {
  return log.level === 'error'
}

/**
 * Check if log has warning-level content
 */
export function isWarningLog(log: LogEntry): boolean {
  return log.level === 'warn'
}

/**
 * Get visible fields from log (excluding hidden keys)
 */
export function getVisibleFields(
  fields: Record<string, unknown>,
): Array<[string, unknown]> {
  return Object.entries(fields).filter(([key]) => !HIDDEN_FIELD_KEYS.has(key))
}
