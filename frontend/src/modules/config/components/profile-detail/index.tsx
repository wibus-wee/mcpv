// Input: profileName prop, useProfile hook, ProfileDetail type
// Output: ProfileDetailPanel component - main entry point
// Position: Right panel in config page master-detail layout

import {
  AlertCircleIcon,
  CheckCircleIcon,
  LayersIcon,
} from 'lucide-react'

import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { useActiveCallers } from '@/hooks/use-active-callers'

import { useConfigMode, useProfile, useProfiles } from '../../hooks'
import { ProfileContent } from './profile-content'
import { useProfileActions } from './use-profile-actions'

const DEFAULT_PROFILE_NAME = 'default'

interface ProfileDetailPanelProps {
  profileName: string | null
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

/**
 * Main panel component for displaying and managing profile details.
 * Orchestrates data fetching, state management, and renders ProfileContent.
 */
export function ProfileDetailPanel({ profileName }: ProfileDetailPanelProps) {
  const { data: profile, isLoading, mutate: mutateProfile } = useProfile(profileName)
  const { mutate: mutateProfiles } = useProfiles()
  const { data: configMode } = useConfigMode()
  const { data: activeCallers } = useActiveCallers()

  // Compute permissions
  const canEditServers = Boolean(configMode?.isWritable)
  const canDeleteProfile = Boolean(
    configMode?.isWritable
    && configMode?.mode === 'directory'
    && profileName !== DEFAULT_PROFILE_NAME,
  )
  const serverActionHint = canEditServers ? undefined : 'Configuration is not writable'

  // Profile action handlers
  const {
    notice,
    pendingServerName,
    deletingProfile,
    clearNotice,
    handleToggleDisabled,
    handleDeleteServer,
    handleDeleteProfile,
  } = useProfileActions({
    profileName,
    canEditServers,
    canDeleteProfile,
    mutateProfile: async () => { await mutateProfile() },
    mutateProfiles: async () => { await mutateProfiles() },
  })

  // Filter callers for this profile
  const profileCallers = (activeCallers ?? []).filter(
    caller => caller.profile === profileName,
  )

  // Empty state
  if (!profileName) {
    return <PanelEmpty />
  }

  // Loading state
  if (isLoading) {
    return <PanelSkeleton />
  }

  // Profile not found
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
              {notice.description && (
                <AlertDescription>{notice.description}</AlertDescription>
              )}
              <AlertAction>
                <Button variant="ghost" size="xs" onClick={clearNotice}>
                  Dismiss
                </Button>
              </AlertAction>
            </Alert>
          </div>
        )}
      </Empty>
    )
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
        onDismissNotice={clearNotice}
        onSubAgentToggle={() => mutateProfile()}
        onToggleDisabled={handleToggleDisabled}
        onDeleteServer={handleDeleteServer}
        onDeleteProfile={handleDeleteProfile}
      />
    </div>
  )
}
