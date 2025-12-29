// Input: TanStack Router, ToolsGrid from tools module
// Output: Tools route component with master-detail layout
// Position: /tools route - dedicated tools page with Linear/Vercel-style UX

import { createFileRoute } from '@tanstack/react-router'
import { m } from 'motion/react'

import { Spring } from '@/lib/spring'
import { ToolsGrid } from '@/modules/tools/components/tools-grid'

export const Route = createFileRoute('/tools')({
  component: ToolsPage,
})

function ToolsPage() {
  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <m.div
        initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
        animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
        transition={Spring.smooth(0.4)}
        className="mb-4 p-6 pb-0"
      >
        <h1 className="text-2xl font-bold tracking-tight">Tools</h1>
        <p className="text-muted-foreground text-sm">
          Available MCP tools from all connected servers
        </p>
      </m.div>
      <ToolsGrid />
    </div>
  )
}
