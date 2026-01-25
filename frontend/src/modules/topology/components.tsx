// Input: UI primitives (Skeleton, Empty components)
// Output: FlowSkeleton and FlowEmpty auxiliary components
// Position: Loading and empty state components for topology visualization

import { Share2Icon } from 'lucide-react'

import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'

export const FlowSkeleton = () => {
  return (
    <div className="h-full rounded-xl border bg-card/60 p-6">
      <div className="flex items-center gap-3">
        <Skeleton className="h-5 w-32" />
        <Skeleton className="h-5 w-20" />
        <Skeleton className="h-5 w-20" />
      </div>
      <div className="mt-6 grid grid-cols-3 gap-4">
        {Array.from({ length: 6 }).map((_, index) => (
          <Skeleton key={index} className="h-20 w-full" />
        ))}
      </div>
    </div>
  )
}

export const FlowEmpty = () => {
  return (
    <div className="flex h-full items-center justify-center rounded-xl border bg-card/60">
      <Empty className="py-16">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Share2Icon className="size-4" />
          </EmptyMedia>
          <EmptyTitle className="text-sm">No topology data</EmptyTitle>
          <EmptyDescription className="text-xs">
            Add servers and tags to render the configuration map.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    </div>
  )
}
