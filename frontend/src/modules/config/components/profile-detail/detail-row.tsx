// Input: label and value props
// Output: DetailRow component for displaying key-value pairs
// Position: Shared component within profile-detail

import { cn } from '@/lib/utils'

interface DetailRowProps {
  label: string
  value: React.ReactNode
  mono?: boolean
  className?: string
}

/**
 * A simple key-value row component for displaying configuration details.
 */
export function DetailRow({
  label,
  value,
  mono = false,
  className,
}: DetailRowProps) {
  return (
    <div className={cn('flex items-center justify-between py-1.5', className)}>
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className={cn(mono ? 'font-mono text-xs' : 'text-xs')}>{value}</span>
    </div>
  )
}
