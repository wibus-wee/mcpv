// Input: SidebarInset from ui/sidebar, AppTopbar, Motion, ReactNode
// Output: MainContent component wrapping page content
// Position: Main content area wrapper in app layout

import { m } from 'motion/react'
import type { ReactNode } from 'react'

import { AppTopbar } from '@/components/common/app-topbar'
import { SidebarInset } from '@/components/ui/sidebar'
import { Spring } from '@/lib/spring'

export interface MainContentProps {
  children: ReactNode
}

export function MainContent({ children }: MainContentProps) {
  return (
    <SidebarInset className="h-screen overflow-scroll">
      <AppTopbar />
      <m.main
        className="flex flex-1 flex-col overflow-hidden"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={Spring.smooth(0.3)}
      >
        {children}
      </m.main>
    </SidebarInset>
  )
}
