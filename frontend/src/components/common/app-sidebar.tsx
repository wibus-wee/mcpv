// Input: Sidebar components from ui/sidebar, icons from lucide-react, TanStack Router
// Output: AppSidebar component with navigation menu
// Position: App-specific sidebar for main layout

import { Link, useMatchRoute } from '@tanstack/react-router'
import {
  FileSliders,
  LayoutDashboardIcon,
  ScrollTextIcon,
  SettingsIcon,
  WrenchIcon,
} from 'lucide-react'
import { m } from 'motion/react'

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'
import { ConnectIdeSheet } from '@/components/common/connect-ide-sheet'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

interface NavItem {
  path: string
  label: string
  icon: typeof LayoutDashboardIcon
}

const navItems: NavItem[] = [
  {
    path: '/',
    label: 'Dashboard',
    icon: LayoutDashboardIcon,
  },
  {
    path: '/tools',
    label: 'Tools',
    icon: WrenchIcon,
  },
  {
    path: '/logs',
    label: 'Logs',
    icon: ScrollTextIcon,
  },
  {
    path: '/config',
    label: 'Configuration',
    icon: FileSliders,
  },
  {
    path: '/settings',
    label: 'Settings',
    icon: SettingsIcon,
  },
]

export function AppSidebar() {
  const matchRoute = useMatchRoute()

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader className="">
        <div className="flex h-10 items-center justify-center px-2" />
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {navItems.map((item, index) => {
                const Icon = item.icon
                const isActive = !!matchRoute({ to: item.path, fuzzy: false })

                return (
                  <SidebarMenuItem key={item.path}>
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
                  </SidebarMenuItem>
                )
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter className="border-sidebar-border border-t flex flex-col gap-2 justify-center">
          <ConnectIdeSheet />
        <m.div
          className="p-2 text-center text-muted-foreground text-xs group-data-[collapsible=icon]:hidden"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={Spring.smooth(0.4)}
        >
          {/* 评估版本 */}
          mcpd © 2025. All rights reserved.
        </m.div>
      </SidebarFooter>
    </Sidebar>
  )
}
