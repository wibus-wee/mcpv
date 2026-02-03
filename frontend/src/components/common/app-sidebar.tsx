// Input: Sidebar components from ui/sidebar, icons from lucide-react, TanStack Router
// Output: AppSidebar component with navigation menu
// Position: App-specific sidebar for main layout

import {
  InfoIcon,
  LayoutDashboardIcon,
  NetworkIcon,
  PlugIcon,
  ScrollTextIcon,
  ServerIcon,
  SettingsIcon,
} from 'lucide-react'
import { m } from 'motion/react'

import { ConnectIdeSheet } from '@/components/common/connect-ide-sheet'
import { NavItem } from '@/components/common/nav-item'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuItem,
} from '@/components/ui/sidebar'
import { isDev } from '@/lib/is-dev'
import { Spring } from '@/lib/spring'

const navItems: NavItem[] = [
  {
    path: '/',
    label: 'Dashboard',
    icon: LayoutDashboardIcon,
  },
  {
    path: '/servers',
    label: 'Servers',
    icon: ServerIcon,
  },
  ...isDev
    ? [{
        path: '/plugins',
        label: 'Plugins',
        icon: PlugIcon,
      }]
    : [],
  {
    path: '/logs',
    label: 'Logs',
    icon: ScrollTextIcon,
  },
  {
    path: '/topology',
    label: 'Topology',
    icon: NetworkIcon,
  },
  {
    path: '/settings',
    label: 'Settings',
    icon: SettingsIcon,
  },
  {
    path: '/about',
    label: 'About',
    icon: InfoIcon,
  },
]

export function AppSidebar() {
  'use no memo'

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader className="">
        <div className="flex h-10 items-center justify-center px-2" />
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {navItems.map((item, index) => (
                <SidebarMenuItem key={item.path}>
                  <NavItem item={item} index={index} variant="sidebar" />
                </SidebarMenuItem>
              ))}
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
          {/* Evaluation build */}
          mcpv © 2025. All rights reserved.
        </m.div>
      </SidebarFooter>
    </Sidebar>
  )
}
