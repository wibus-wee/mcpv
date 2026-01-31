import type { PluginListEntry } from '@bindings/mcpv/internal/ui'
import { PlusIcon } from 'lucide-react'
import { useCallback, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

import { PluginEditSheet } from './components/plugin-edit-sheet'
import { PluginListTable } from './components/plugin-list-table'
import { usePluginList } from './hooks'

export function PluginPage() {
  const { data: plugins, isLoading, error, mutate } = usePluginList()
  const [editingPlugin, setEditingPlugin] = useState<PluginListEntry | null>(null)
  const [isSheetOpen, setIsSheetOpen] = useState(false)

  const handleAddPlugin = useCallback(() => {
    setEditingPlugin(null)
    setIsSheetOpen(true)
  }, [])

  const handleEditPlugin = useCallback((plugin: PluginListEntry) => {
    setEditingPlugin(plugin)
    setIsSheetOpen(true)
  }, [])

  const handleSheetOpenChange = useCallback((open: boolean) => {
    setIsSheetOpen(open)
    if (!open) {
      setEditingPlugin(null)
    }
  }, [])

  const handleSaved = useCallback(() => {
    mutate()
  }, [mutate])

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex items-start justify-between">
        <div className="flex flex-col gap-2">
          <h1 className="text-3xl font-bold tracking-tight">Plugins</h1>
          <p className="text-muted-foreground">
            Manage governance plugins for request and response processing
          </p>
        </div>
        <Button onClick={handleAddPlugin}>
          <PlusIcon className="mr-2 size-4" />
          Add Plugin
        </Button>
      </div>

      {error && (
        <Card className="border-destructive/50 bg-destructive/5 p-4">
          <p className="text-destructive text-sm">
            Failed to load plugins:
            {' '}
            {error instanceof Error ? error.message : 'Unknown error'}
          </p>
        </Card>
      )}

      <Card className="p-0">
        <PluginListTable
          plugins={plugins || []}
          isLoading={isLoading}
          onEditPlugin={handleEditPlugin}
        />
      </Card>

      <PluginEditSheet
        open={isSheetOpen}
        onOpenChange={handleSheetOpenChange}
        plugin={editingPlugin}
        onSaved={handleSaved}
      />
    </div>
  )
}
