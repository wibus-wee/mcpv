// Status badge component for log level display
// Vercel style: fixed-width pill with color coding

import { Badge } from '@/components/ui/badge'

import type { LogLevel } from '../types'

interface LogLevelBadgeProps {
  level: LogLevel
}

const levelVariantMap = {
  debug: 'outline',
  info: 'info',
  warn: 'warning',
  error: 'error',
} as const

export function LogLevelBadge({ level }: LogLevelBadgeProps) {
  return (
    <Badge
      variant={levelVariantMap[level]}
      size="sm"
      className="w-14 justify-center font-mono uppercase"
    >
      {level}
    </Badge>
  )
}
