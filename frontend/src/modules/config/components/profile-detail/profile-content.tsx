// Input: ProfileDetail, ActiveCaller types, sub-components
// Output: ProfileContent component - main content area
// Position: Presentational component for profile details

import type { ActiveCaller, ProfileDetail } from '@bindings/mcpd/internal/ui'
import {
  AlertCircleIcon,
  CheckCircleIcon,
  ServerIcon,
  TrashIcon,
} from 'lucide-react'
import { m } from 'motion/react'

import { Accordion } from '@/components/ui/accordion'
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
import { Spring } from '@/lib/spring'

import type { NoticeState } from '../../hooks/use-config-reload'
import { RuntimeSection } from './runtime-section'
import { ServerItem, type ServerSpecWithKey } from './server-item'
import { SubAgentSection } from './subagent-section'

interface ProfileContentProps {
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
}

/**
 * Main content component for displaying profile details.
 * Receives all data and callbacks as props for clean separation.
 */
export function ProfileContent({
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
}: ProfileContentProps) {
  return (
    <m.div
      className="space-y-4"
      initial={{ opacity: 0, x: 8 }}
      animate={{ opacity: 1, x: 0 }}
      transition={Spring.smooth(0.25)}
    >
      {/* Notice Alert */}
      {notice && (
        <Alert variant={notice.variant}>
          {notice.variant === 'success' ? <CheckCircleIcon /> : <AlertCircleIcon />}
          <AlertTitle>{notice.title}</AlertTitle>
          {notice.description && (
            <AlertDescription>{notice.description}</AlertDescription>
          )}
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

      {/* Active Callers */}
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

      {/* Runtime Config & SubAgent */}
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
