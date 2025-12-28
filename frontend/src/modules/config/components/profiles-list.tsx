// Input: ProfileSummary array, selection state
// Output: ProfilesList component - minimal list view of profiles
// Position: Left panel in config page master-detail layout

import type { ProfileSummary } from '@bindings/mcpd/internal/ui'
import { LayersIcon, StarIcon } from 'lucide-react'
import { m } from 'motion/react'

import { Badge } from '@/components/ui/badge'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

interface ProfilesListProps {
  profiles: ProfileSummary[]
  selectedProfile: string | null
  onSelect: (name: string) => void
  isLoading: boolean
  onRefresh: () => void
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
          Create a profile in your configuration file.
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
}: ProfilesListProps) {
  if (isLoading) {
    return <ProfilesListSkeleton />
  }

  if (profiles.length === 0) {
    return <ProfilesListEmpty />
  }

  return (
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
          </div>
        </m.button>
      ))}
    </div>
  )
}
