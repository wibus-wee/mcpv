// Input: Hooks, child components, UI primitives
// Output: ServersPage component - DataGrid layout with detail sheet
// Position: Main page for servers module

import type { ServerDetail, ServerSummary } from '@bindings/mcpd/internal/ui'
import { useNavigate } from '@tanstack/react-router'
import { useSetAtom } from 'jotai'
import { PlusIcon, SearchIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useEffect, useState } from 'react'

import { RefreshButton } from '@/components/custom/refresh-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Spring } from '@/lib/spring'
import { ImportMcpServersSheet } from '@/modules/servers/components/import-mcp-servers-sheet'

import { selectedServerAtom } from './atoms'
import { ServerDetailSheet } from './components/server-detail-sheet'
import { ServerEditSheet } from './components/server-edit-sheet'
import { ServersDataTable } from './components/servers-data-table'
import type { ServerTab } from './constants'
import { useConfigMode, useFilteredServers, useServer, useServers } from './hooks'

interface ServersPageProps {
  initialTab?: ServerTab
  initialServer?: string
}

export function ServersPage({ initialTab = 'overview', initialServer }: ServersPageProps) {
  const navigate = useNavigate()
  const { data: servers, isLoading, mutate } = useServers()
  const { data: configMode } = useConfigMode()
  const { data: selectedServer } = useServer(initialServer || null)
  const setSelectedServer = useSetAtom(selectedServerAtom)

  const [editSheetOpen, setEditSheetOpen] = useState(false)
  const [editingServer, setEditingServer] = useState<ServerDetail | null>(null)
  const [detailSheetOpen, setDetailSheetOpen] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  const isWritable = configMode?.isWritable ?? false
  const serverCount = servers?.length ?? 0

  const filteredServers = useFilteredServers(servers ?? [], searchQuery)

  // Update selectedServerAtom when selectedServer changes
  useEffect(() => {
    setSelectedServer(selectedServer || null)
  }, [selectedServer, setSelectedServer])

  const handleAddServer = () => {
    setEditingServer(null)
    setEditSheetOpen(true)
  }

  const handleEditServer = () => {
    if (selectedServer) {
      setEditingServer(selectedServer as ServerDetail)
      setEditSheetOpen(true)
      setDetailSheetOpen(false)
    }
  }

  const handleRefresh = async () => {
    setIsRefreshing(true)
    try {
      await mutate()
    }
    finally {
      setIsRefreshing(false)
    }
  }

  const handleRowClick = (server: ServerSummary) => {
    navigate({
      to: '/servers',
      search: { tab: initialTab, server: server.name },
    })
    setDetailSheetOpen(true)
  }

  const handleDetailSheetClose = () => {
    setDetailSheetOpen(false)
  }

  const handleDeleted = () => {
    navigate({
      to: '/servers',
      search: { tab: initialTab, server: undefined },
    })
    setDetailSheetOpen(false)
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="px-6 pt-6 pb-4">
        <m.div
          className="flex items-center justify-between"
          initial={{ opacity: 0, y: -8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={Spring.presets.smooth}
        >
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-semibold tracking-tight">Servers</h1>
              {serverCount > 0 && (
                <Badge variant="secondary" size="sm">
                  {serverCount}
                </Badge>
              )}
            </div>
            <p className="text-sm text-muted-foreground">
              Manage MCP servers, tools, and configurations
            </p>
          </div>
          <div className="flex items-center gap-1">
            <Button
              variant="default"
              size="sm"
              onClick={handleAddServer}
              disabled={!isWritable}
            >
              <PlusIcon className="size-4" />
              Add Server
            </Button>
            <ImportMcpServersSheet />
            <RefreshButton
              onClick={handleRefresh}
              isLoading={isRefreshing}
              tooltip="Refresh servers"
            />
          </div>
        </m.div>
      </div>

      <Separator />

      {/* Toolbar */}
      <div className="px-6 py-4 flex items-center gap-3">
        <div className="relative flex-1 max-w-md">
          <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
          <Input
            type="search"
            placeholder="Search servers..."
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="pl-9"
          />
        </div>
        {searchQuery && (
          <Badge variant="secondary" size="sm">
            {filteredServers?.length ?? 0} results
          </Badge>
        )}
      </div>

      <Separator />

      {/* Table */}
      <ScrollArea className="flex-1">
        <div>
          {isLoading ? (
            <div className="space-y-3">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-16 w-full" />
            </div>
          ) : (
            <ServersDataTable
              servers={filteredServers ?? []}
              onRowClick={handleRowClick}
              selectedServerName={initialServer || null}
              canEdit={isWritable}
              onDeleted={handleDeleted}
            />
          )}
        </div>
      </ScrollArea>

      {/* Detail Sheet */}
      <ServerDetailSheet
        open={detailSheetOpen}
        onOpenChange={handleDetailSheetClose}
        onDeleted={handleDeleted}
        onEdit={handleEditServer}
        initialTab={initialTab}
      />

      {/* Edit Sheet */}
      <ServerEditSheet
        open={editSheetOpen}
        onOpenChange={setEditSheetOpen}
        server={editingServer}
        onSaved={() => mutate()}
      />
    </div>
  )
}
