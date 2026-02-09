// Input: Hooks, child components, UI primitives, analytics
// Output: ServersPage component - Full-width table with expandable rows and right-side drawer
// Position: Main page for servers module

import type { ServerSummary } from '@bindings/mcpv/internal/ui/types'
import { m } from 'motion/react'
import { useCallback, useEffect, useRef, useState } from 'react'

import { RefreshButton } from '@/components/custom/refresh-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { AnalyticsEvents, track } from '@/lib/analytics'
import { Spring } from '@/lib/spring'
import { ImportMcpServersSheet } from '@/modules/servers/components/import-mcp-servers-sheet'

import { ServerDetailDrawer } from './components/server-detail-drawer'
import { ServerEditSheet } from './components/server-edit-sheet'
import { ServersDataTable } from './components/servers-data-table'
import { useConfigMode, useFilteredServers, useServer, useServers } from './hooks'

export function ServersPage() {
  const { data: servers, mutate } = useServers()
  const { data: configMode } = useConfigMode()

  const [selectedServer, setSelectedServer] = useState<string | null>(null)
  const [editSheetOpen, setEditSheetOpen] = useState(false)
  const [editingServerName, setEditingServerName] = useState<string | null>(null)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const searchTrackTimerRef = useRef<number | null>(null)
  const lastTrackedQueryRef = useRef<string>('')

  const {
    data: editingServer,
    isLoading: isEditingServerLoading,
  } = useServer(editingServerName)

  const isWritable = configMode?.isWritable ?? false
  const serverCount = servers?.length ?? 0

  const filteredServers = useFilteredServers(servers ?? [], searchQuery)

  useEffect(() => {
    return () => {
      if (searchTrackTimerRef.current !== null) {
        window.clearTimeout(searchTrackTimerRef.current)
        searchTrackTimerRef.current = null
      }
    }
  }, [])

  useEffect(() => {
    if (searchTrackTimerRef.current !== null) {
      window.clearTimeout(searchTrackTimerRef.current)
    }
    searchTrackTimerRef.current = window.setTimeout(() => {
      const query = searchQuery.trim()
      if (query === lastTrackedQueryRef.current) return
      lastTrackedQueryRef.current = query
      track(AnalyticsEvents.SERVER_SEARCH, {
        query_len: query.length,
        has_query: query.length > 0,
        result_count: filteredServers.length,
      })
    }, 400)
  }, [searchQuery, filteredServers.length])

  const handleAddServer = useCallback(() => {
    track(AnalyticsEvents.SERVER_EDIT_OPENED, { mode: 'create' })
    setEditingServerName(null)
    setEditSheetOpen(true)
  }, [])

  const handleEditRequest = useCallback((serverName: string) => {
    track(AnalyticsEvents.SERVER_EDIT_OPENED, { mode: 'edit' })
    setEditingServerName(serverName)
    setEditSheetOpen(true)
  }, [])

  const handleRefresh = useCallback(async () => {
    setIsRefreshing(true)
    try {
      await mutate()
    }
    finally {
      setIsRefreshing(false)
    }
  }, [mutate])

  const handleRowClick = useCallback((server: ServerSummary) => {
    track(AnalyticsEvents.SERVER_DETAIL_OPENED, {
      transport: server.transport ?? 'unknown',
      disabled: Boolean(server.disabled),
      tags_count: server.tags?.length ?? 0,
    })
    setSelectedServer(server.name)
  }, [])

  const handleDrawerClose = useCallback(() => {
    setSelectedServer(null)
  }, [])

  const handleDeleted = useCallback(() => {
    setSelectedServer(null)
  }, [])

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="px-6 pt-6 pb-4">
        <m.div
          className="flex items-center justify-between gap-6"
          initial={{ opacity: 0, y: -8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={Spring.presets.smooth}
        >
          <div className="flex items-center gap-4 flex-1">
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

            <div className="flex-1 max-w-md ml-6">
              <Input
                type="search"
                placeholder="Search servers..."
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
              />
            </div>
          </div>

          <div className="flex items-center gap-1">
            <Button
              variant="default"
              size="sm"
              onClick={handleAddServer}
              disabled={!isWritable}
            >
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

      {/* Table */}
      <div className="flex-1 min-h-0 overflow-auto">
        <ServersDataTable
          servers={filteredServers ?? []}
          onRowClick={handleRowClick}
          selectedServerName={selectedServer}
          canEdit={isWritable}
          onEditRequest={handleEditRequest}
          onDeleted={handleDeleted}
        />
      </div>

      {/* Drawer */}
      <ServerDetailDrawer
        serverName={selectedServer}
        open={!!selectedServer}
        onClose={handleDrawerClose}
        onDeleted={handleDeleted}
        onEditRequest={handleEditRequest}
      />

      {/* Edit Sheet - Single source of truth */}
      <ServerEditSheet
        open={editSheetOpen}
        onOpenChange={(open) => {
          setEditSheetOpen(open)
          if (!open) {
            setEditingServerName(null)
          }
        }}
        server={editingServer}
        editTargetName={editingServerName}
        isLoading={isEditingServerLoading}
        onSaved={() => mutate()}
      />
    </div>
  )
}
