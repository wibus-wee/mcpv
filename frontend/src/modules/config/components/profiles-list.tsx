// Input: ProfileSummary array, selection state
// Output: ProfilesList component - minimal list view of profiles
// Position: Left panel in config page master-detail layout

import type { ActiveCaller, ProfileSummary } from '@bindings/mcpd/internal/ui'
import { ProfileService } from '@bindings/mcpd/internal/ui'
import {
  AlertCircleIcon,
  CheckCircleIcon,
  LayersIcon,
  PlusIcon,
  StarIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useEffect, useMemo, useState } from 'react'

import { CallerChipGroup } from '@/components/common/caller-chip-group'
import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogPanel,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { useConfigMode } from '../hooks'
import { reloadConfig } from '../lib/reload-config'

interface ProfilesListProps {
  profiles: ProfileSummary[]
  selectedProfile: string | null
  onSelect: (name: string) => void
  isLoading: boolean
  onRefresh: () => void
  activeCallers: ActiveCaller[]
}

type NoticeState = {
  variant: 'success' | 'error'
  title: string
  description: string
}

function ProfilesListSkeleton() {
  return (
    <div className="space-y-1">
      {Array.from({ length: 3 }).map((_, i) => (
        <Skeleton key={i} className="h-12 w-full rounded-md" />
      ))}
    </div>
  )
}

function ProfilesListEmpty() {
  return (
    <Empty className="py-8">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <LayersIcon className="size-4" />
        </EmptyMedia>
        <EmptyTitle className="text-sm">No profiles</EmptyTitle>
        <EmptyDescription className="text-xs">
          Create a profile to get started.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function ProfilesList({
  profiles,
  selectedProfile,
  onSelect,
  isLoading,
  activeCallers,
  onRefresh,
}: ProfilesListProps) {
  const { data: configMode } = useConfigMode()
  const [createOpen, setCreateOpen] = useState(false)
  const [createName, setCreateName] = useState('')
  const [createError, setCreateError] = useState<string | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [notice, setNotice] = useState<NoticeState | null>(null)
  const activeCallersByProfile = useMemo(() => {
    const map = new Map<string, ActiveCaller[]>()
    activeCallers.forEach(caller => {
      const list = map.get(caller.profile) ?? []
      list.push(caller)
      map.set(caller.profile, list)
    })
    return map
  }, [activeCallers])

  useEffect(() => {
    if (!createOpen) {
      setCreateName('')
      setCreateError(null)
      setIsCreating(false)
    }
  }, [createOpen])

  const canCreate = Boolean(
    configMode?.isWritable && configMode?.mode === 'directory',
  )
  const createHint = !configMode?.isWritable
    ? 'Configuration is not writable.'
    : configMode?.mode === 'directory'
      ? undefined
      : 'Profile store is required.'
  const createDisabled = !canCreate || !createName.trim() || isCreating

  const handleCreateProfile = async () => {
    if (createDisabled) {
      return
    }
    setIsCreating(true)
    setCreateError(null)
    setNotice(null)
    try {
      await ProfileService.CreateProfile({ name: createName.trim() })
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setNotice({
          variant: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }
      await onRefresh()
      setCreateOpen(false)
      setNotice({
        variant: 'success',
        title: 'Profile created',
        description: 'Changes applied.',
      })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Create failed.'
      setCreateError(message)
    } finally {
      setIsCreating(false)
    }
  }

  if (isLoading) {
    return <ProfilesListSkeleton />
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">Profiles</span>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger
            disabled={!canCreate}
            render={(
              <Button
                variant="secondary"
                size="xs"
                title={createHint}
              >
                <PlusIcon className="size-3.5" />
                New profile
              </Button>
            )}
          />
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create profile</DialogTitle>
              <DialogDescription>
                Profiles are stored as separate YAML files in the profile store.
              </DialogDescription>
            </DialogHeader>
            <DialogPanel className="space-y-3">
              {createError && (
                <Alert variant="error">
                  <AlertCircleIcon />
                  <AlertTitle>Create failed</AlertTitle>
                  <AlertDescription>{createError}</AlertDescription>
                </Alert>
              )}
              <div className="space-y-1">
                <span className="text-xs text-muted-foreground">Profile name</span>
                <Input
                  value={createName}
                  onChange={event => setCreateName(event.target.value)}
                  placeholder="e.g. cursor"
                  className="font-mono text-sm"
                />
              </div>
            </DialogPanel>
            <DialogFooter>
              <DialogClose render={<Button variant="ghost">Cancel</Button>} />
              <Button
                variant="default"
                onClick={handleCreateProfile}
                disabled={createDisabled}
              >
                {isCreating ? 'Creating...' : 'Create profile'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {notice && (
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
      )}

      {profiles.length === 0 ? (
        <ProfilesListEmpty />
      ) : (
        <div className="space-y-0.5">
          {profiles.map((profile, index) => (
            <m.button
              key={profile.name}
              type="button"
              initial={{ opacity: 0, x: -8 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.15, delay: index * 0.02 }}
              onClick={() => onSelect(profile.name)}
              className={cn(
                'group flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left transition-colors',
                selectedProfile === profile.name
                  ? 'bg-accent text-accent-foreground'
                  : 'hover:bg-muted/50'
              )}
            >
              {profile.isDefault ? (
                <StarIcon className="size-3.5 fill-warning text-warning shrink-0" />
              ) : (
                <LayersIcon className="size-3.5 text-muted-foreground shrink-0" />
              )}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="font-medium text-sm truncate">
                    {profile.name}
                  </span>
                  {profile.isDefault && (
                    <Badge variant="secondary" size="sm" className="shrink-0">
                      Default
                    </Badge>
                  )}
                </div>
                <p className="text-muted-foreground text-xs">
                  {profile.serverCount} server{profile.serverCount !== 1 ? 's' : ''}
                </p>
                <CallerChipGroup
                  callers={activeCallersByProfile.get(profile.name) ?? []}
                  maxVisible={2}
                  className="mt-1"
                />
              </div>
            </m.button>
          ))}
        </div>
      )}
    </div>
  )
}
