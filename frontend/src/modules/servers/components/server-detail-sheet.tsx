// Input: ServerSummary, open state, tab content components
// Output: 60% width sheet with tabs for Overview/Tools/Config
// Position: Detail panel for selected server

import { LayoutGridIcon, SettingsIcon, WrenchIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useAtom } from 'jotai'
import { useNavigate } from '@tanstack/react-router'

import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Sheet, SheetContent, SheetHeader } from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Spring } from '@/lib/spring'
import { useToolsByServer } from '@/modules/servers/hooks'

import { selectedServerAtom } from '../atoms'

import { ServerConfigPanel } from './server-config-panel'
import { ServerOverviewPanel } from './server-overview-panel'
import { ServerToolsPanel } from './server-tools-panel'
import type { ServerTab } from '../constants'

interface ServerDetailSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onDeleted?: () => void
  onEdit?: () => void
  initialTab?: ServerTab
}

export function ServerDetailSheet({
  open,
  onOpenChange,
  onDeleted,
  onEdit,
  initialTab = 'overview',
}: ServerDetailSheetProps) {
  const navigate = useNavigate({ from: '/servers' })
  const { serverMap } = useToolsByServer()
  const [selectedServer] = useAtom(selectedServerAtom)

  const handleTabChange = (value: string) => {
    navigate({
      search: (prev) => ({ ...prev, tab: value as ServerTab }),
      replace: true,
    })
  }

  if (!selectedServer) return null

  const toolCount = serverMap.get(selectedServer.specKey)?.tools?.length ?? 0

  const handleDelete = () => {
    onDeleted?.()
    onOpenChange(false)
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="w-full p-0 flex flex-col gap-0 max-w-xl"
      >
        {/* Header */}
        <SheetHeader className="px-6 pt-6 pb-4 space-y-0">
          <m.div
            className="flex items-start justify-between"
            initial={{ opacity: 0, y: -8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={Spring.presets.smooth}
          >
            <div className="space-y-2 min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <h2 className="text-lg font-semibold tracking-tight truncate">
                  {selectedServer.name}
                </h2>
                {selectedServer.tags && selectedServer.tags.length > 0 && (
                  <div className="flex gap-1 flex-wrap">
                    {selectedServer.tags.map(tag => (
                      <Badge key={tag} variant="secondary" size="sm">
                        {tag}
                      </Badge>
                    ))}
                  </div>
                )}
              </div>
              <p className="text-sm text-muted-foreground">
                {toolCount} tools available
              </p>
            </div>
          </m.div>
        </SheetHeader>

        <Separator />

        {/* Tabs */}
        <Tabs value={initialTab} onValueChange={handleTabChange} className="flex-1 flex flex-col min-h-0 w-full">
          <TabsList variant="underline" className="px-4 border-b w-full">
            <TabsTrigger value="overview">
              <LayoutGridIcon className="size-4" />
              Overview
            </TabsTrigger>
            <TabsTrigger value="tools">
              <WrenchIcon className="size-4" />
              Tools
            </TabsTrigger>
            <TabsTrigger value="configuration">
              <SettingsIcon className="size-4" />
              Configuration
            </TabsTrigger>
          </TabsList>

          <ScrollArea className="flex-1">
            <TabsContent value="overview" className="m-0 p-0">
              <ServerOverviewPanel
                className="p-6 pt-2"
              />
            </TabsContent>

            <TabsContent value="tools" className="m-0 p-0 h-full">
              <ServerToolsPanel serverName={selectedServer.name} />
            </TabsContent>

            <TabsContent value="configuration" className="m-0 p-0">
              <div className="p-6 pt-2">
                <ServerConfigPanel
                  serverName={selectedServer.name}
                  onDeleted={handleDelete}
                  onEdit={onEdit}
                />
              </div>
            </TabsContent>
          </ScrollArea>
        </Tabs>
      </SheetContent>
    </Sheet>
  )
}
