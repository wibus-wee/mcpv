export const formatDuration = (ms: number): string => {
  const totalSeconds = Math.floor(ms / 1000)
  if (totalSeconds <= 0) return '0s'

  const seconds = totalSeconds % 60
  const totalMinutes = Math.floor(totalSeconds / 60)
  const minutes = totalMinutes % 60
  const hours = Math.floor(totalMinutes / 60)
  const days = Math.floor(hours / 24)
  const remHours = hours % 24

  if (totalSeconds < 60) return `${totalSeconds}s`
  if (totalSeconds < 3600) return `${totalMinutes}m ${seconds}s`
  if (totalSeconds < 86400) return `${hours}h ${minutes}m`
  return `${days}d ${remHours}h`
}

export const formatLatency = (ms: number): string => {
  if (ms < 1000) {
    return `${Math.round(ms)}ms`
  }
  const seconds = ms / 1000
  if (seconds < 60) {
    return `${seconds.toFixed(1)}s`
  }
  const minutes = Math.floor(seconds / 60)
  const remSeconds = Math.round(seconds % 60)
  return `${minutes}m ${remSeconds}s`
}

export const getElapsedMs = (timestamp?: string | null): number | null => {
  if (!timestamp) return null
  const parsed = Date.parse(timestamp)
  if (Number.isNaN(parsed)) return null
  return Math.max(0, Date.now() - parsed)
}

export function formatRelativeTime(timestamp?: string): string {
  if (!timestamp) {
    return '—'
  }
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return '—'
  }
  const diffMs = Date.now() - date.getTime()
  if (diffMs < 0) {
    return 'just now'
  }
  const seconds = Math.floor(diffMs / 1000)
  if (seconds < 60) {
    return `${seconds}s ago`
  }
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) {
    return `${minutes} minutes ago`
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return `${hours} hours ago`
  }
  const days = Math.floor(hours / 24)
  return `${days} days ago`
}
