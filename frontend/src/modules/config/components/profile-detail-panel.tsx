// Input: profileName prop, useProfile hook, ProfileDetail type, ServerRuntimeIndicator
// Output: ProfileDetailPanel component - inline detail view with minimal visual weight
// Position: Right panel in config page master-detail layout with live runtime status

import type { ActiveCaller, ProfileDetail, ServerSpecDetail } from '@bindings/mcpd/internal/ui'
import { WailsService } from '@bindings/mcpd/internal/ui'
import {
  AlertCircleIcon,
  CheckCircleIcon,
  ClockIcon,
  CpuIcon,
  LayersIcon,
  NetworkIcon,
  ServerIcon,
  SettingsIcon,
  TerminalIcon,
  TrashIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useEffect, useState } from 'react'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { CallerChipGroup } from '@/components/common/caller-chip-group'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useActiveCallers } from '@/hooks/use-active-callers'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

import { useConfigMode, useProfile, useProfiles } from '../hooks'
import { reloadConfig } from '../lib/reload-config'
import { ServerRuntimeIndicator, ServerRuntimeSummary } from './server-runtime-status'

interface ProfileDetailPanelProps {
  profileName: string | null
}

type ServerSpecWithKey = ServerSpecDetail & { specKey?: string }

const defaultProfileName = 'default'

type NoticeState = {
  variant: 'success' | 'error'
  title: string
  description: string
}

function DetailRow({
  label,
  value,
  mono = false,
}: {
  label: string
  value: React.ReactNode
  mono?: boolean
}) {
  return (
    <div className="flex items-center justify-between py-1.5">
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className={mono ? 'font-mono text-xs' : 'text-xs'}>{value}</span>
    </div>
  )
}

function RuntimeSection({ profile }: { profile: ProfileDetail }) {
  const { runtime } = profile

  return (
    <AccordionItem value="runtime" className="border-none">
      <AccordionTrigger className="py-2 hover:no-underline">
        <div className="flex items-center gap-2">
          <SettingsIcon className="size-3.5 text-muted-foreground" />
          <span className="text-sm font-medium">Runtime Configuration</span>
        </div>
      </AccordionTrigger>
      <AccordionContent className="pb-0">
        <div className="divide-y divide-border/50 pb-3">
          <DetailRow label="Route Timeout" value={`${runtime.routeTimeoutSeconds}s`} mono />
          <DetailRow label="Ping Interval" value={`${runtime.pingIntervalSeconds}s`} mono />
          <DetailRow label="Tool Refresh" value={`${runtime.toolRefreshSeconds}s`} mono />
          <DetailRow label="Caller Check" value={`${runtime.callerCheckSeconds}s`} mono />
          <DetailRow label="Init Retry Base" value={`${runtime.serverInitRetryBaseSeconds}s`} mono />
          <DetailRow label="Init Retry Max" value={`${runtime.serverInitRetryMaxSeconds}s`} mono />
          <DetailRow label="Init Max Retries" value={`${runtime.serverInitMaxRetries}`} mono />
          <DetailRow
            label="Expose Tools"
            value={
              <Badge variant={runtime.exposeTools ? 'success' : 'secondary'} size="sm">
                {runtime.exposeTools ? 'Yes' : 'No'}
              </Badge>
            }
          />
          <DetailRow
            label="Namespace Strategy"
            value={
              <Badge variant="outline" size="sm">
                {runtime.toolNamespaceStrategy || 'prefix'}
              </Badge>
            }
          />
        </div>

        <div className="border-t pt-3 pb-2">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground mb-2">
            <NetworkIcon className="size-3" />
            RPC Configuration
          </div>
          <div className="divide-y divide-border/50">
            <DetailRow
              label="Listen Address"
              value={
                <span className="font-mono text-xs truncate max-w-40 block text-right">
                  {runtime.rpc.listenAddress}
                </span>
              }
            />
            <DetailRow label="Socket Mode" value={runtime.rpc.socketMode || '0660'} mono />
          </div>
        </div>
      </AccordionContent>
    </AccordionItem>
  )
}

function SubAgentSection({
  profile,
  canEdit,
  onToggle
}: {
  profile: ProfileDetail
  canEdit: boolean
  onToggle: (enabled: boolean) => void
}) {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const isEnabled = profile.subAgent?.enabled ?? false

  const handleToggle = async (checked: boolean) => {
    setIsLoading(true)
    setError(null)

    try {
      await WailsService.SetProfileSubAgentEnabled({
        profile: profile.name,
        enabled: checked,
      })
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setError(reloadResult.message)
        return
      }
      onToggle(checked)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update SubAgent')
      console.error('Failed to toggle SubAgent:', err)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AccordionItem value="subagent" className="border-none">
      <AccordionTrigger className="py-2 hover:no-underline">
        <div className="flex items-center gap-2">
          <CpuIcon className="size-3.5 text-muted-foreground" />
          <span className="text-sm font-medium">SubAgent</span>
          <Badge variant={isEnabled ? 'success' : 'secondary'} size="sm">
            {isEnabled ? 'Enabled' : 'Disabled'}
          </Badge>
        </div>
      </AccordionTrigger>
      <AccordionContent className="pb-3">
        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-2.5 py-2">
            <div className="flex items-center gap-2">
              <Switch
                checked={isEnabled}
                onCheckedChange={handleToggle}
                disabled={!canEdit || isLoading}
              />
              <span className="text-xs text-muted-foreground">
                LLM-based tool filtering
              </span>
            </div>
          </div>

          {error && (
            <Alert variant="destructive" className="py-2">
              <AlertCircleIcon className="size-3.5" />
              <AlertDescription className="text-xs">{error}</AlertDescription>
            </Alert>
          )}

          {isEnabled && (
            <div className="text-xs text-muted-foreground border-l-2 border-primary pl-2 space-y-1">
              <p>When enabled, exposes only:</p>
              <ul className="list-disc list-inside space-y-0.5 ml-1">
                <li><code className="text-xs">mcpd.automatic_mcp</code></li>
                <li><code className="text-xs">mcpd.automatic_eval</code></li>
              </ul>
            </div>
          )}
        </div>
      </AccordionContent>
    </AccordionItem>
  )
}

function ServerItem({
  server,
  canEdit,
  isBusy,
  disabledHint,
  onToggleDisabled,
  onDelete,
}: {
  server: ServerSpecWithKey
  canEdit: boolean
  isBusy: boolean
  disabledHint?: string
  onToggleDisabled: (server: ServerSpecWithKey, disabled: boolean) => void
  onDelete: (server: ServerSpecWithKey) => void
}) {
  const specKey = server.specKey ?? server.name
  const isDisabled = Boolean(server.disabled)

  return (
    <AccordionItem
      value={`server-${server.name}`}
      className={cn('border-none', isDisabled && 'opacity-70')}
    >
      <AccordionTrigger className="py-2 hover:no-underline">
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <span className="font-mono text-sm truncate">{server.name}</span>
          <div className="flex items-center gap-1.5 ml-auto mr-2">
            <ServerRuntimeIndicator specKey={specKey} />
            {isDisabled && (
              <Badge variant="warning" size="sm">Disabled</Badge>
            )}
            {server.persistent && (
              <Badge variant="secondary" size="sm">Persistent</Badge>
            )}
            {server.sticky && (
              <Badge variant="outline" size="sm">Sticky</Badge>
            )}
          </div>
        </div>
      </AccordionTrigger>
      <AccordionContent className="pb-3">
        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-2.5 py-2">
            <div className="flex items-center gap-2">
              <Switch
                checked={!isDisabled}
                onCheckedChange={checked => onToggleDisabled(server, !checked)}
                disabled={!canEdit || isBusy}
                title={disabledHint}
              />
              <span className="text-xs text-muted-foreground">
                {isDisabled ? 'Disabled' : 'Enabled'}
              </span>
            </div>
            <AlertDialog>
              <AlertDialogTrigger
                disabled={!canEdit || isBusy}
                render={(
                  <Button
                    variant="ghost"
                    size="xs"
                    title={disabledHint}
                  >
                    <TrashIcon className="size-3.5" />
                    Delete
                  </Button>
                )}
              />
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete server</AlertDialogTitle>
                  <AlertDialogDescription>
                    This removes the server from the profile configuration.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogClose
                    render={<Button variant="ghost">Cancel</Button>}
                  />
                  <AlertDialogClose
                    render={(
                      <Button
                        variant="destructive"
                        onClick={() => onDelete(server)}
                      >
                        Delete server
                      </Button>
                    )}
                  />
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>

          <ServerRuntimeSummary
            specKey={specKey}
            className="rounded-lg border bg-muted/10 px-2.5 py-2"
          />

          {/* Command */}
          <div>
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground mb-1.5">
              <TerminalIcon className="size-3" />
              Command
            </div>
            <div className="bg-muted/40 rounded px-2.5 py-2 font-mono text-xs overflow-x-auto">
              {server.cmd.join(' ')}
            </div>
          </div>

          {/* Working Directory */}
          {server.cwd && (
            <div>
              <p className="text-muted-foreground text-xs mb-1">Working Directory</p>
              <p className="font-mono text-xs">{server.cwd}</p>
            </div>
          )}

          {/* Environment Variables */}
          {Object.keys(server.env).length > 0 && (
            <div>
              <p className="text-muted-foreground text-xs mb-1.5">Environment Variables</p>
              <div className="space-y-1">
                {Object.entries(server.env).map(([key, value]) => (
                  <div key={key} className="flex items-center gap-2 text-xs">
                    <Badge variant="outline" size="sm" className="font-mono shrink-0">
                      {key}
                    </Badge>
                    <span className="text-muted-foreground truncate">
                      {value.startsWith('${') ? value : '••••••'}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          <Separator className="my-2" />

          {/* Settings Grid */}
          <div className="grid grid-cols-3 gap-3 text-xs">
            <div className="flex items-center gap-1.5">
              <ClockIcon className="size-3 text-muted-foreground" />
              <div>
                <p className="text-muted-foreground">Idle</p>
                <p className="font-mono">{server.idleSeconds}s</p>
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              <CpuIcon className="size-3 text-muted-foreground" />
              <div>
                <p className="text-muted-foreground">Max</p>
                <p className="font-mono">{server.maxConcurrent}</p>
              </div>
            </div>
            <div>
              <p className="text-muted-foreground">Min Ready</p>
              <p className="font-mono">{server.minReady}</p>
            </div>
          </div>

          {/* Exposed Tools */}
          {server.exposeTools && server.exposeTools.length > 0 && (
            <div>
              <p className="text-muted-foreground text-xs mb-1.5">Exposed Tools</p>
              <div className="flex flex-wrap gap-1">
                {server.exposeTools.map(tool => (
                  <Badge key={tool} variant="secondary" size="sm" className="font-mono">
                    {tool}
                  </Badge>
                ))}
              </div>
            </div>
          )}
        </div>
      </AccordionContent>
    </AccordionItem>
  )
}

function ProfileContent({
  profile,
  activeCallers,
  canEditServers,
  canDeleteProfile,
  serverActionHint,
  pendingServerName,
  deletingProfile,
  notice,
  onDismissNotice,
  onSubAgentToggle,
  onToggleDisabled,
  onDeleteServer,
  onDeleteProfile,
}: {
  profile: ProfileDetail
  activeCallers: ActiveCaller[]
  canEditServers: boolean
  canDeleteProfile: boolean
  serverActionHint?: string
  pendingServerName: string | null
  deletingProfile: boolean
  notice: NoticeState | null
  onDismissNotice: () => void
  onSubAgentToggle: (enabled: boolean) => void
  onToggleDisabled: (server: ServerSpecWithKey, disabled: boolean) => void
  onDeleteServer: (server: ServerSpecWithKey) => void
  onDeleteProfile: () => void
}) {
  return (
    <m.div
      className="space-y-4"
      initial={{ opacity: 0, x: 8 }}
      animate={{ opacity: 1, x: 0 }}
      transition={Spring.smooth(0.25)}
    >
      {notice && (
        <Alert variant={notice.variant}>
          {notice.variant === 'success' ? <CheckCircleIcon /> : <AlertCircleIcon />}
          <AlertTitle>{notice.title}</AlertTitle>
          <AlertDescription>{notice.description}</AlertDescription>
          <AlertAction>
            <Button variant="ghost" size="xs" onClick={onDismissNotice}>
              Dismiss
            </Button>
          </AlertAction>
        </Alert>
      )}

      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="font-semibold">{profile.name}</h2>
        <div className="flex items-center gap-2">
          <Badge variant="secondary" size="sm">
            {profile.servers.length} server{profile.servers.length !== 1 ? 's' : ''}
          </Badge>
          <AlertDialog>
            <AlertDialogTrigger
              disabled={!canDeleteProfile || deletingProfile}
              render={(
                <Button
                  variant="destructive-outline"
                  size="xs"
                  title={canDeleteProfile ? undefined : 'Profile deletion is not available'}
                >
                  <TrashIcon className="size-3.5" />
                  Delete profile
                </Button>
              )}
            />
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete profile</AlertDialogTitle>
                <AlertDialogDescription>
                  This removes the profile file and all servers inside it.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogClose
                  render={<Button variant="ghost">Cancel</Button>}
                />
                <AlertDialogClose
                  render={(
                    <Button
                      variant="destructive"
                      onClick={onDeleteProfile}
                    >
                      Delete profile
                    </Button>
                  )}
                />
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      <div className="rounded-lg border bg-muted/30 px-3 py-2">
        <div className="text-xs text-muted-foreground">Active Callers</div>
        <CallerChipGroup
          callers={activeCallers}
          maxVisible={3}
          showPid
          emptyText="No active callers"
          className="mt-1"
        />
      </div>

      {/* Runtime Config */}
      <Accordion multiple defaultValue={['runtime']}>
        <RuntimeSection profile={profile} />
        <SubAgentSection
          profile={profile}
          canEdit={canEditServers}
          onToggle={onSubAgentToggle}
        />
      </Accordion>

      {/* Servers */}
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <ServerIcon className="size-3.5 text-muted-foreground" />
          <span className="text-sm font-medium">Servers</span>
        </div>

        {profile.servers.length === 0 ? (
          <Empty className="py-6">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <ServerIcon className="size-4" />
              </EmptyMedia>
              <EmptyTitle className="text-sm">No servers</EmptyTitle>
              <EmptyDescription className="text-xs">
                Import servers or update your configuration to get started.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <Accordion multiple>
            {profile.servers.map(server => (
              <ServerItem
                key={server.name}
                server={server}
                canEdit={canEditServers}
                isBusy={pendingServerName === server.name}
                disabledHint={serverActionHint}
                onToggleDisabled={onToggleDisabled}
                onDelete={onDeleteServer}
              />
            ))}
          </Accordion>
        )}
      </div>
    </m.div>
  )
}

function PanelSkeleton() {
  return (
    <div className="space-y-4 p-4">
      <div className="flex items-center justify-between">
        <Skeleton className="h-5 w-32" />
        <Skeleton className="h-5 w-16" />
      </div>
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-24 w-full" />
    </div>
  )
}

function PanelEmpty() {
  return (
    <Empty className="h-full">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <LayersIcon className="size-5" />
        </EmptyMedia>
        <EmptyTitle>Select a profile</EmptyTitle>
        <EmptyDescription>
          Choose a profile from the list to view its details.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function ProfileDetailPanel({ profileName }: ProfileDetailPanelProps) {
  const { data: profile, isLoading, mutate: mutateProfile } = useProfile(profileName)
  const { mutate: mutateProfiles } = useProfiles()
  const { data: configMode } = useConfigMode()
  const { data: activeCallers } = useActiveCallers()
  const [notice, setNotice] = useState<NoticeState | null>(null)
  const [pendingServerName, setPendingServerName] = useState<string | null>(null)
  const [deletingProfile, setDeletingProfile] = useState(false)
  const profileCallers = (activeCallers ?? []).filter(
    caller => caller.profile === profileName,
  )

  useEffect(() => {
    setNotice(null)
    setPendingServerName(null)
    setDeletingProfile(false)
  }, [profileName])

  if (!profileName) {
    return <PanelEmpty />
  }

  if (isLoading) {
    return <PanelSkeleton />
  }

  if (!profile) {
    return (
      <Empty className="h-full">
        <EmptyHeader>
          <EmptyTitle>Profile not found</EmptyTitle>
          <EmptyDescription>
            The selected profile could not be loaded.
          </EmptyDescription>
        </EmptyHeader>
        {notice && (
          <div className="mt-4 w-full">
            <Alert variant={notice.variant}>
              {notice.variant === 'success' ? <CheckCircleIcon /> : <AlertCircleIcon />}
              <AlertTitle>{notice.title}</AlertTitle>
              <AlertDescription>{notice.description}</AlertDescription>
              <AlertAction>
                <Button variant="ghost" size="xs" onClick={() => setNotice(null)}>
                  Dismiss
                </Button>
              </AlertAction>
            </Alert>
          </div>
        )}
      </Empty>
    )
  }

  const canEditServers = Boolean(configMode?.isWritable)
  const canDeleteProfile = Boolean(
    configMode?.isWritable
    && configMode?.mode === 'directory'
    && profile.name !== defaultProfileName,
  )
  const serverActionHint = canEditServers ? undefined : 'Configuration is not writable'

  const handleToggleDisabled = async (
    server: ServerSpecWithKey,
    disabled: boolean,
  ) => {
    if (!canEditServers || pendingServerName) {
      return
    }
    setPendingServerName(server.name)
    setNotice(null)
    try {
      await WailsService.SetServerDisabled({
        profile: profile.name,
        server: server.name,
        disabled,
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
      await Promise.all([mutateProfile(), mutateProfiles()])
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
      setPendingServerName(null)
    }
  }

  const handleDeleteServer = async (server: ServerSpecWithKey) => {
    if (!canEditServers || pendingServerName) {
      return
    }
    setPendingServerName(server.name)
    setNotice(null)
    try {
      await WailsService.DeleteServer({
        profile: profile.name,
        server: server.name,
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
      await Promise.all([mutateProfile(), mutateProfiles()])
      setNotice({
        variant: 'success',
        title: 'Server deleted',
        description: 'Changes applied.',
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Delete failed.'
      setNotice({
        variant: 'error',
        title: 'Delete failed',
        description: message,
      })
    } finally {
      setPendingServerName(null)
    }
  }

  const handleDeleteProfile = async () => {
    if (!canDeleteProfile || deletingProfile) {
      return
    }
    setDeletingProfile(true)
    setNotice(null)
    try {
      await WailsService.DeleteProfile({ name: profile.name })
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setNotice({
          variant: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }
      await Promise.all([mutateProfiles(), mutateProfile()])
      setNotice({
        variant: 'success',
        title: 'Profile deleted',
        description: 'Changes applied.',
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Delete failed.'
      setNotice({
        variant: 'error',
        title: 'Delete failed',
        description: message,
      })
    } finally {
      setDeletingProfile(false)
    }
  }

  return (
    <div className="p-4">
      <ProfileContent
        profile={profile}
        activeCallers={profileCallers}
        canEditServers={canEditServers}
        canDeleteProfile={canDeleteProfile}
        serverActionHint={serverActionHint}
        pendingServerName={pendingServerName}
        deletingProfile={deletingProfile}
        notice={notice}
        onDismissNotice={() => setNotice(null)}
        onSubAgentToggle={(_enabled) => mutateProfile()}
        onToggleDisabled={handleToggleDisabled}
        onDeleteServer={handleDeleteServer}
        onDeleteProfile={handleDeleteProfile}
      />
    </div>
  )
}
