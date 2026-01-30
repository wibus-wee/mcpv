// Input: serverName, open state, callbacks
// Output: Drawer component displaying server details with tabs
// Position: Right-side drawer triggered from table row click

import { LayoutGridIcon, PencilIcon, SettingsIcon, Trash2Icon } from 'lucide-react'
import { m } from 'motion/react'
import { Activity, useState } from 'react'

import {
  AlertDialog,
  AlertDialogClose,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toastManager } from '@/components/ui/toast'
import { Spring } from '@/lib/spring'

import type { ServerTab } from '../constants'
import { useServer, useServerOperation, useServers } from '../hooks'
import { ServerConfigPanel } from './server-config-panel'
import { ServerOverviewPanel } from './server-overview-panel'
import { ServerRuntimeIndicator } from './server-runtime-status'

interface ServerDetailDrawerProps {
  serverName: string | null
  open: boolean
  onClose: () => void
  onDeleted?: () => void
  onEditRequest?: (serverName: string) => void
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

export function ServerDetailDrawer({
  serverName,
  open,
  onClose,
  onDeleted,
  onEditRequest,
}: ServerDetailDrawerProps) {
  const { data: server, isLoading, mutate: mutateServer } = useServer(serverName)
  const { mutate: mutateServers } = useServers()
  // const { serverMap } = useToolsByServer()
  const [tab, setTab] = useState<ServerTab>('overview')
  const [deleteOpen, setDeleteOpen] = useState(false)

  // const toolCount = server ? (serverMap.get(server.specKey)?.tools?.length ?? 0) : 0

  const { isWorking, deleteServer } = useServerOperation(
    true, // canEdit
    mutateServers,
    mutateServer,
    () => {
      onDeleted?.()
      onClose()
    },
    (title, description) => toastManager.add({ type: 'error', title, description }),
    (title, description) => toastManager.add({ type: 'success', title, description }),
  )

  const handleEdit = () => {
    if (server) {
      onEditRequest?.(server.name)
    }
  }

  const handleDelete = async () => {
    if (server) {
      await deleteServer(server)
    }
  }

  const handleDeleted = () => {
    onDeleted?.()
    onClose()
  }

  return (
    <>
      <Sheet
        open={open}
        onOpenChange={(open) => {
          if (!open) onClose()
        }}
      >
        <SheetContent side="right" className="min-w-[50vw] p-0 flex flex-col">
          {/* Header */}
          <SheetHeader className="px-6 pt-6 pb-4 space-y-3">
            {isLoading || !server
              ? (
                  <>
                    <Skeleton className="h-7 w-48" />
                    <Skeleton className="h-4 w-32" />
                  </>
                )
              : (
                  <m.div
                    initial={{ opacity: 0, y: -8 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={Spring.smooth(0.2)}
                  >
                    <SheetTitle className="text-xl font-semibold">
                      {server.name}
                    </SheetTitle>
                    <div className="flex items-center gap-2 mt-2">
                      <ServerRuntimeIndicator specKey={server.specKey} />
                      <Badge variant="outline" size="sm">
                        {server.transport}
                      </Badge>
                      {server.disabled && (
                        <Badge variant="secondary" size="sm">
                          Disabled
                        </Badge>
                      )}
                    </div>
                  </m.div>
                )}
          </SheetHeader>

          {/* Tabs */}
          {isLoading || !server
            ? (
                <DetailSkeleton />
              )
            : (
                <Tabs
                  value={tab}
                  onValueChange={v => setTab(v as ServerTab)}
                  className="flex-1 flex flex-col min-h-0"
                >
                  <TabsList variant="underline" className="px-6 border-b w-full">
                    <TabsTrigger value="overview">
                      <LayoutGridIcon className="size-4" />
                      Overview
                    </TabsTrigger>
                    <TabsTrigger value="configuration">
                      <SettingsIcon className="size-4" />
                      Configuration
                    </TabsTrigger>
                  </TabsList>

                  <div className="flex-1 min-h-0 overflow-hidden">
                    <TabsContent value="overview" keepMounted className="m-0 p-0 h-full">
                      <Activity mode={tab === 'overview' ? 'visible' : 'hidden'}>
                        <ScrollArea className="h-full">
                          <div className="p-6">
                            <ServerOverviewPanel server={server} />
                          </div>
                        </ScrollArea>
                      </Activity>
                    </TabsContent>

                    <TabsContent value="configuration" keepMounted className="m-0 p-0 h-full">
                      <Activity mode={tab === 'configuration' ? 'visible' : 'hidden'}>
                        <ScrollArea className="h-full">
                          <div className="p-6">
                            <ServerConfigPanel
                              serverName={server.name}
                              onDeleted={handleDeleted}
                              onEdit={handleEdit}
                            />
                          </div>
                        </ScrollArea>
                      </Activity>
                    </TabsContent>
                  </div>
                </Tabs>
              )}

          {/* Footer with actions */}
          {!isLoading && server && (
            <SheetFooter className="px-6 py-4 border-t flex-row justify-end gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={handleEdit}
              >
                <PencilIcon className="size-4" />
                Edit
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setDeleteOpen(true)}
                disabled={isWorking}
              >
                <Trash2Icon className="size-4" />
                Delete
              </Button>
            </SheetFooter>
          )}
        </SheetContent>
      </Sheet>

      {/* Delete Dialog */}
      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete server</AlertDialogTitle>
            <AlertDialogDescription>
              This removes
              {' '}
              <strong>{server?.name}</strong>
              {' '}
              from configuration. The change is permanent.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogClose
              render={(
                <Button variant="ghost" size="sm">
                  Cancel
                </Button>
              )}
            />
            <AlertDialogClose
              render={(
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleDelete}
                  disabled={isWorking}
                >
                  Delete server
                </Button>
              )}
            />
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
