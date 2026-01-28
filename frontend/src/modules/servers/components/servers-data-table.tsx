// Input: ServerSummary array, selection handler
// Output: Data table with sorting, filtering, and row selection
// Position: Main table component for servers page

import type { ServerRuntimeStatus, ServerSummary } from '@bindings/mcpd/internal/ui'
import type { ColumnDef, ColumnFiltersState, SortingState } from '@tanstack/react-table'
import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import {
  ArrowUpDownIcon,
  MoreHorizontalIcon,
  PowerIcon,
  ServerIcon,
  Trash2Icon,
  WrenchIcon,
} from 'lucide-react'
import { memo, useMemo, useState } from 'react'

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
import { useActiveClients } from '@/hooks/use-active-clients'
import { formatDuration, getElapsedMs } from '@/lib/time'
import { cn } from '@/lib/utils'
import { ServerRuntimeIndicator } from '@/modules/servers/components/server-runtime-status'
import { useRuntimeStatus, useServerOperation, useServers, useToolsByServer } from '@/modules/servers/hooks'
import type { ServerRuntimeState } from '@/modules/shared/server-status'
import { ACTIVE_INSTANCE_STATES, hasActiveInstance } from '@/modules/shared/server-status'

interface ServersDataTableProps {
  servers: ServerSummary[]
  onRowClick: (server: ServerSummary) => void
  selectedServerName: string | null
  canEdit: boolean
  onDeleted?: (serverName: string) => void
}

const StatusCell = memo(function StatusCell({ status }: { status: ServerRuntimeStatus | undefined }) {
  // Determine overall state from instances
  const hasActive = hasActiveInstance(status?.instances)
  const state = hasActive ? 'running' : (status?.instances?.length ?? 0) > 0 ? 'idle' : 'stopped'

  return (
    <div className="flex items-center gap-2">
      {status?.specKey && <ServerRuntimeIndicator specKey={status.specKey} />}
      <span className="text-xs text-muted-foreground capitalize">
        {state}
      </span>
    </div>
  )
})

const LoadCell = memo(function LoadCell({ status }: { status: ServerRuntimeStatus | undefined }) {
  if (!status || !hasActiveInstance(status?.instances)) {
    return <span className="text-xs text-muted-foreground">-</span>
  }

  // Calculate load as busy instances / total instances
  const total = status.stats?.ready + status.stats?.busy || 0
  const busy = status.stats?.busy || 0
  const loadPercent = total > 0 ? Math.round((busy / total) * 100) : 0

  return <span className="text-xs tabular-nums">{loadPercent}%</span>
})

const ClientsCell = memo(function ClientsCell({ count }: { count: number }) {
  return (
    <span className="text-xs tabular-nums">
      {count}
    </span>
  )
})

const UptimeCell = memo(function UptimeCell({ status }: { status: ServerRuntimeStatus | undefined }) {
  if (!status || !status.instances?.length) {
    return <span className="text-xs text-muted-foreground">-</span>
  }

  // Find the oldest running instance
  const activeInstances = status.instances.filter((i: any) => ACTIVE_INSTANCE_STATES.has(i.state as ServerRuntimeState))
  if (activeInstances.length === 0) {
    return <span className="text-xs text-muted-foreground">-</span>
  }

  const oldestInstance = activeInstances.reduce((oldest: any, current: any) => {
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
})

function ToolsCell({ specKey }: { specKey: string }) {
  const { serverMap } = useToolsByServer()

  const toolCount = serverMap.get(specKey)?.tools?.length ?? 0

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
  const [deleteOpen, setDeleteOpen] = useState(false)

  const { isWorking, toggleDisabled, deleteServer } = useServerOperation(
    canEdit,
    mutateServers,
    undefined, // no mutateServer
    onDeleted,
    (title, description) => toastManager.add({ type: 'error', title, description }),
    (title, description) => toastManager.add({ type: 'success', title, description }),
  )

  const handleToggleDisabled = async () => {
    await toggleDisabled(server)
  }

  const handleDeleteServer = async () => {
    await deleteServer(server)
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
          onClick={event => event.stopPropagation()}
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
                  onClick={event => event.stopPropagation()}
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

  const { data: statusList } = useRuntimeStatus()
  const { data: clients } = useActiveClients()

  // Create index Maps - O(n) once
  const statusMap = useMemo(() => {
    const map = new Map<string, ServerRuntimeStatus>()
    statusList?.forEach((status) => {
      if (status.specKey) {
        map.set(status.specKey, status)
      }
    })
    return map
  }, [statusList])

  const clientsMap = useMemo(() => {
    const map = new Map<string, number>()
    clients?.forEach((client) => {
      if (client.server) {
        const count = map.get(client.server) ?? 0
        map.set(client.server, count + 1)
      }
    })
    return map
  }, [clients])

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
                  {row.original.tags.slice(0, 2).map(tag => (
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
        cell: ({ row }) => <StatusCell status={statusMap.get(row.original.specKey)} />,
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
        cell: ({ row }) => <LoadCell status={statusMap.get(row.original.specKey)} />,
      },
      {
        id: 'clients',
        header: 'Clients',
        cell: ({ row }) => <ClientsCell count={clientsMap.get(row.original.name) ?? 0} />,
      },
      {
        id: 'uptime',
        header: 'Uptime',
        cell: ({ row }) => <UptimeCell status={statusMap.get(row.original.specKey)} />,
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
    [canEdit, onDeleted, statusMap, clientsMap],
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
          {table.getHeaderGroups().map(headerGroup => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map(header => (
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
          {table.getRowModel().rows.map(row => (
            <TableRow
              key={row.id}
              onClick={() => onRowClick(row.original)}
              className={cn(
                'cursor-pointer transition-colors',
                selectedServerName === row.original.name
                && 'bg-accent/50 hover:bg-accent/70',
              )}
            >
              {row.getVisibleCells().map(cell => (
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
