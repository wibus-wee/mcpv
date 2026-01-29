// Input: serverName, tab from router, callbacks
// Output: Inline detail panel with tabs for Overview/Tools/Config
// Position: Right pane in master-detail layout

import type { ServerDetail } from '@bindings/mcpd/internal/ui'
import { LayoutGridIcon, SettingsIcon, WrenchIcon } from 'lucide-react'
import { Activity } from 'react'

import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

import type { ServerTab } from '../constants'
import { ServerConfigPanel } from './server-config-panel'
import { ServerDetailEmptyState } from './server-detail-empty-state'
import { ServerOverviewPanel } from './server-overview-panel'
import { ServerToolsPanel } from './server-tools-panel'

interface ServerDetailPanelProps {
  server: ServerDetail | null
  isLoading: boolean
  tab: ServerTab
  toolCount: number
  onTabChange: (tab: ServerTab) => void
  onEdit: () => void
  onDeleted: () => void
  className?: string
}

function DetailSkeleton() {
  return (
    <div className="p-6 space-y-4">
      <div className="space-y-2">
        <Skeleton className="h-7 w-48" />
        <Skeleton className="h-4 w-32" />
      </div>
      <Skeleton className="h-10 w-64" />
      <div className="space-y-3 pt-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    </div>
  )
}

export function ServerDetailPanel({
  server,
  isLoading,
  tab,
  toolCount,
  onTabChange,
  onEdit,
  onDeleted,
  className,
}: ServerDetailPanelProps) {
  if (!server && !isLoading) {
    return <ServerDetailEmptyState />
  }

  if (isLoading) {
    return <DetailSkeleton />
  }

  if (!server) {
    return <DetailSkeleton />
  }

  return (
    <div className={cn('flex flex-col h-full w-full', className)}>
      {/* Tabs */}
      <Tabs
        value={tab}
        onValueChange={v => onTabChange(v as ServerTab)}
        className="flex-1 flex flex-col min-h-0"
      >
        <TabsList variant="underline" className="px-4 border-b w-full">
          <TabsTrigger value="overview">
            <LayoutGridIcon className="size-4" />
            Overview
          </TabsTrigger>
          <TabsTrigger value="tools">
            <WrenchIcon className="size-4" />
            Tools
            <Badge variant="secondary" size="sm" className="ml-1.5">
              {toolCount}
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="configuration">
            <SettingsIcon className="size-4" />
            Configuration
          </TabsTrigger>
        </TabsList>

        <div className="flex-1 min-h-0">
          <TabsContent value="overview" keepMounted className="m-0 p-0 h-full">
            <Activity mode={tab === 'overview' ? 'visible' : 'hidden'}>
              <ScrollArea className="h-full">
                <div className="p-6">
                  <ServerOverviewPanel server={server} />
                </div>
              </ScrollArea>
            </Activity>
          </TabsContent>

          <TabsContent value="tools" keepMounted className="m-0 p-0 h-full">
            <Activity mode={tab === 'tools' ? 'visible' : 'hidden'}>
              <ServerToolsPanel serverName={server.name} />
            </Activity>
          </TabsContent>

          <TabsContent value="configuration" keepMounted className="m-0 p-0 h-full">
            <Activity mode={tab === 'configuration' ? 'visible' : 'hidden'}>
              <ScrollArea className="h-full">
                <div className="p-6">
                  <ServerConfigPanel
                    serverName={server.name}
                    onDeleted={onDeleted}
                    onEdit={onEdit}
                  />
                </div>
              </ScrollArea>
            </Activity>
          </TabsContent>
        </div>
      </Tabs>
    </div>
  )
}
