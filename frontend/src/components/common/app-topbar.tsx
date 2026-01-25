// Input: Sidebar components, Badge from ui, icons, theme provider, core status hook
// Output: AppTopbar component with status indicator and controls
// Position: Top bar for app layout showing status and window controls

import { Loader2Icon, MoonIcon, SunIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useTheme } from 'next-themes'

import { ActiveClientsIndicator } from '@/components/common/active-clients-indicator'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { SidebarTrigger } from '@/components/ui/sidebar'
import { useCoreState } from '@/hooks/use-core-state'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

const statusConfig = {
  stopped: {
    label: 'Stopped',
    color: 'bg-muted text-muted-foreground',
    icon: null,
  },
  starting: {
    label: 'Starting',
    color: 'bg-warning text-warning-foreground',
    icon: Loader2Icon,
  },
  running: {
    label: 'Running',
    color: 'bg-success text-success-foreground',
    icon: null,
  },
  stopping: {
    label: 'Stopping',
    color: 'bg-warning text-warning-foreground',
    icon: Loader2Icon,
  },
  error: {
    label: 'Error',
    color: 'bg-destructive text-destructive-foreground',
    icon: null,
  },
} as const

export function AppTopbar() {
  const { theme, setTheme } = useTheme()
  const { coreStatus } = useCoreState()
  const config = statusConfig[coreStatus]
  const StatusIcon = config.icon

  return (
    <header className="flex h-14 items-center justify-between border-border border-b bg-background px-4">
      {/* Left section - Sidebar trigger and status */}
      <div className="flex items-center gap-3">
        <SidebarTrigger />

        <m.div
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={Spring.snappy(0.3)}
        >
          <Badge
            className={cn(
              'flex items-center gap-1.5 px-2.5 py-1',
              config.color,
            )}
          >
            {StatusIcon && (
              <StatusIcon className="size-3 animate-spin" />
            )}
            <span className="font-medium text-white text-xs">{config.label}</span>
          </Badge>
        </m.div>

        <ActiveClientsIndicator />
      </div>

      {/* Right section - Theme toggle */}
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
          className="size-8"
        >
          <SunIcon className="size-4 rotate-0 scale-100 transition-all dark:-rotate-90 dark:scale-0" />
          <MoonIcon className="absolute size-4 rotate-90 scale-0 transition-all dark:rotate-0 dark:scale-100" />
          <span className="sr-only">Toggle theme</span>
        </Button>
      </div>
    </header>
  )
}
