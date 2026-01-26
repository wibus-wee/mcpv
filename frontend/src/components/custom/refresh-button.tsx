// Input: onClick handler, loading state
// Output: RefreshButton component with rotation animation
// Position: Custom atomic component for refresh actions

import { RefreshCwIcon } from 'lucide-react'
import { m } from 'motion/react'

import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

interface RefreshButtonProps {
  onClick: () => void
  isLoading?: boolean
  className?: string
  tooltip?: string
  size?: 'icon-xs' | 'icon-sm' | 'icon'
}

export function RefreshButton({
  onClick,
  isLoading = false,
  className,
  tooltip = 'Refresh',
  size = 'icon-sm',
}: RefreshButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={(
          <Button
            variant="ghost"
            size={size}
            onClick={onClick}
            disabled={isLoading}
            className={cn('text-muted-foreground', className)}
          >
            <m.div
              animate={isLoading ? { rotate: 360 } : { rotate: 0 }}
              transition={
                isLoading
                  ? { duration: 1, repeat: Infinity, ease: 'linear' }
                  : { duration: 0.2 }
              }
            >
              <RefreshCwIcon className="size-3.5" />
            </m.div>
          </Button>
        )}
      />
      <TooltipContent>{tooltip}</TooltipContent>
    </Tooltip>
  )
}
