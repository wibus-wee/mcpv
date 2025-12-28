// Input: TanStack Router, LogsPanel from dashboard module
// Output: Logs route component with full logs viewer
// Position: /logs route - dedicated logs page

import { createFileRoute } from '@tanstack/react-router'
import { m } from 'motion/react'

import { Separator } from '@/components/ui/separator'
import { Spring } from '@/lib/spring'
import { LogsPanel } from '@/modules/dashboard/components'

export const Route = createFileRoute('/logs')({
  component: LogsPage,
})

function LogsPage() {
  return (
    <div className="flex flex-1 flex-col p-6 overflow-auto">
      <m.div
        initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
        animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
        transition={Spring.smooth(0.4)}
      >
        <h1 className="text-2xl font-bold tracking-tight">Logs</h1>
        <p className="text-muted-foreground text-sm">
          System logs from MCP servers
        </p>
      </m.div>
      <Separator className="my-6" />
      <m.div
        initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
        animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
        transition={Spring.smooth(0.4)}
        className="flex-1"
      >
        <LogsPanel />
      </m.div>
    </div>
  )
}
