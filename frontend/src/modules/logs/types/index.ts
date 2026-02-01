// Log module type definitions
// Mapping: Vercel Logs concepts -> mcpv concepts

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'
export type LogSource = 'core' | 'downstream' | 'ui' | 'unknown'

/**
 * Core log entry type - extends the base log with display metadata
 */
export interface LogEntry {
  id: string
  timestamp: Date
  level: LogLevel
  message: string
  source: LogSource
  logger?: string
  serverType?: string
  stream?: string
  fields: Record<string, unknown>
}

/**
 * Processed log row data for virtualized list rendering
 */
export interface LogRowData {
  log: LogEntry
  segments: LogSegment[]
}

/**
 * Text segment with styling for log row display
 */
export interface LogSegment {
  text: string
  type: 'time' | 'level' | 'source' | 'server' | 'message' | 'field' | 'error'
}

/**
 * Filter state for the log list
 */
export interface LogFilters {
  level: LogLevel | 'all'
  source: LogSource | 'all'
  server: string | 'all'
  search: string
}

/**
 * Execution trace node for detail drawer
 * Maps to: Vercel's Function Invocation / Middleware layer
 */
export interface TraceNode {
  id: string
  type: 'receive' | 'route' | 'instance' | 'call' | 'response'
  label: string
  timestamp: Date
  duration?: number
  status?: 'success' | 'error' | 'pending'
  metadata?: Record<string, unknown>
}

/**
 * Log detail for drawer view
 */
export interface LogDetail {
  log: LogEntry
  trace?: TraceNode[]
  relatedLogs?: LogEntry[]
}

/**
 * Status badge variant mapping
 */
export const levelVariantMap: Record<LogLevel, 'default' | 'info' | 'warning' | 'error'> = {
  debug: 'default',
  info: 'info',
  warn: 'warning',
  error: 'error',
}

/**
 * Level display labels
 */
export const levelLabels: Record<LogLevel | 'all', string> = {
  all: 'All levels',
  debug: 'Debug',
  info: 'Info',
  warn: 'Warning',
  error: 'Error',
}

/**
 * Source display labels
 */
export const sourceLabels: Record<LogSource | 'all', string> = {
  all: 'All sources',
  core: 'Core',
  downstream: 'Downstream',
  ui: 'UI',
  unknown: 'Unknown',
}

/**
 * Node type icons - used for trace visualization
 * M = Middleware (route), F = Function (instance call)
 */
export const nodeTypeLabels: Record<TraceNode['type'], string> = {
  receive: 'R',
  route: 'M',
  instance: 'I',
  call: 'F',
  response: '✓',
}
