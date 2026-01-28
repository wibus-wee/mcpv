import type { LucideIcon } from 'lucide-react'
import { ServerIcon } from 'lucide-react'

import { UniversalEmptyState } from '@/components/common/universal-empty-state'

interface ServerEmptyStateProps {
  icon?: LucideIcon
  title?: string
  description?: string
  action?: {
    label: string
    onClick: () => void
  }
}

export function ServerEmptyState({
  icon = ServerIcon,
  title = 'No servers',
  description = 'No servers are currently configured or available.',
  action,
}: ServerEmptyStateProps) {
  return (
    <UniversalEmptyState
      icon={icon}
      title={title}
      description={description}
      action={action}
    />
  )
}
