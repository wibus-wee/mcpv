// Input: ServerSummary array, selection handler, analytics
// Output: Data table with sorting, filtering, and row selection
// Position: Main table component for servers page

import type { ActiveClient, ServerRuntimeStatus, ServerSummary } from '@bindings/mcpv/internal/ui/types'
import type { ColumnDef, ColumnFiltersState, ExpandedState, SortingState } from '@tanstack/react-table'
import {
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import {
  ArrowUpDownIcon,
  ChevronRightIcon,
  MoreHorizontalIcon,
  PencilIcon,
  PowerIcon,
  ServerIcon,
  Trash2Icon,
  WrenchIcon,
} from 'lucide-react'
import { AnimatePresence, m } from 'motion/react'
import * as React from 'react'
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
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useActiveClients } from '@/hooks/use-active-clients'
import { AnalyticsEvents, track } from '@/lib/analytics'
import { Spring } from '@/lib/spring'
import { formatDuration, getElapsedMs } from '@/lib/time'
import { getToolDisplayName } from '@/lib/tool-names'
import { parseToolJson } from '@/lib/tool-schema'
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
  onEditRequest: (serverName: string) => void
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

const ClientsCell = memo(function ClientsCell({ count, clients }: { count: number, clients: ActiveClient[] }) {
  if (count === 0) {
    return <span className="text-xs tabular-nums text-muted-foreground">-</span>
  }

  return (
    <Tooltip>
      <TooltipTrigger delay={200} render={<div className="text-xs tabular-nums hover:underline cursor-pointer w-full">{count}</div>} />
      <TooltipContent className="w-full">
        <div className="space-y-2">
          <h4 className="font-medium text-sm">Active Clients</h4>
          <div className="space-y-1">
            {clients.map(client => (
              <div key={`${client.client}:${client.pid}`} className="flex items-center justify-between text-xs gap-2">
                <span className="font-mono">{client.client}</span>
                <span className="text-muted-foreground">PID: {client.pid}</span>
              </div>
            ))}
          </div>
        </div>
      </TooltipContent>
    </Tooltip>
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
  onEditRequest: (serverName: string) => void
  onDeleted?: (serverName: string) => void
}

function ServerActionsCell({ server, canEdit, onEditRequest, onDeleted }: ServerActionsCellProps) {
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
        className="w-20"
        size="xs"
        disabled={!canEdit || isWorking}
        onClick={(event) => {
          event.stopPropagation()
          handleToggleDisabled()
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
            onClick={(event) => {
              event.stopPropagation()
              onEditRequest(server.name)
            }}
          >
            <PencilIcon className="size-4" />
            Edit configuration
          </MenuItem>
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
  onEditRequest,
  onDeleted,
}: ServersDataTableProps) {
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [expanded, setExpanded] = useState<ExpandedState>({})

  const { data: statusList } = useRuntimeStatus()
  const { data: clients } = useActiveClients()
  const { serverMap } = useToolsByServer()

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
    const map = new Map<string, ActiveClient[]>()
    clients?.forEach((client) => {
      if (client.server) {
        const existing = map.get(client.server) ?? []
        map.set(client.server, [...existing, client])
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
          const tools = serverMap.get(row.original.specKey)?.tools ?? []
          const canExpand = tools.length > 0

          return (
            <div className="flex items-center gap-2">
              {canExpand && (
                <Button
                  variant="ghost"
                  size="icon-xs"
                  onClick={(e) => {
                    e.stopPropagation()
                    const nextExpanded = !row.getIsExpanded()
                    row.toggleExpanded()
                    track(AnalyticsEvents.SERVER_TOOLS_EXPAND, {
                      expanded: nextExpanded,
                      tool_count: tools.length,
                    })
                  }}
                  className="-ml-1"
                >
                  <ChevronRightIcon
                    className={cn(
                      'size-3.5 transition-transform duration-200',
                      row.getIsExpanded() && 'rotate-90',
                    )}
                  />
                </Button>
              )}
              {!canExpand && <div className="w-7" />}
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
        cell: ({ row }) => {
          const serverClients = clientsMap.get(row.original.name) ?? []
          return <ClientsCell count={serverClients.length} clients={serverClients} />
        },
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
            onEditRequest={onEditRequest}
            onDeleted={onDeleted}
          />
        ),
      },
    ],
    [canEdit, onDeleted, onEditRequest, statusMap, clientsMap, serverMap],
  )

  const table = useReactTable({
    data: servers,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getExpandedRowModel: getExpandedRowModel(),
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onExpandedChange: setExpanded,
    state: {
      sorting,
      columnFilters,
      expanded,
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
          {table.getRowModel().rows.map((row) => {
            const tools = serverMap.get(row.original.specKey)?.tools ?? []
            const hasTools = tools.length > 0

            return (
              <React.Fragment key={row.id}>
                {/* Main Row */}
                <TableRow
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

                {/* Expanded Tools Row */}
                <AnimatePresence>
                  {row.getIsExpanded() && hasTools && (
                    <TableRow className="bg-transparent">
                      <TableCell colSpan={columns.length} className="p-0 overflow-hidden">
                        <m.div
                          initial={{ opacity: 0, height: 0, filter: 'blur(4px)' }}
                          animate={{ opacity: 1, height: 'auto', filter: 'blur(0px)' }}
                          exit={{ opacity: 0, height: 0, filter: 'blur(4px)' }}
                          transition={Spring.smooth(0.4)}
                        >
                          <div className="px-6 py-1">
                            <div className="max-w-4xl">
                              <div>
                                <AnimatePresence mode="popLayout">
                                  {tools.map((tool, index) => {
                                    const schema = parseToolJson(tool)
                                    const paramCount = Object.keys(schema.inputSchema?.properties ?? {}).length
                                    const displayName = getToolDisplayName(tool.name, tool.serverName)

                                    return (
                                      <m.div
                                        key={tool.name}
                                        initial={{ opacity: 0, x: -10 }}
                                        animate={{ opacity: 1, x: 0 }}
                                        exit={{ opacity: 0, x: 10 }}
                                        transition={Spring.smooth(0.15, index * 0.03)}
                                        className="flex items-start gap-3 p-2 rounded-md pl-3"
                                      >

                                        <div className="flex-1 min-w-0">
                                          <div className="flex items-center gap-2 flex-wrap">
                                            <span className="font-mono text-sm font-medium">{displayName}</span>

                                            {paramCount > 0 && (
                                              <Badge variant="secondary" size="sm">
                                                {paramCount}
                                                {' '}
                                                params
                                              </Badge>
                                            )}

                                            {tool.description && (
                                              <span className="text-xs text-muted-foreground">
                                                {tool.description}
                                              </span>
                                            )}

                                            {tool.source === 'cache' && (
                                              <Tooltip>
                                                <TooltipTrigger render={<Badge variant="outline" size="sm">cached</Badge>} />
                                                <TooltipContent>
                                                  {tool.cachedAt ? `Cached at ${new Date(tool.cachedAt).toLocaleString()}` : 'Cached metadata'}
                                                </TooltipContent>
                                              </Tooltip>
                                            )}
                                          </div>
                                        </div>
                                      </m.div>
                                    )
                                  })}
                                </AnimatePresence>
                              </div>
                            </div>
                          </div>
                        </m.div>
                      </TableCell>
                    </TableRow>
                  )}
                </AnimatePresence>
              </React.Fragment>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
