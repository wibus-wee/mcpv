// Input: TanStack Router, ToolsGrid from tools module
// Output: Tools route component with server-organized tools
// Position: /tools route - dedicated tools page

import { createFileRoute } from '@tanstack/react-router'
import { m } from 'motion/react'

import { Separator } from '@/components/ui/separator'
import { Spring } from '@/lib/spring'
import { ToolsGrid } from '@/modules/tools/components/tools-grid'

export const Route = createFileRoute('/tools')({
  component: ToolsPage,
})

function ToolsPage() {
  return (
    <div className="flex flex-1 flex-col p-6 overflow-auto">
      <m.div
        initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
        animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
        transition={Spring.smooth(0.4)}
      >
        <h1 className="text-2xl font-bold tracking-tight">Tools</h1>
        <p className="text-muted-foreground text-sm">
          Available MCP tools from all connected servers
        </p>
      </m.div>
      <Separator className="my-6" />
      <m.div
        initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
        animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
        transition={Spring.smooth(0.4)}
      >
        <ToolsGrid />
      </m.div>
    </div>
  )
}
