// Input: NavItem interface, variant prop for different rendering modes
// Output: Reusable NavItem component with animation and active state logic
// Position: Common component for navigation items across the app

import { Link, useMatchRoute } from '@tanstack/react-router'
import type { LucideIcon } from 'lucide-react'
import { m } from 'motion/react'

import { SidebarMenuButton } from '@/components/ui/sidebar'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

export interface NavItem {
  path: string
  label: string
  icon: LucideIcon
  description?: string
}

interface NavItemProps {
  item: NavItem
  index: number
  variant: 'sidebar' | 'inline'
  fuzzy?: boolean
}

export function NavItem({ item, index, variant, fuzzy = true }: NavItemProps) {
  'use no memo'
  const matchRoute = useMatchRoute()
  const Icon = item.icon
  const isActive = !!matchRoute({ to: item.path, fuzzy })

  if (variant === 'sidebar') {
    return (
      <m.div
        initial={{ opacity: 0, x: -10 }}
        animate={{ opacity: 1, x: 0 }}
        transition={Spring.smooth(0.3, index * 0.05)}
      >
        <SidebarMenuButton
          render={props => (
            <Link
              to={item.path}
              {...props}
            />
          )}
          isActive={isActive}
          tooltip={item.label}
        >
          <Icon className={cn(
            'transition-colors',
            isActive && 'text-sidebar-accent-foreground',
          )}
          />
          <span>{item.label}</span>
        </SidebarMenuButton>
      </m.div>
    )
  }

  return (
    <m.div
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
