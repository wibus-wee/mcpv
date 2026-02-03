// Input: Hooks, child components, UI primitives, analytics
// Output: PluginPage component - Full-width table with right-side drawer
// Position: Main page for plugins module

import { m } from 'motion/react'
import { useCallback, useEffect, useRef, useState } from 'react'

import { ExperimentalBanner } from '@/components/common/experimental-banner'
import { RefreshButton } from '@/components/custom/refresh-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { AnalyticsEvents, track } from '@/lib/analytics'
import { Spring } from '@/lib/spring'

import { PluginEditSheet } from './components/plugin-edit-sheet'
import { PluginListTable } from './components/plugin-list-table'
import { useFilteredPlugins, usePluginList } from './hooks'

export function PluginPage() {
  const { data: plugins, mutate } = usePluginList()

  const [editSheetOpen, setEditSheetOpen] = useState(false)
  const [editingPluginName, setEditingPluginName] = useState<string | null>(null)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const searchTrackTimerRef = useRef<number | null>(null)
  const lastTrackedQueryRef = useRef<string>('')

  const pluginCount = plugins?.length ?? 0
  const filteredPlugins = useFilteredPlugins(plugins ?? [], searchQuery)

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
      track(AnalyticsEvents.PLUGIN_SEARCH, {
        query_len: query.length,
        has_query: query.length > 0,
        result_count: filteredPlugins.length,
      })
    }, 400)
  }, [searchQuery, filteredPlugins.length])

  // Find the editing plugin from the list
  const editingPlugin = editingPluginName
    ? plugins?.find(p => p.name === editingPluginName) ?? null
    : null

  const handleAddPlugin = useCallback(() => {
    track(AnalyticsEvents.PLUGIN_EDIT_OPENED, { mode: 'create' })
    setEditingPluginName(null)
    setEditSheetOpen(true)
  }, [])

  const handleEditRequest = useCallback((pluginName: string) => {
    track(AnalyticsEvents.PLUGIN_EDIT_OPENED, { mode: 'edit' })
    setEditingPluginName(pluginName)
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
                <h1 className="text-xl font-semibold tracking-tight">Plugins</h1>
                {pluginCount > 0 && (
                  <Badge variant="secondary" size="sm">
                    {pluginCount}
                  </Badge>
                )}
              </div>
              <p className="text-sm text-muted-foreground">
                Manage governance plugins for request and response processing
              </p>
            </div>

            <div className="flex-1 max-w-md ml-6">
              <Input
                type="search"
                placeholder="Search plugins..."
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
              />
            </div>
          </div>

          <div className="flex items-center gap-1">
            <Button
              variant="default"
              size="sm"
              onClick={handleAddPlugin}
            >
              Add Plugin
            </Button>
            <RefreshButton
              onClick={handleRefresh}
              isLoading={isRefreshing}
              tooltip="Refresh plugins"
            />
          </div>
        </m.div>
      </div>

      <div className="px-4 py-3 pt-0">
        <ExperimentalBanner
          feature="Feature"
          description="Plugins are currently under very early design and development. Your feedback is welcome!"
        />
      </div>

      <Separator />

      {/* Table */}
      <div className="flex-1 min-h-0 overflow-auto">
        <PluginListTable
          plugins={filteredPlugins ?? []}
          onEditRequest={handleEditRequest}
        />
      </div>

      {/* Edit Sheet */}
      <PluginEditSheet
        open={editSheetOpen}
        onOpenChange={(open) => {
          setEditSheetOpen(open)
          if (!open) {
            setEditingPluginName(null)
          }
        }}
        plugin={editingPlugin}
        editTargetName={editingPluginName}
        onSaved={() => mutate()}
      />
    </div>
  )
}
