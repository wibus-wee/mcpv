// Input: profileName prop, useProfile hook, ProfileDetail type
// Output: ProfileDetailPanel component - inline detail view with minimal visual weight
// Position: Right panel in config page master-detail layout (replaces Sheet overlay)

import type { ProfileDetail, ServerSpecDetail } from '@bindings/mcpd/internal/ui'
import {
  ClockIcon,
  CpuIcon,
  LayersIcon,
  NetworkIcon,
  ServerIcon,
  SettingsIcon,
  TerminalIcon,
} from 'lucide-react'
import { m } from 'motion/react'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Spring } from '@/lib/spring'

import { useProfile } from '../hooks'

interface ProfileDetailPanelProps {
  profileName: string | null
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

function ServerItem({ server }: { server: ServerSpecDetail }) {
  return (
    <AccordionItem value={`server-${server.name}`} className="border-none">
      <AccordionTrigger className="py-2 hover:no-underline">
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <ServerIcon className="size-3.5 text-muted-foreground shrink-0" />
          <span className="font-mono text-sm truncate">{server.name}</span>
          <div className="flex items-center gap-1 ml-auto mr-2">
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

function ProfileContent({ profile }: { profile: ProfileDetail }) {
  return (
    <m.div
      className="space-y-4"
      initial={{ opacity: 0, x: 8 }}
      animate={{ opacity: 1, x: 0 }}
      transition={Spring.smooth(0.25)}
    >
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="font-semibold">{profile.name}</h2>
        <Badge variant="secondary" size="sm">
          {profile.servers.length} server{profile.servers.length !== 1 ? 's' : ''}
        </Badge>
      </div>

      {/* Runtime Config */}
      <Accordion multiple defaultValue={['runtime']}>
        <RuntimeSection profile={profile} />
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
                Add servers to this profile in your configuration file.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <Accordion multiple>
            {profile.servers.map(server => (
              <ServerItem key={server.name} server={server} />
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
  const { data: profile, isLoading } = useProfile(profileName)

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
      </Empty>
    )
  }

  return (
    <div className="p-4">
      <ProfileContent profile={profile} />
    </div>
  )
}
