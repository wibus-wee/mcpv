// Input: Tools data from hooks, runtime status
// Output: ToolsGrid component with responsive layout
// Position: Main grid component for tools page

import { useState } from 'react'
import { m } from 'motion/react'
import { ServerOffIcon } from 'lucide-react'

import { Skeleton } from '@/components/ui/skeleton'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

import { useToolsByServer } from '../hooks'
import { ServerCard } from './server-card'
import { ServerDetailsSheet } from './server-details-sheet'

interface ToolsGridProps {
  className?: string
}

export function ToolsGrid({ className }: ToolsGridProps) {
  const { servers, isLoading } = useToolsByServer()
  const [selectedServerId, setSelectedServerId] = useState<string | null>(null)
  const [sheetOpen, setSheetOpen] = useState(false)

  const handleServerClick = (serverId: string) => {
    setSelectedServerId(serverId)
    setSheetOpen(true)
  }

  const handleSheetClose = (open: boolean) => {
    setSheetOpen(open)
    if (!open) {
      setTimeout(() => setSelectedServerId(null), 200)
    }
  }

  if (isLoading) {
    return (
      <div className={cn('grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4', className)}>
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="h-48 rounded-xl" />
        ))}
      </div>
    )
  }

  if (servers.length === 0) {
    return (
      <div className={cn('flex flex-col items-center justify-center h-full gap-4', className)}>
        <ServerOffIcon className="size-16 text-muted-foreground" />
        <div className="text-center">
          <h3 className="text-lg font-semibold">No servers configured</h3>
          <p className="text-sm text-muted-foreground">
            Add MCP servers to your configuration to see tools
          </p>
        </div>
      </div>
    )
  }

  const selectedServerData = servers.find(s => s.id === selectedServerId)

  return (
    <>
      <div className={cn('grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4', className)}>
        {servers.map((server, index) => (
          <m.div
            key={server.id}
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={Spring.smooth(0.3, index * 0.05)}
          >
            <ServerCard
              specKey={server.specKey}
              serverName={server.serverName}
              toolCount={server.tools.length}
              onClick={() => handleServerClick(server.id)}
            />
          </m.div>
        ))}
      </div>

      {selectedServerData && (
        <ServerDetailsSheet
          specKey={selectedServerData.specKey}
          serverName={selectedServerData.serverName}
          tools={selectedServerData.tools}
          open={sheetOpen}
          onOpenChange={handleSheetClose}
        />
      )}
    </>
  )
}
