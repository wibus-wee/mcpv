// Input: Sidebar components, Badge from ui, icons, theme provider, core status view + connection hooks
// Output: AppTopbar component with status indicator and controls
// Position: Top bar for app layout showing status and window controls

import { CloudIcon, HardDriveIcon, MoonIcon, SunIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useTheme } from 'next-themes'

import { ActiveClientsIndicator } from '@/components/common/active-clients-indicator'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { SidebarTrigger } from '@/components/ui/sidebar'
import { useCoreConnectionMode } from '@/hooks/use-core-connection'
import { useCoreStatusViewState } from '@/hooks/use-core-status-view'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'
import { coreStatusConfig } from '@/modules/shared/core-status'

export function AppTopbar() {
  const { theme, setTheme } = useTheme()
  const { coreStatus, view } = useCoreStatusViewState()
  const { isRemote } = useCoreConnectionMode()
  const config = coreStatusConfig[coreStatus]
  const StatusIcon = config.icon
  const ConnectionIcon = isRemote ? CloudIcon : HardDriveIcon
  const showViewBadge = isRemote && view === 'local'

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
              config.topbarColor,
            )}
          >
            {StatusIcon && (
              <StatusIcon className="size-3" />
            )}
            <span className="font-medium text-white text-xs">{config.label}</span>
          </Badge>
        </m.div>

        <m.div
          initial={{ opacity: 0, y: -6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={Spring.snappy(0.3)}
        >
          <Badge variant={isRemote ? 'info' : 'secondary'} className="flex items-center gap-1.5 px-2 py-1">
            <ConnectionIcon className="size-3" />
            <span className="font-medium text-xs">{isRemote ? 'Remote' : 'Local'}</span>
          </Badge>
        </m.div>

        {showViewBadge && (
          <m.div
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            transition={Spring.snappy(0.3, 0.05)}
          >
            <Badge variant="outline" size="sm" className="px-2 py-1 text-xs">
              Local view
            </Badge>
          </m.div>
        )}

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
