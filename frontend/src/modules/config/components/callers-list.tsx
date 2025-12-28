// Input: Callers mapping (Record<string, string>)
// Output: CallersList component - minimal list view of caller-to-profile mappings
// Position: Tab content in config page, uses divide-y pattern for visual separation

import { ArrowRightIcon, UsersIcon } from 'lucide-react'
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
    <Empty className="py-12">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <UsersIcon className="size-4" />
        </EmptyMedia>
        <EmptyTitle className="text-sm">No caller mappings</EmptyTitle>
        <EmptyDescription className="text-xs">
          Define caller mappings in your configuration to route clients to specific profiles.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function CallersList({
  callers,
  isLoading,
}: CallersListProps) {
  const entries = Object.entries(callers)

  if (isLoading) {
    return <CallersListSkeleton />
  }

  if (entries.length === 0) {
    return <CallersListEmpty />
  }

  return (
    <m.div
      className="divide-y divide-border/50"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.2 }}
    >
      {entries.map(([caller, profile], index) => (
        <m.div
          key={caller}
          initial={{ opacity: 0, x: -8 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.15, delay: index * 0.02 }}
          className="flex items-center gap-3 py-2.5 group"
        >
          <span className="font-mono text-sm truncate flex-1 min-w-0">
            {caller}
          </span>
          <ArrowRightIcon className="size-3 text-muted-foreground/60 shrink-0" />
          <Badge variant="secondary" size="sm" className="shrink-0 font-mono">
            {profile}
          </Badge>
        </m.div>
      ))}
    </m.div>
  )
}
