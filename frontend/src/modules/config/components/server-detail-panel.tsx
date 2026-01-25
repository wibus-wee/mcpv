// Input: selected server name, config hooks, runtime status component
// Output: ServerDetailPanel component - detail view for a server
// Position: Right panel in config page master-detail layout

import type { ServerDetail } from '@bindings/mcpd/internal/ui'
import { ServerService } from '@bindings/mcpd/internal/ui'
import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  PowerIcon,
  ServerIcon,
  Trash2Icon,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  AlertDialog,
  AlertDialogClose,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { useConfigMode, useServer, useServers } from '../hooks'
import { reloadConfig } from '../lib/reload-config'
import { ServerRuntimeSummary } from './server-runtime-status'

interface ServerDetailPanelProps {
  serverName: string | null
  onDeleted?: () => void
}

type NoticeState = {
  variant: 'success' | 'error'
  title: string
  description: string
}

function DetailRow({
  label,
  value,
  mono,
}: {
  label: string
  value?: React.ReactNode
  mono?: boolean
}) {
  return (
    <div className="flex items-start justify-between gap-4 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <div className={cn('text-right', mono && 'font-mono text-xs')}>{value}</div>
    </div>
  )
}

function DetailSkeleton() {
  return (
    <div className="space-y-4 p-6">
      <Skeleton className="h-6 w-48" />
      <Skeleton className="h-4 w-64" />
      <Skeleton className="h-36 w-full" />
      <Skeleton className="h-36 w-full" />
    </div>
  )
}

function buildCommandSummary(server: ServerDetail) {
  if (server.transport === 'streamable_http') {
    return server.http?.endpoint ?? '--'
  }
  return server.cmd.join(' ')
}

export function ServerDetailPanel({ serverName, onDeleted }: ServerDetailPanelProps) {
  const { data: server, isLoading, mutate: mutateServer } = useServer(serverName)
  const { mutate: mutateServers } = useServers()
  const { data: configMode } = useConfigMode()
  const [notice, setNotice] = useState<NoticeState | null>(null)
  const [isWorking, setIsWorking] = useState(false)

  useEffect(() => {
    setNotice(null)
    setIsWorking(false)
  }, [serverName])

  const canEdit = Boolean(configMode?.isWritable)

  const handleToggleDisabled = useCallback(async () => {
    if (!server || isWorking || !canEdit) {
      return
    }
    setIsWorking(true)
    setNotice(null)

    try {
      await ServerService.SetServerDisabled({
        server: server.name,
        disabled: !server.disabled,
      })
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setNotice({
          variant: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }
      await Promise.all([mutateServer(), mutateServers()])
      setNotice({
        variant: 'success',
        title: 'Saved',
        description: 'Changes applied.',
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Update failed.'
      setNotice({
        variant: 'error',
        title: 'Update failed',
        description: message,
      })
    } finally {
      setIsWorking(false)
    }
  }, [server, isWorking, canEdit, mutateServer, mutateServers])

  const handleDeleteServer = useCallback(async () => {
    if (!server || isWorking || !canEdit) {
      return
    }
    setIsWorking(true)
    setNotice(null)

    try {
      await ServerService.DeleteServer({ server: server.name })
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setNotice({
          variant: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }
      await Promise.all([mutateServer(), mutateServers()])
      setNotice({
        variant: 'success',
        title: 'Server deleted',
        description: 'Changes applied.',
      })
      onDeleted?.()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Delete failed.'
      setNotice({
        variant: 'error',
        title: 'Delete failed',
        description: message,
      })
    } finally {
      setIsWorking(false)
    }
  }, [server, isWorking, canEdit, mutateServer, mutateServers, onDeleted])

  const tags = server?.tags ?? []
  const commandSummary = server ? buildCommandSummary(server) : '--'
  const envCount = server ? Object.keys(server.env ?? {}).length : 0

  const statusBadge = useMemo(() => {
    if (!server) return null
    return server.disabled
      ? { label: 'Disabled', variant: 'warning' as const }
      : { label: 'Enabled', variant: 'success' as const }
  }, [server])

  if (!serverName) {
    return (
      <Empty className="py-16">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <ServerIcon className="size-4" />
          </EmptyMedia>
          <EmptyTitle className="text-sm">Select a server</EmptyTitle>
          <EmptyDescription className="text-xs">
            Choose a server from the list to inspect its configuration.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  if (isLoading) {
    return <DetailSkeleton />
  }

  if (!server) {
    return (
      <Empty className="py-16">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <AlertTriangleIcon className="size-4" />
          </EmptyMedia>
          <EmptyTitle className="text-sm">Server not found</EmptyTitle>
          <EmptyDescription className="text-xs">
            The selected server could not be loaded.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className="space-y-4 p-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-lg font-semibold">{server.name}</h2>
            {statusBadge && (
              <Badge variant={statusBadge.variant} size="sm">
                {statusBadge.label}
              </Badge>
            )}
            <Badge variant="outline" size="sm" className="uppercase">
              {server.transport}
            </Badge>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {tags.length === 0 ? (
              <span className="text-xs text-muted-foreground">No tags</span>
            ) : (
              tags.map(tag => (
                <Badge key={tag} variant="secondary" size="sm">
                  {tag}
                </Badge>
              ))
            )}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={handleToggleDisabled}
            disabled={!canEdit || isWorking}
            className="gap-2"
          >
            <PowerIcon className="size-4" />
            {server.disabled ? 'Enable' : 'Disable'}
          </Button>
          <AlertDialog>
            <AlertDialogTrigger
              render={(
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={!canEdit || isWorking}
                  className="gap-2 text-destructive"
                >
                  <Trash2Icon className="size-4" />
                  Delete
                </Button>
              )}
            />
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
                      onClick={handleDeleteServer}
                    >
                      Delete server
                    </Button>
                  )}
                />
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      {notice && (
        <Alert variant={notice.variant}>
          {notice.variant === 'success' ? <CheckCircle2Icon /> : <AlertTriangleIcon />}
          <AlertTitle>{notice.title}</AlertTitle>
          <AlertDescription>{notice.description}</AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Specification</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <DetailRow label="Command" value={commandSummary} mono />
          <DetailRow label="Working directory" value={server.cwd || '--'} mono />
          <DetailRow label="Environment" value={`${envCount} variables`} />
          <DetailRow label="Activation mode" value={server.activationMode || '--'} />
          <DetailRow label="Idle timeout" value={`${server.idleSeconds}s`} />
          <DetailRow label="Max concurrency" value={server.maxConcurrent} />
          <DetailRow label="Session TTL" value={`${server.sessionTTLSeconds}s`} />
          <DetailRow label="Protocol" value={server.protocolVersion || '--'} />
          <DetailRow
            label="Expose tools"
            value={(server.exposeTools ?? []).length > 0
              ? server.exposeTools.join(', ')
              : 'Default'}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Runtime status</CardTitle>
        </CardHeader>
        <CardContent>
          <ServerRuntimeSummary specKey={server.specKey} className="mt-0" />
        </CardContent>
      </Card>
    </div>
  )
}
