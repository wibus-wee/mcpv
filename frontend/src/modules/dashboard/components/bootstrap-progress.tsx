// Input: useBootstrapProgress hook, motion/react, lucide icons
// Output: BootstrapProgressPanel component showing bootstrap status
// Position: Dashboard component for displaying server bootstrap progress

import {
  AlertCircleIcon,
  CheckCircle2Icon,
  Loader2Icon,
  ServerIcon,
} from 'lucide-react'
import { AnimatePresence, m } from 'motion/react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

import type { BootstrapState } from '../hooks'
import { useBootstrapProgress } from '../hooks'

interface BootstrapProgressPanelProps {
  className?: string
}

const stateConfig: Record<BootstrapState, {
  label: string
  variant: 'secondary' | 'warning' | 'success' | 'error'
  icon: typeof Loader2Icon
  iconClassName?: string
}> = {
  pending: {
    label: 'Preparing',
    variant: 'secondary',
    icon: ServerIcon,
  },
  running: {
    label: 'Bootstrapping',
    variant: 'warning',
    icon: Loader2Icon,
    iconClassName: 'animate-spin',
  },
  completed: {
    label: 'Ready',
    variant: 'success',
    icon: CheckCircle2Icon,
  },
  failed: {
    label: 'Failed',
    variant: 'error',
    icon: AlertCircleIcon,
  },
}

/**
 * Displays bootstrap progress with animated progress bar and server status.
 * Shows current server being bootstrapped and any errors.
 */
export function BootstrapProgressPanel({ className }: BootstrapProgressPanelProps) {
  const {
    state,
    total,
    completed,
    failed,
    current,
    errors,
    percentage,
    isBootstrapping,
    hasErrors,
  } = useBootstrapProgress()

  const config = stateConfig[state]
  const Icon = config.icon
  const errorEntries = Object.entries(errors)

  return (
    <m.div
      initial={{ opacity: 0, y: 20, scale: 0.95 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: -10, scale: 0.95 }}
      transition={Spring.smooth(0.4)}
      className={className}
    >
      <Card className="overflow-hidden">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <m.div
                initial={{ rotate: 0 }}
                animate={isBootstrapping ? { rotate: 360 } : { rotate: 0 }}
                transition={isBootstrapping ? { repeat: Infinity, duration: 2, ease: 'linear' } : {}}
              >
                <Icon className={cn('size-5', config.iconClassName)} />
              </m.div>
              <CardTitle className="text-base">Server Bootstrap</CardTitle>
            </div>
            <Badge variant={config.variant} size="sm">
              {config.label}
            </Badge>
          </div>
          <CardDescription>
            {state === 'pending' && 'Preparing to initialize MCP servers...'}
            {state === 'running' && `Initializing servers and fetching metadata...`}
            {state === 'completed' && 'All servers initialized successfully.'}
            {state === 'failed' && 'Some servers failed to initialize.'}
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {/* Progress bar */}
          <div className="space-y-2">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>Progress</span>
              <span>{completed + failed} / {total}</span>
            </div>
            <Progress
              value={percentage}
              className={cn(
                'h-2 transition-all duration-300',
                hasErrors && 'bg-error/20',
              )}
            />
          </div>

          {/* Current server being bootstrapped */}
          <AnimatePresence mode="wait">
            {current && (
              <m.div
                key={current}
                initial={{ opacity: 0, x: -10 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 10 }}
                transition={Spring.smooth(0.2)}
                className="flex items-center gap-2 rounded-md bg-muted/50 px-3 py-2"
              >
                <Loader2Icon className="size-3.5 animate-spin text-muted-foreground" />
                <span className="text-sm font-medium">{current}</span>
              </m.div>
            )}
          </AnimatePresence>

          {/* Stats */}
          <div className="grid grid-cols-3 gap-3 text-center">
            <Tooltip>
              <TooltipTrigger
                render={<div className="rounded-md bg-muted/30 px-2 py-1.5 cursor-default" />}
              >
                <div className="text-lg font-semibold">{total}</div>
                <div className="text-xs text-muted-foreground">Total</div>
              </TooltipTrigger>
              <TooltipContent>Total servers to bootstrap</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger
                render={<div className="rounded-md bg-success/10 px-2 py-1.5 cursor-default" />}
              >
                <div className="text-lg font-semibold text-success">{completed}</div>
                <div className="text-xs text-muted-foreground">Ready</div>
              </TooltipTrigger>
              <TooltipContent>Successfully bootstrapped servers</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger
                render={(
                  <div className={cn(
                    'rounded-md px-2 py-1.5 cursor-default',
                    failed > 0 ? 'bg-error/10' : 'bg-muted/30',
                  )}
                  />
                )}
              >
                <div className={cn(
                  'text-lg font-semibold',
                  failed > 0 ? 'text-error' : 'text-muted-foreground',
                )}
                >
                  {failed}
                </div>
                <div className="text-xs text-muted-foreground">Failed</div>
              </TooltipTrigger>
              <TooltipContent>Failed to bootstrap</TooltipContent>
            </Tooltip>
          </div>

          {/* Error list */}
          {errorEntries.length > 0 && (
            <m.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              className="space-y-1.5"
            >
              <div className="flex items-center gap-1.5 text-xs font-medium text-error">
                <AlertCircleIcon className="size-3.5" />
                <span>Errors</span>
              </div>
              <div className="max-h-24 space-y-1 overflow-y-auto rounded-md bg-error/5 p-2">
                {errorEntries.map(([specKey, error]) => (
                  <Tooltip key={specKey}>
                    <TooltipTrigger
                      render={<div className="truncate text-xs text-error/80 cursor-default" />}
                    >
                      <span className="font-medium">{specKey}:</span> {error}
                    </TooltipTrigger>
                    <TooltipContent side="bottom" className="max-w-xs">
                      <div className="space-y-1">
                        <div className="font-medium">{specKey}</div>
                        <div className="text-xs opacity-80">{error}</div>
                      </div>
                    </TooltipContent>
                  </Tooltip>
                ))}
              </div>
            </m.div>
          )}
        </CardContent>
      </Card>
    </m.div>
  )
}

/**
 * Compact inline version for status bar or header.
 */
export function BootstrapProgressInline() {
  const { state, percentage, current, isBootstrapping, total } = useBootstrapProgress()

  if (state === 'completed' || total === 0) {
    return null
  }

  const config = stateConfig[state]
  const Icon = config.icon

  return (
    <m.div
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.9 }}
      transition={Spring.smooth(0.2)}
      className="flex items-center gap-2"
    >
      <Icon className={cn('size-4', config.iconClassName)} />
      <div className="flex items-center gap-2">
        <Progress value={percentage} className="h-1.5 w-20" />
        <span className="text-xs text-muted-foreground">
          {isBootstrapping && current ? current : `${percentage}%`}
        </span>
      </div>
    </m.div>
  )
}
