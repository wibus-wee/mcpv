// Input: Hooks, child components, UI primitives
// Output: PluginPage component - Full-width table with right-side drawer
// Position: Main page for plugins module

import { m } from 'motion/react'
import { useCallback, useState } from 'react'

import { RefreshButton } from '@/components/custom/refresh-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
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

  const pluginCount = plugins?.length ?? 0
  const filteredPlugins = useFilteredPlugins(plugins ?? [], searchQuery)

  // Find the editing plugin from the list
  const editingPlugin = editingPluginName
    ? plugins?.find(p => p.name === editingPluginName) ?? null
    : null

  const handleAddPlugin = useCallback(() => {
    setEditingPluginName(null)
    setEditSheetOpen(true)
  }, [])

  const handleEditRequest = useCallback((pluginName: string) => {
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
