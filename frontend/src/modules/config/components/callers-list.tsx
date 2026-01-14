// Input: Callers mapping (Record<string, string>)
// Output: CallersList component - editable list view of caller-to-profile mappings
// Position: Tab content in config page, uses divide-y pattern for visual separation

import type { ProfileSummary } from '@bindings/mcpd/internal/ui'
import { ProfileService } from '@bindings/mcpd/internal/ui'
import { ArrowRightIcon, TrashIcon, UsersIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useMemo, useState } from 'react'

import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { useConfigMode, useProfiles } from '../hooks'
import { reloadConfig } from '../lib/reload-config'

interface CallersListProps {
  callers: Record<string, string>
  isLoading: boolean
  onRefresh: () => void
}

function CallersListSkeleton() {
  return (
    <div className="divide-y divide-border/50">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="py-3">
          <Skeleton className="h-5 w-full" />
        </div>
      ))}
    </div>
  )
}

function CallersListEmpty() {
  return (
    <Empty className="py-8">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <UsersIcon className="size-4" />
        </EmptyMedia>
        <EmptyTitle className="text-sm">No caller mappings</EmptyTitle>
        <EmptyDescription className="text-xs">
          Use the form above to create your first mapping.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function CallersList({
  callers,
  isLoading,
  onRefresh,
}: CallersListProps) {
  const { data: profiles } = useProfiles()
  const { data: configMode } = useConfigMode()
  const [draftCaller, setDraftCaller] = useState('')
  const [draftProfile, setDraftProfile] = useState('')
  const [pendingCaller, setPendingCaller] = useState<string | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const entries = Object.entries(callers)
  const profileOptions = useMemo(
    () => (profiles ?? []).map((profile: ProfileSummary) => profile.name),
    [profiles],
  )
  const hasProfiles = profileOptions.length > 0
  const canEdit = Boolean(
    configMode?.isWritable && configMode?.mode === 'directory',
  )
  const editHint = !configMode?.isWritable
    ? 'Configuration is not writable.'
    : configMode?.mode === 'directory'
      ? undefined
      : 'Profile store is required.'
  const createDisabled = !canEdit
    || !draftCaller.trim()
    || !draftProfile
    || isCreating
    || !hasProfiles

  const handleAddMapping = async () => {
    if (createDisabled) {
      return
    }
    const caller = draftCaller.trim()
    const profile = draftProfile
    setIsCreating(true)
    try {
      await ProfileService.SetCallerMapping({ caller, profile })
      const reloadResult = await reloadConfig()
      setDraftCaller('')
      setDraftProfile('')
      if (!reloadResult.ok) {
        return
      }
      await onRefresh()
    } catch (err) {
    } finally {
      setIsCreating(false)
    }
  }

  const handleUpdateMapping = async (caller: string, profile: string) => {
    if (!canEdit || pendingCaller || profile === callers[caller]) {
      return
    }
    setPendingCaller(caller)
    try {
      await ProfileService.SetCallerMapping({ caller, profile })
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        return
      }
      await onRefresh()
    } finally {
      setPendingCaller(null)
    }
  }

  const handleRemoveMapping = async (caller: string) => {
    if (!canEdit || pendingCaller) {
      return
    }
    setPendingCaller(caller)
    try {
      await ProfileService.RemoveCallerMapping(caller)
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        return
      }
      await onRefresh()
    } finally {
      setPendingCaller(null)
    }
  }

  if (isLoading) {
    return <CallersListSkeleton />
  }

  return (
    <div className="space-y-4">
      <div className="rounded-lg border bg-muted/20 px-3 py-2">
        <div className="flex flex-wrap items-center gap-2">
          <Input
            value={draftCaller}
            onChange={event => setDraftCaller(event.target.value)}
            placeholder="Caller"
            className="h-8 min-w-[160px] font-mono text-xs"
            disabled={!canEdit || isCreating}
          />
          <Select
            value={draftProfile || undefined}
            onValueChange={(value) => value && setDraftProfile(value)}
            disabled={!canEdit || !hasProfiles || isCreating}
          >
            <SelectTrigger size="sm" className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {profileOptions.map(profile => (
                <SelectItem key={profile} value={profile}>
                  {profile}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleAddMapping}
            disabled={createDisabled}
            title={editHint}
          >
            {isCreating ? 'Saving...' : 'Add mapping'}
          </Button>
        </div>
        {!canEdit && (
          <p className="mt-2 text-xs text-muted-foreground">
            Caller mappings require a writable profile store.
          </p>
        )}
      </div>

      {entries.length === 0 ? (
        <CallersListEmpty />
      ) : (
        <m.div
          className="divide-y divide-border/50"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.2 }}
        >
          {entries.map(([caller, profile], index) => {
            const hasProfileOption = profileOptions.includes(profile)
            const rowOptions = hasProfileOption
              ? profileOptions
              : [profile, ...profileOptions]

            return (
              <m.div
                key={caller}
                initial={{ opacity: 0, x: -8 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.15, delay: index * 0.02 }}
                className={cn(
                  'flex items-center gap-3 py-2.5',
                  pendingCaller === caller && 'opacity-60',
                )}
              >
                <span className="font-mono text-sm truncate flex-1 min-w-0">
                  {caller}
                </span>
                <ArrowRightIcon className="size-3 text-muted-foreground/60 shrink-0" />
                <Select
                  value={profile}
                  onValueChange={(next) => next && handleUpdateMapping(caller, next)}
                  disabled={!canEdit || pendingCaller === caller}
                >
                  <SelectTrigger size="sm" className="w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {rowOptions.map(option => (
                      <SelectItem key={option} value={option}>
                        {option === profile && !hasProfileOption
                          ? `${option} (missing)`
                          : option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  onClick={() => handleRemoveMapping(caller)}
                  disabled={!canEdit || pendingCaller === caller}
                  title={editHint}
                >
                  <TrashIcon className="size-3.5" />
                </Button>
              </m.div>
            )
          })}
        </m.div>
      )}
    </div>
  )
}
