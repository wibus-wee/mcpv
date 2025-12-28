// Input: Table, Badge, Button, Input, Dialog, ScrollArea, Accordion components, tools atom
// Output: ToolsTable component displaying available MCP tools
// Position: Dashboard tools section with search and detail view

import { useAtomValue } from 'jotai'
import {
  ChevronRightIcon,
  CopyIcon,
  SearchIcon,
  WrenchIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useMemo, useState } from 'react'

import { toolsAtom } from '@/atoms/dashboard'
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
import { Spring } from '@/lib/spring'

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

export function ToolsTable() {
  const tools = useAtomValue(toolsAtom)
  const { isLoading } = useTools()
  const [search, setSearch] = useState('')

  const filteredTools = useMemo(() => {
    if (!search) return tools
    const lower = search.toLowerCase()
    return tools.filter(tool =>
      tool.name.toLowerCase().includes(lower),
    )
  }, [tools, search])

  const parseToolJson = (tool: typeof tools[0]): ToolSchema => {
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
    <m.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.4, 0.1)}
    >
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
                    const parsed = parseToolJson(tool)
                    return (
                      <TableRow key={tool.name}>
                        <TableCell className="py-1.5 font-mono ">
                          {tool.name}
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
                                    onClick={() => copyToClipboard(tool.name)}
                                  >
                                    <CopyIcon className="size-3" />
                                  </Button>
                                )}
                              />
                              <TooltipContent>Copy name</TooltipContent>
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
                                    {parsed.name}
                                  </DialogTitle>
                                  <DialogDescription className="">
                                    {parsed.description || 'No description available'}
                                  </DialogDescription>
                                </DialogHeader>
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
    </m.div>
  )
}
