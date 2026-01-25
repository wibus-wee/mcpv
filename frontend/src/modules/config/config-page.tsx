// Input: Config hooks, atoms, UI components
// Output: ConfigPage component - tabbed configuration view with list, detail, and topology
// Position: Main page in config module

import { useAtom, useAtomValue } from 'jotai'
import {
  ExternalLinkIcon,
  FileSliders,
  MonitorIcon,
  ServerIcon,
  TagsIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useState } from 'react'
import { useSWRConfig } from 'swr'

import { RefreshButton } from '@/components/custom'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toastManager } from '@/components/ui/toast'
import { useActiveClients, activeClientsKey } from '@/hooks/use-active-clients'
import { Spring } from '@/lib/spring'

import { selectedServerNameAtom } from './atoms'
import { ClientsList } from './components/clients-list'
import { ImportMcpServersSheet } from './components/import-mcp-servers-sheet'
import { ServerDetailPanel } from './components/server-detail-panel'
import { ServersList } from './components/servers-list'
import { useConfigMode, useOpenConfigInEditor, useServers } from './hooks'
import { reloadConfig } from './lib/reload-config'

type MutateFn = ReturnType<typeof useSWRConfig>['mutate']

const refreshConfigData = async (mutate: MutateFn, selectedServerName: string | null) => {
  const requests = [
    mutate('servers'),
    mutate(activeClientsKey),
    mutate('runtime-status'),
    mutate('server-init-status'),
  ]
  if (selectedServerName) {
    requests.push(mutate(['server', selectedServerName]))
  }
  await Promise.all(requests)
}

function ConfigHeader() {
  const { data: configMode, isLoading } = useConfigMode()
  const { openInEditor, isOpening } = useOpenConfigInEditor()
  const [isRefreshing, setIsRefreshing] = useState(false)
  const { mutate } = useSWRConfig()
  const selectedServerName = useAtomValue(selectedServerNameAtom)

  const handleRefresh = async () => {
    setIsRefreshing(true)
    const result = await reloadConfig()
    if (result.ok) {
      await refreshConfigData(mutate, selectedServerName)
      toastManager.add({
        type: 'success',
        title: 'Configuration reloaded',
        description: 'Latest changes are now active.',
      })
    } else {
      toastManager.add({
        type: 'error',
        title: 'Reload failed',
        description: result.message,
      })
    }
    setIsRefreshing(false)
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-between">
        <div className="space-y-2">
          <Skeleton className="h-6 w-40" />
          <Skeleton className="h-4 w-56" />
        </div>
      </div>
    )
  }

  return (
    <m.div
      className="flex items-center justify-between"
      initial={{ opacity: 0, y: -8, filter: 'blur(4px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={Spring.smooth(0.3)}
    >
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <div className="flex size-7 items-center justify-center rounded-md bg-primary/10">
            <FileSliders className="size-4 text-primary" />
          </div>
          <h1 className="font-semibold text-xl tracking-tight">Configuration</h1>
        </div>
        <p className="text-muted-foreground text-sm ml-9">
          Manage MCP servers, tags, and active clients.
        </p>
      </div>
      <div className="flex items-center gap-1">
        <ImportMcpServersSheet />
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={openInEditor}
          disabled={isOpening || !configMode?.path}
          title="Open in editor"
        >
          <ExternalLinkIcon className="size-4" />
        </Button>
        <RefreshButton
          onClick={handleRefresh}
          isLoading={isRefreshing}
          tooltip="Reload configuration"
        />
      </div>
    </m.div>
  )
}

function ServersTabContent() {
  const [selectedServerName, setSelectedServerName] = useAtom(selectedServerNameAtom)
  const {
    data: servers,
    isLoading: serversLoading,
    mutate: mutateServers,
  } = useServers()

  const handleServerSelect = (name: string) => {
    setSelectedServerName(name === selectedServerName ? null : name)
  }

  return (
    <div className="flex flex-1 min-h-0 gap-px h-full">
      <div className="w-72 shrink-0 flex flex-col min-h-0 border-r">
        <ScrollArea className="flex-1" scrollFade>
          <div className="p-3">
            <ServersList
              servers={servers ?? []}
              isLoading={serversLoading}
              selectedServer={selectedServerName}
              onSelect={handleServerSelect}
              onRefresh={() => mutateServers()}
            />
          </div>
        </ScrollArea>
      </div>

      <div className="flex-1 min-w-0 min-h-0">
        <ScrollArea className="h-full" scrollFade>
          <ServerDetailPanel
            serverName={selectedServerName}
            onDeleted={() => setSelectedServerName(null)}
          />
        </ScrollArea>
      </div>
    </div>
  )
}

function ClientsTabContent() {
  const { data: clients, isLoading } = useActiveClients()

  return (
    <ScrollArea className="flex-1" scrollFade>
      <div className="p-4">
        <ClientsList clients={clients ?? []} isLoading={isLoading} />
      </div>
    </ScrollArea>
  )
}

function ConfigTabs() {
  const { data: servers } = useServers()
  const { data: clients } = useActiveClients()

  const serverCount = servers?.length ?? 0
  const clientCount = clients?.length ?? 0

  return (
    <Tabs defaultValue="servers" className="flex-1 flex flex-col min-h-0">
      <TabsList variant="underline" className="w-full justify-start border-b px-4">
        <TabsTrigger value="servers" className="gap-1.5">
          <ServerIcon className="size-3.5" />
          Servers
          {serverCount > 0 && (
            <Badge variant="secondary" size="sm">
              {serverCount}
            </Badge>
          )}
        </TabsTrigger>
        <TabsTrigger value="clients" className="gap-1.5">
          <MonitorIcon className="size-3.5" />
          Clients
          {clientCount > 0 && (
            <Badge variant="secondary" size="sm">
              {clientCount}
            </Badge>
          )}
        </TabsTrigger>
      </TabsList>

      <TabsContent value="servers" className="flex-1 min-h-0 mt-0">
        <ServersTabContent />
      </TabsContent>

      <TabsContent value="clients" className="flex-1 min-h-0 mt-0">
        <ClientsTabContent />
      </TabsContent>
    </Tabs>
  )
}

function ConfigEmpty() {
  return (
    <Empty className="h-full">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <TagsIcon className="size-5" />
        </EmptyMedia>
        <EmptyTitle>No configuration loaded</EmptyTitle>
        <EmptyDescription>
          Start the server with a configuration file to see your servers here.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function ConfigPage() {
  const { data: configMode } = useConfigMode()
  const { data: servers } = useServers()

  const hasConfig = configMode && servers

  return (
    <div className="flex flex-col h-full">
      <div className="px-6 pt-6 pb-4">
        <ConfigHeader />
      </div>
      <Separator />
      {hasConfig ? <ConfigTabs /> : <ConfigEmpty />}
    </div>
  )
}
