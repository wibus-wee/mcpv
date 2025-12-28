// Input: Card, Collapsible, Badge, Button, Empty components, resources hook
// Output: ResourcesList component displaying available MCP resources
// Position: Dashboard resources section with collapsible details

import {
  ChevronDownIcon,
  ExternalLinkIcon,
  FileTextIcon,
  RefreshCwIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useState } from 'react'

import type { ResourceEntry } from '@bindings/mcpd/internal/ui'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
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
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

import { useResources } from '../hooks'

interface ResourceSchema {
  uri: string
  name?: string
  description?: string
  mimeType?: string
}

export function ResourcesList() {
  const { resources, isLoading, mutate } = useResources()
  const [expandedItems, setExpandedItems] = useState<Set<string>>(new Set())

  const toggleExpanded = (uri: string) => {
    setExpandedItems((prev) => {
      const next = new Set(prev)
      if (next.has(uri)) {
        next.delete(uri)
      }
      else {
        next.add(uri)
      }
      return next
    })
  }

  const parseResourceJson = (resource: ResourceEntry): ResourceSchema => {
    try {
      const parsed = typeof resource.resourceJson === 'string'
        ? JSON.parse(resource.resourceJson)
        : resource.resourceJson
      return { uri: resource.uri, ...parsed }
    }
    catch {
      return { uri: resource.uri }
    }
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <FileTextIcon className="size-5" />
            Resources
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </CardContent>
      </Card>
    )
  }

  return (
    <div>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <FileTextIcon className="size-5" />
              Resources
              <Badge variant="secondary" size="sm">
                {resources.length}
              </Badge>
            </CardTitle>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => mutate()}
            >
              <RefreshCwIcon className="size-4" />
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <ScrollArea className="h-[300px]">
            {resources.length === 0 ? (
              <Empty className="h-full">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <FileTextIcon className="size-5 text-muted-foreground" />
                  </EmptyMedia>
                  <EmptyTitle>No resources</EmptyTitle>
                  <EmptyDescription>
                    Resources will appear here when MCP servers expose them.
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : (
              <div className="space-y-2">
                {resources.map((resource) => {
                  const parsed = parseResourceJson(resource)
                  const isExpanded = expandedItems.has(resource.uri)

                  return (
                    <Collapsible
                      key={resource.uri}
                      open={isExpanded}
                      onOpenChange={() => toggleExpanded(resource.uri)}
                    >
                      <div className="rounded-lg border bg-card">
                        <CollapsibleTrigger className="flex w-full items-center justify-between p-3 text-left hover:bg-muted/50">
                          <div className="flex items-center gap-3 min-w-0">
                            <FileTextIcon className="size-4 shrink-0 text-muted-foreground" />
                            <div className="min-w-0">
                              <p className="truncate font-medium text-sm">
                                {parsed.name || resource.uri}
                              </p>
                              <p className="truncate text-muted-foreground text-xs">
                                {resource.uri}
                              </p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                            {parsed.mimeType && (
                              <Badge variant="outline" size="sm">
                                {parsed.mimeType}
                              </Badge>
                            )}
                            <ChevronDownIcon
                              className={cn(
                                'size-4 text-muted-foreground transition-transform',
                                isExpanded && 'rotate-180',
                              )}
                            />
                          </div>
                        </CollapsibleTrigger>
                        <CollapsibleContent>
                          <Separator />
                          <div className="p-3 space-y-2">
                            {parsed.description && (
                              <p className="text-muted-foreground text-sm">
                                {parsed.description}
                              </p>
                            )}
                            <div className="flex items-center gap-2">
                              <Button variant="outline" size="sm">
                                <ExternalLinkIcon className="size-3.5" />
                                Read Resource
                              </Button>
                            </div>
                          </div>
                        </CollapsibleContent>
                      </div>
                    </Collapsible>
                  )
                })}
              </div>
            )}
          </ScrollArea>
        </CardContent>
      </Card>
    </div>
  )
}
