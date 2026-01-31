import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

const categoryColors = {
  observability: 'bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20',
  authentication: 'bg-orange-500/10 text-orange-700 dark:text-orange-400 border-orange-500/20',
  authorization: 'bg-red-500/10 text-red-700 dark:text-red-400 border-red-500/20',
  rate_limiting: 'bg-yellow-500/10 text-yellow-700 dark:text-yellow-400 border-yellow-500/20',
  validation: 'bg-green-500/10 text-green-700 dark:text-green-400 border-green-500/20',
  content: 'bg-purple-500/10 text-purple-700 dark:text-purple-400 border-purple-500/20',
  audit: 'bg-gray-500/10 text-gray-700 dark:text-gray-400 border-gray-500/20',
} as const

const categoryLabels = {
  observability: 'Observability',
  authentication: 'Authentication',
  authorization: 'Authorization',
  rate_limiting: 'Rate Limiting',
  validation: 'Validation',
  content: 'Content',
  audit: 'Audit',
} as const

type PluginCategory = keyof typeof categoryColors

interface PluginCategoryBadgeProps {
  category: string
  className?: string
}

export function PluginCategoryBadge({ category, className }: PluginCategoryBadgeProps) {
  const categoryKey = category.toLowerCase() as PluginCategory
  const colorClass = categoryColors[categoryKey] || categoryColors.audit
  const label = categoryLabels[categoryKey] || category

  return (
    <Badge
      variant="outline"
      size="sm"
      className={cn(
        colorClass,
        'font-medium',
        className,
      )}
    >
      {label}
    </Badge>
  )
}
