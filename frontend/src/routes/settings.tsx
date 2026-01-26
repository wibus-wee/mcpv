// Input: TanStack Router, Outlet, Settings layout components
// Output: Settings layout route with sidebar navigation
// Position: /settings layout route for nested settings pages

import { createFileRoute, Link, Outlet, useMatchRoute } from '@tanstack/react-router'
import { BugIcon, PaletteIcon, ServerIcon, SettingsIcon } from 'lucide-react'
import { m } from 'motion/react'

import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/settings')({
  component: SettingsLayout,
})

interface NavItem {
  path: string
  label: string
  icon: typeof SettingsIcon
  description: string
}

const navItems: NavItem[] = [
  {
    path: '/settings/runtime',
    label: 'Runtime',
    icon: ServerIcon,
    description: 'Timeouts, retries, and global defaults',
  },
  {
    path: '/settings/subagent',
    label: 'SubAgent',
    icon: SettingsIcon,
    description: 'AI assistant configuration',
  },
  {
    path: '/settings/appearance',
    label: 'Appearance',
    icon: PaletteIcon,
    description: 'Theme and UI preferences',
  },
  {
    path: '/settings/advanced',
    label: 'Advanced',
    icon: BugIcon,
    description: 'Debug logs and telemetry',
  },
]

function SettingsLayout() {
  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <m.div
        className="px-6 pt-6 pb-4"
        initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
        animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
        transition={Spring.presets.smooth}
      >
        <div className="flex items-center gap-2">
          <SettingsIcon className="size-4 text-muted-foreground" />
          <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
        </div>
        <p className="text-sm text-muted-foreground">
          Configure runtime defaults and preferences
        </p>
      </m.div>

      <Separator />

      <div className="flex min-h-0 flex-1">
        {/* Sidebar Navigation */}
        <nav className="w-56 shrink-0 border-r">
          <ScrollArea className="h-full">
            <div className="space-y-1 p-3">
              {navItems.map((item, index) => (
                <NavItemComponent key={item.path} item={item} index={index} />
              ))}
            </div>
          </ScrollArea>
        </nav>

        {/* Content Area */}
        <div className="min-w-0 flex-1 overflow-auto">
          <Outlet />
        </div>
      </div>
    </div>
  )
}

const NavItemComponent = ({ item, index }: { item: NavItem, index: number }) => {
  const matchRoute = useMatchRoute()
  const Icon = item.icon
  const isActive = !!matchRoute({ to: item.path, fuzzy: false })

  return (
    <m.div
      key={item.path}
      initial={{ opacity: 0, x: -10 }}
      animate={{ opacity: 1, x: 0 }}
      transition={Spring.smooth(0.3, index * 0.05)}
    >
      <Link
        to={item.path}
        className={cn(
          'flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors',
          isActive
            ? 'bg-primary/10 text-primary'
            : 'text-muted-foreground hover:bg-muted hover:text-foreground',
        )}
      >
        <Icon className="size-4" />
        <div className="min-w-0">
          <div className="font-medium">{item.label}</div>
        </div>
      </Link>
    </m.div>
  )
}
