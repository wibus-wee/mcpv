// Input: ServerSummary array, selection handler
// Output: Data table with sorting, filtering, and row selection
// Position: Main table component for servers page

import type { ServerSummary } from '@bindings/mcpd/internal/ui'
import { ServerService } from '@bindings/mcpd/internal/ui'
import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type ColumnFiltersState,
} from '@tanstack/react-table'
import {
  ArrowUpDownIcon,
  MoreHorizontalIcon,
  PowerIcon,
  ServerIcon,
  Trash2Icon,
  WrenchIcon,
} from 'lucide-react'
import { useMemo, useState } from 'react'

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
import { Button, buttonVariants } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Menu,
  MenuItem,
  MenuPopup,
  MenuTrigger,
} from '@/components/ui/menu'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toastManager } from '@/components/ui/toast'
import { cn } from '@/lib/utils'
import { formatDuration, getElapsedMs } from '@/lib/time'
import { ServerRuntimeIndicator } from '@/modules/config/components/server-runtime-status'
import { useRuntimeStatus, useServers } from '@/modules/config/hooks'
import { useActiveClients } from '@/hooks/use-active-clients'
import { useToolsByServer } from '@/modules/tools/hooks'
import { reloadConfig } from '@/modules/config/lib/reload-config'

interface ServersDataTableProps {
  servers: ServerSummary[]
  onRowClick: (server: ServerSummary) => void
  selectedServerName: string | null
  canEdit: boolean
  onDeleted?: (serverName: string) => void
}

const activeInstanceStates = new Set([
  'ready',
  'busy',
  'starting',
  'initializing',
  'handshaking',
  'draining',
])

function hasActiveInstance(status?: { instances?: { state: string }[] }) {
  return (status?.instances ?? []).some(instance => activeInstanceStates.has(instance.state))
}

function StatusCell({ specKey }: { specKey: string }) {
  const { data: statusList } = useRuntimeStatus()
  const status = statusList?.find(s => s.specKey === specKey)

  // Determine overall state from instances
  const hasActive = hasActiveInstance(status)
  const state = hasActive ? 'running' : (status?.instances?.length ?? 0) > 0 ? 'idle' : 'stopped'

  return (
    <div className="flex items-center gap-2">
      <ServerRuntimeIndicator specKey={specKey} />
      <span className="text-xs text-muted-foreground capitalize">
        {state}
      </span>
    </div>
  )
}

function LoadCell({ specKey }: { specKey: string }) {
  const { data: statusList } = useRuntimeStatus()
  const status = statusList?.find(s => s.specKey === specKey)

  if (!status || !hasActiveInstance(status)) {
    return <span className="text-xs text-muted-foreground">-</span>
  }

  // Calculate load as busy instances / total instances
  const total = status.stats?.ready + status.stats?.busy || 0
  const busy = status.stats?.busy || 0
  const loadPercent = total > 0 ? Math.round((busy / total) * 100) : 0

  return <span className="text-xs tabular-nums">{loadPercent}%</span>
}

function ClientsCell({ serverName }: { serverName: string }) {
  const { data: clients } = useActiveClients()

  if (!clients) {
    return <span className="text-xs text-muted-foreground">-</span>
  }

  const count = clients.filter(c => c.server === serverName).length
  return (
    <span className="text-xs tabular-nums">
      {count}
    </span>
  )
}

function UptimeCell({ specKey }: { specKey: string }) {
  const { data: statusList } = useRuntimeStatus()
  const status = statusList?.find(s => s.specKey === specKey)

  if (!status || !status.instances?.length) {
    return <span className="text-xs text-muted-foreground">-</span>
  }

  // Find the oldest running instance
  const activeInstances = status.instances.filter(i => activeInstanceStates.has(i.state))
  if (activeInstances.length === 0) {
    return <span className="text-xs text-muted-foreground">-</span>
  }

  const oldestInstance = activeInstances.reduce((oldest, current) => {
    return current.spawnedAt < oldest.spawnedAt ? current : oldest
  })

  const elapsed = getElapsedMs(oldestInstance.spawnedAt)
  if (elapsed === null) {
    return <span className="text-xs text-muted-foreground">-</span>
  }
  return (
    <span className="text-xs tabular-nums">
      {formatDuration(elapsed)}
    </span>
  )
}

function ToolsCell({ specKey }: { specKey: string }) {
  const { serverMap } = useToolsByServer()

  // Try to find by specKey in the serverMap
  let toolCount = 0
  for (const server of serverMap.values()) {
    if (server.specKey === specKey) {
      toolCount = server.tools?.length ?? 0
      break
    }
  }

  return (
    <div className="flex items-center gap-2">
      <WrenchIcon className="size-3.5 text-muted-foreground" />
      <span className="text-xs tabular-nums">{toolCount}</span>
    </div>
  )
}

interface ServerActionsCellProps {
  server: ServerSummary
  canEdit: boolean
  onDeleted?: (serverName: string) => void
}

function ServerActionsCell({ server, canEdit, onDeleted }: ServerActionsCellProps) {
  const { mutate: mutateServers } = useServers()
  const [isWorking, setIsWorking] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)

  const handleToggleDisabled = async () => {
    if (!canEdit || isWorking) return
    setIsWorking(true)
    try {
      await ServerService.SetServerDisabled({
        server: server.name,
        disabled: !server.disabled,
      })
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        toastManager.add({
          type: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }
      await mutateServers()
      toastManager.add({
        type: 'success',
        title: server.disabled ? 'Server enabled' : 'Server disabled',
        description: 'Changes applied.',
      })
    } catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Update failed',
        description: err instanceof Error ? err.message : 'Update failed.',
      })
    } finally {
      setIsWorking(false)
    }
  }

  const handleDeleteServer = async () => {
    if (!canEdit || isWorking) return
    setIsWorking(true)
    try {
      await ServerService.DeleteServer({ server: server.name })
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        toastManager.add({
          type: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }
      await mutateServers()
      onDeleted?.(server.name)
      toastManager.add({
        type: 'success',
        title: 'Server deleted',
        description: 'Changes applied.',
      })
    } catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Delete failed',
        description: err instanceof Error ? err.message : 'Delete failed.',
      })
    } finally {
      setIsWorking(false)
    }
  }

  return (
    <div className="flex items-center justify-end gap-2">
      <Button
        variant="secondary"
        size="xs"
        disabled={!canEdit || isWorking}
        onClick={(event) => {
          event.stopPropagation()
          void handleToggleDisabled()
        }}
      >
        <PowerIcon className="size-3.5" />
        {server.disabled ? 'Enable' : 'Disable'}
      </Button>
      <Menu>
        <MenuTrigger
          className={cn(buttonVariants({ variant: 'ghost', size: 'icon-xs' }))}
          disabled={!canEdit || isWorking}
          onClick={(event) => event.stopPropagation()}
        >
          <MoreHorizontalIcon className="size-4" />
        </MenuTrigger>
        <MenuPopup align="end">
          <MenuItem
            variant="destructive"
            onClick={(event) => {
              event.stopPropagation()
              setDeleteOpen(true)
            }}
          >
            <Trash2Icon className="size-4" />
            Delete server
          </MenuItem>
        </MenuPopup>
      </Menu>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete server</AlertDialogTitle>
            <AlertDialogDescription>
              This removes the server from configuration. The change is permanent.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogClose
              render={(
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={(event) => event.stopPropagation()}
                >
                  Cancel
                </Button>
              )}
            />
            <AlertDialogClose
              render={(
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={(event) => {
                    event.stopPropagation()
                    void handleDeleteServer()
                  }}
                  disabled={!canEdit || isWorking}
                >
                  Delete server
                </Button>
              )}
            />
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export function ServersDataTable({
  servers,
  onRowClick,
  selectedServerName,
  canEdit,
  onDeleted,
}: ServersDataTableProps) {
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])

  const columns = useMemo<ColumnDef<ServerSummary>[]>(
    () => [
      {
        accessorKey: 'name',
        header: ({ column }) => {
          return (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
              className="-ml-1 h-8 data-[state=open]:bg-accent"
            >
              Name
              <ArrowUpDownIcon className="ml-2 size-3.5" />
            </Button>
          )
        },
        cell: ({ row }) => {
          return (
            <div className="flex items-center gap-2 ml-2">
              <ServerIcon className="size-3.5 text-muted-foreground shrink-0" />
              <span className="font-medium text-sm">{row.original.name}</span>
              {row.original.tags && row.original.tags.length > 0 && (
                <div className="flex gap-1">
                  {row.original.tags.slice(0, 2).map((tag) => (
                    <Badge key={tag} variant="secondary" size="sm">
                      {tag}
                    </Badge>
                  ))}
                  {row.original.tags.length > 2 && (
                    <Badge variant="secondary" size="sm">
                      +{row.original.tags.length - 2}
                    </Badge>
                  )}
                </div>
              )}
            </div>
          )
        },
      },
      {
        id: 'status',
        header: 'Status',
        cell: ({ row }) => <StatusCell specKey={row.original.specKey} />,
      },
      {
        accessorKey: 'toolCount',
        header: ({ column }) => {
          return (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
              className="-ml-3 h-8 data-[state=open]:bg-accent"
            >
              Tools
              <ArrowUpDownIcon className="ml-2 size-3.5" />
            </Button>
          )
        },
        cell: ({ row }) => <ToolsCell specKey={row.original.specKey} />,
      },
      {
        id: 'load',
        header: 'Load',
        cell: ({ row }) => <LoadCell specKey={row.original.specKey} />,
      },
      {
        id: 'clients',
        header: 'Clients',
        cell: ({ row }) => <ClientsCell serverName={row.original.name} />,
      },
      {
        id: 'uptime',
        header: 'Uptime',
        cell: ({ row }) => <UptimeCell specKey={row.original.specKey} />,
      },
      {
        id: 'actions',
        header: '',
        cell: ({ row }) => (
          <ServerActionsCell
            server={row.original}
            canEdit={canEdit}
            onDeleted={onDeleted}
          />
        ),
      },
    ],
    [canEdit, onDeleted],
  )

  const table = useReactTable({
    data: servers,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    state: {
      sorting,
      columnFilters,
    },
  })

  if (servers.length === 0) {
    return (
      <Empty className="py-16">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <ServerIcon className="size-4" />
          </EmptyMedia>
          <EmptyTitle>No servers</EmptyTitle>
          <EmptyDescription>
            Add MCP servers to start routing tools.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className="rounded-lg">
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <TableHead key={header.id}>
                  {header.isPlaceholder
                    ? null
                    : flexRender(
                      header.column.columnDef.header,
                      header.getContext(),
                    )}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.map((row) => (
            <TableRow
              key={row.id}
              onClick={() => onRowClick(row.original)}
              className={cn(
                'cursor-pointer transition-colors',
                selectedServerName === row.original.name &&
                'bg-accent/50 hover:bg-accent/70',
              )}
            >
              {row.getVisibleCells().map((cell) => (
                <TableCell key={cell.id}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
