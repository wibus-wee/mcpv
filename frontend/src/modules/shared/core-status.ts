import type { VariantProps } from 'class-variance-authority'
import { CheckCircle2Icon, Loader2Icon, XCircleIcon } from 'lucide-react'

import type { badgeVariants } from '@/components/ui/badge'

export type CoreStatus = 'stopped' | 'starting' | 'running' | 'stopping' | 'error'

type BadgeVariant = VariantProps<typeof badgeVariants>['variant']

export interface CoreStatusConfig {
  variant: BadgeVariant
  label: string
  icon: typeof CheckCircle2Icon | typeof Loader2Icon | typeof XCircleIcon | null
  dotColor: string
  topbarColor: string
}

export const coreStatusConfig: Record<CoreStatus, CoreStatusConfig> = {
  stopped: {
    variant: 'secondary',
    label: 'Stopped',
    icon: XCircleIcon,
    dotColor: 'bg-slate-400',
    topbarColor: 'bg-muted text-muted-foreground',
  },
  starting: {
    variant: 'warning',
    label: 'Starting',
    icon: Loader2Icon,
    dotColor: 'bg-amber-500',
    topbarColor: 'bg-warning text-warning-foreground',
  },
  running: {
    variant: 'success',
    label: 'Running',
    icon: CheckCircle2Icon,
    dotColor: 'bg-emerald-500',
    topbarColor: 'bg-success text-success-foreground',
  },
  stopping: {
    variant: 'warning',
    label: 'Stopping',
    icon: Loader2Icon,
    dotColor: 'bg-amber-500',
    topbarColor: 'bg-warning text-warning-foreground',
  },
  error: {
    variant: 'error',
    label: 'Error',
    icon: XCircleIcon,
    dotColor: 'bg-red-500',
    topbarColor: 'bg-destructive text-destructive-foreground',
  },
}

// Legacy exports for backward compatibility
export const coreStatusVariants: Record<CoreStatus, BadgeVariant> = Object.fromEntries(
  Object.entries(coreStatusConfig).map(([key, config]) => [key, config.variant]),
) as Record<CoreStatus, BadgeVariant>

export const coreStatusLabels: Record<CoreStatus, string> = Object.fromEntries(
  Object.entries(coreStatusConfig).map(([key, config]) => [key, config.label]),
) as Record<CoreStatus, string>
