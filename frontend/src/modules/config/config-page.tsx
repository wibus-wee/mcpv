// Input: Config hooks, atoms, UI components
// Output: ConfigPage component - master-detail configuration view
// Position: Main page in config module

import { useAtom } from 'jotai'
import {
  ExternalLinkIcon,
  FileIcon,
  FileSliders,
  FolderIcon,
  LayersIcon,
  PenIcon,
  UsersIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useState } from 'react'

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
import { Spring } from '@/lib/spring'

import { selectedProfileNameAtom } from './atoms'
import { CallersList } from './components/callers-list'
import { ProfileDetailPanel } from './components/profile-detail-panel'
import { ProfilesList } from './components/profiles-list'
import { useCallers, useConfigMode, useOpenConfigInEditor, useProfiles } from './hooks'

function ConfigHeader() {
  const { data: configMode, isLoading, mutate } = useConfigMode()
  const { openInEditor, isOpening } = useOpenConfigInEditor()
  const [isRefreshing, setIsRefreshing] = useState(false)

  const handleRefresh = async () => {
    setIsRefreshing(true)
    await mutate()
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

  const ModeIcon = configMode?.mode === 'directory' ? FolderIcon : FileIcon

  return (
    <m.div
      className="flex items-center justify-between"
      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.3)}
    >
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <FileSliders className="size-4 text-muted-foreground" />
          <h1 className="font-semibold text-lg">Configuration</h1>
        </div>
        <div className="flex items-center gap-2 text-muted-foreground text-xs">
          {configMode && (
            <>
              <ModeIcon className="size-3" />
              <span className="font-mono truncate max-w-80">
                {configMode.path}
              </span>
              {configMode.isWritable && (
                <Badge variant="outline" size="sm" className="text-success border-success/30 gap-1">
                  <PenIcon className="size-2.5" />
                  Writable
                </Badge>
              )}
            </>
          )}
        </div>
      </div>
      <div className="flex items-center gap-1">
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

function ProfilesTabContent() {
  const [selectedProfileName, setSelectedProfileName] = useAtom(selectedProfileNameAtom)
  const {
    data: profiles,
    isLoading: profilesLoading,
    mutate: mutateProfiles,
  } = useProfiles()

  const handleProfileSelect = (name: string) => {
    setSelectedProfileName(name === selectedProfileName ? null : name)
  }

  return (
    <div className="flex flex-1 min-h-0 gap-px h-full">
      {/* Left: Profiles List */}
      <div className="w-64 shrink-0 flex flex-col min-h-0 border-r">
        <ScrollArea className="flex-1" scrollFade>
          <div className="p-3">
            <ProfilesList
              profiles={profiles ?? []}
              isLoading={profilesLoading}
              selectedProfile={selectedProfileName}
              onSelect={handleProfileSelect}
              onRefresh={() => mutateProfiles()}
            />
          </div>
        </ScrollArea>
      </div>

      {/* Right: Profile Detail */}
      <div className="flex-1 min-w-0 min-h-0">
        <ScrollArea className="h-full" scrollFade>
          <ProfileDetailPanel profileName={selectedProfileName} />
        </ScrollArea>
      </div>
    </div>
  )
}

function CallersTabContent() {
  const {
    data: callers,
    isLoading: callersLoading,
    mutate: mutateCallers,
  } = useCallers()

  return (
    <ScrollArea className="flex-1" scrollFade>
      <div className="p-4">
        <CallersList
          callers={callers ?? {}}
          isLoading={callersLoading}
          onRefresh={() => mutateCallers()}
        />
      </div>
    </ScrollArea>
  )
}

function ConfigTabs() {
  const { data: profiles } = useProfiles()
  const { data: callers } = useCallers()

  const profileCount = profiles?.length ?? 0
  const callerCount = callers ? Object.keys(callers).length : 0

  return (
    <Tabs defaultValue="profiles" className="flex-1 flex flex-col min-h-0">
      <TabsList variant="underline" className="w-full justify-start border-b px-4">
        <TabsTrigger value="profiles" className="gap-1.5">
          <LayersIcon className="size-3.5" />
          Profiles
          {profileCount > 0 && (
            <Badge variant="secondary" size="sm">
              {profileCount}
            </Badge>
          )}
        </TabsTrigger>
        <TabsTrigger value="callers" className="gap-1.5">
          <UsersIcon className="size-3.5" />
          Callers
          {callerCount > 0 && (
            <Badge variant="secondary" size="sm">
              {callerCount}
            </Badge>
          )}
        </TabsTrigger>
      </TabsList>

      <TabsContent value="profiles" className="flex-1 min-h-0 mt-0">
        <ProfilesTabContent />
      </TabsContent>

      <TabsContent value="callers" className="flex-1 min-h-0 mt-0">
        <CallersTabContent />
      </TabsContent>
    </Tabs>
  )
}

function ConfigEmpty() {
  return (
    <Empty className="h-full">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <FileSliders className="size-5" />
        </EmptyMedia>
        <EmptyTitle>No configuration loaded</EmptyTitle>
        <EmptyDescription>
          Start the server with a configuration file to see your profiles here.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function ConfigPage() {
  const { data: configMode } = useConfigMode()
  const { data: profiles } = useProfiles()

  const hasConfig = configMode && profiles

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
