// Input: Table, Badge, Button, Input, Dialog, ScrollArea, Accordion components, tools hook
// Output: ToolsTable component displaying available MCP tools
// Position: Dashboard tools section with search and detail view

import type { ToolEntry } from '@bindings/mcpd/internal/ui'
import {
  ChevronRightIcon,
  CopyIcon,
  SearchIcon,
  WrenchIcon,
} from 'lucide-react'
import { useMemo, useState } from 'react'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatRelativeTime } from '@/lib/time'
import { getToolDisplayName, getToolQualifiedName } from '@/lib/tool-names'

import { useTools } from '../hooks'

interface ToolSchema {
  name: string
  description?: string
  inputSchema?: {
    type: string
    properties?: Record<string, { type: string, description?: string }>
    required?: string[]
  }
}

const parseToolJson = (tool: ToolEntry): ToolSchema => {
  try {
    const parsed = typeof tool.toolJson === 'string'
      ? JSON.parse(tool.toolJson)
      : tool.toolJson
    return { name: tool.name, ...parsed }
  }
  catch {
    return { name: tool.name }
  }
}

export function ToolsTable() {
  const { tools, isLoading } = useTools()
  const [search, setSearch] = useState('')

  const parsedTools = useMemo(() => {
    const map = new Map<string, ToolSchema>()
    tools.forEach((tool) => {
      map.set(tool.name, parseToolJson(tool))
    })
    return map
  }, [tools])

  const filteredTools = useMemo(() => {
    if (!search) return tools
    const lower = search.toLowerCase()
    return tools.filter(tool =>
      getToolDisplayName(tool.name, tool.serverName).toLowerCase().includes(lower)
      || tool.name.toLowerCase().includes(lower)
      || (tool.serverName ?? '').toLowerCase().includes(lower),
    )
  }, [tools, search])

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-1.5">
            <WrenchIcon className="size-4" />
            Tools
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-1.5">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-8 w-full" />
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
            <CardTitle className="flex items-center gap-1.5">
              <WrenchIcon className="size-4" />
              Tools
              <Badge variant="secondary" size="sm">
                {tools.length}
              </Badge>
            </CardTitle>
            <div className="relative w-48">
              <SearchIcon className="absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Search tools..."
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="h-7 pl-7"
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <ScrollArea className="h-64">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="">Name</TableHead>
                  <TableHead className="">Description</TableHead>
                  <TableHead className="w-20">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredTools.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={3} className="text-center  text-muted-foreground">
                      {search ? 'No tools match your search' : 'No tools available'}
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredTools.map((tool) => {
                    const parsed = parsedTools.get(tool.name) ?? { name: tool.name }
                    const displayName = getToolDisplayName(tool.name, tool.serverName)
                    const qualifiedName = getToolQualifiedName(tool.name, tool.serverName)
                    const isCached = tool.source === 'cache'
                    const cachedLabel = tool.cachedAt
                      ? `Cached ${formatRelativeTime(tool.cachedAt)}`
                      : 'Cached metadata'
                    return (
                      <TableRow key={tool.name}>
                        <TableCell className="py-1.5 font-mono ">
                          <div className="flex items-center gap-2">
                            <span>{displayName}</span>
                            {tool.serverName && (
                              <Badge variant="outline" size="sm">
                                {tool.serverName}
                              </Badge>
                            )}
                            {isCached && (
                              <Tooltip>
                                <TooltipTrigger
                                  render={(
                                    <Badge variant="outline" size="sm">
                                      cached
                                    </Badge>
                                  )}
                                />
                                <TooltipContent>{cachedLabel}</TooltipContent>
                              </Tooltip>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="max-w-60 truncate py-1.5  text-muted-foreground">
                          {parsed.description || '--'}
                        </TableCell>
                        <TableCell className="py-1.5">
                          <div className="flex items-center gap-0.5">
                            <Tooltip>
                              <TooltipTrigger
                                render={(
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                    onClick={() => copyToClipboard(qualifiedName)}
                                  >
                                    <CopyIcon className="size-3" />
                                  </Button>
                                )}
                              />
                              <TooltipContent>Copy qualified name</TooltipContent>
                            </Tooltip>
                            <Dialog>
                              <DialogTrigger
                                render={(
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                  >
                                    <ChevronRightIcon className="size-3" />
                                  </Button>
                                )}
                              />
                              <DialogContent>
                                <DialogHeader>
                                  <DialogTitle className="font-mono text-sm">
                                    {displayName}
                                  </DialogTitle>
                                  <DialogDescription className="">
                                    {parsed.description || 'No description available'}
                                  </DialogDescription>
                                </DialogHeader>
                                {tool.serverName && (
                                  <div className="text-xs text-muted-foreground">
                                    Server <span className="font-mono">{tool.serverName}</span>
                                  </div>
                                )}
                                {qualifiedName !== displayName && (
                                  <div className="text-xs text-muted-foreground">
                                    Qualified name <span className="font-mono">{qualifiedName}</span>
                                  </div>
                                )}
                                <Separator />
                                {parsed.inputSchema?.properties && (
                                  <div className="space-y-3 p-8">
                                    <h4 className="font-medium ">Parameters</h4>
                                    <Accordion>
                                      {Object.entries(parsed.inputSchema.properties).map(
                                        ([key, value]) => (
                                          <AccordionItem key={key} value={key}>
                                            <AccordionTrigger>
                                              <div className="flex items-center gap-1.5">
                                                <span className="font-mono ">{key}</span>
                                                <Badge variant="outline" size="sm">
                                                  {value.type}
                                                </Badge>
                                                {parsed.inputSchema?.required?.includes(key) && (
                                                  <Badge variant="error" size="sm">
                                                    required
                                                  </Badge>
                                                )}
                                              </div>
                                            </AccordionTrigger>
                                            <AccordionContent>
                                              {value.description || 'No description'}
                                            </AccordionContent>
                                          </AccordionItem>
                                        ),
                                      )}
                                    </Accordion>
                                  </div>
                                )}
                              </DialogContent>
                            </Dialog>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
          </ScrollArea>
        </CardContent>
      </Card>
    </div>
  )
}
