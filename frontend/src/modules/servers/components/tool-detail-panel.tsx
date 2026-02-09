// Input: Selected tool entry, server info
// Output: ToolDetailPanel component showing tool details inline
// Position: Right panel in master-detail tools layout

import type { ToolEntry } from '@bindings/mcpv/internal/ui/types'
import { CheckIcon, ChevronDownIcon, CodeIcon, CopyIcon, WrenchIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Spring } from '@/lib/spring'
import { getToolDisplayName, getToolQualifiedName } from '@/lib/tool-names'
import type { ToolSchema } from '@/lib/tool-schema'
import { cn } from '@/lib/utils'

interface ToolDetailPanelProps {
  tool: ToolEntry | null
  serverName?: string
  className?: string
}

export function ToolDetailPanel({ tool, serverName, className }: ToolDetailPanelProps) {
  const [schemaOpen, setSchemaOpen] = useState(false)
  const [copied, setCopied] = useState(false)

  if (!tool) {
    return (
      <div className={cn('flex flex-col items-center justify-center h-full text-muted-foreground', className)}>
        <WrenchIcon className="size-12 mb-4 opacity-20" />
        <p className="text-sm">Select a tool to view details</p>
      </div>
    )
  }

  const schema: ToolSchema = tool.toolJson
  const properties = schema.inputSchema?.properties || {}
  const required = schema.inputSchema?.required || []
  const hasParams = Object.keys(properties).length > 0
  const displayName = getToolDisplayName(tool.name, serverName)
  const qualifiedName = getToolQualifiedName(tool.name, serverName)

  const handleCopySchema = async () => {
    await navigator.clipboard.writeText(JSON.stringify(schema, null, 2))
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <ScrollArea className={cn('h-full', className)}>
      <m.div
        key={tool.name}
        initial={{ opacity: 0, x: 20 }}
        animate={{ opacity: 1, x: 0 }}
        transition={Spring.smooth(0.3)}
        className="p-6 space-y-6 min-w-0 w-full"
      >
        {/* Header */}
        <div>
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1 min-w-0">
              <h2 className="text-xl font-semibold font-mono truncate">{displayName}</h2>
              {serverName && (
                <p className="text-xs text-muted-foreground mt-1">
                  from {serverName}
                </p>
              )}
              {qualifiedName !== displayName && (
                <p className="text-xs text-muted-foreground mt-1">
                  Qualified name <code className="font-mono">{qualifiedName}</code>
                </p>
              )}
            </div>
          </div>
          {schema.description && (
            <p className="text-sm text-muted-foreground mt-3 leading-relaxed">
              {schema.description}
            </p>
          )}
        </div>

        {/* Parameters */}
        <div>
          <h3 className="text-sm font-semibold mb-3 flex items-center gap-2">
            Parameters
            {hasParams && (
              <Badge variant="secondary" className="text-xs font-normal">
                {Object.keys(properties).length}
              </Badge>
            )}
          </h3>

          {hasParams ? (
            <div className="space-y-2">
              {Object.entries(properties).map(([name, prop]) => {
                const isRequired = required.includes(name)

                return (
                  <Card key={name} className="p-3 shadow-none">
                    <div className="flex items-start gap-3">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <code className="text-sm font-semibold">{name}</code>
                          <Badge
                            variant="outline"
                            className="text-xs font-mono"
                          >
                            {prop.type || 'any'}
                          </Badge>
                          {isRequired && (
                            <Badge className="text-xs">required</Badge>
                          )}
                        </div>
                        {prop.description && (
                          <p className="text-xs text-muted-foreground mt-1.5 leading-relaxed">
                            {prop.description}
                          </p>
                        )}
                        {prop.enum && (
                          <div className="flex flex-wrap gap-1 mt-2">
                            {prop.enum.map(val => (
                              <Badge key={val} variant="secondary" className="text-xs font-mono">
                                {val}
                              </Badge>
                            ))}
                          </div>
                        )}
                        {prop.default !== undefined && (
                          <p className="text-xs text-muted-foreground mt-1">
                            Default: <code className="bg-muted px-1 rounded">{JSON.stringify(prop.default)}</code>
                          </p>
                        )}
                      </div>
                    </div>
                  </Card>
                )
              })}
            </div>
          ) : (
            <Card className="p-4">
              <p className="text-sm text-muted-foreground text-center">
                This tool has no parameters
              </p>
            </Card>
          )}
        </div>

        {/* Raw Schema */}
        <Collapsible open={schemaOpen} onOpenChange={setSchemaOpen}>
          <CollapsibleTrigger className="w-full">
            <Button variant="ghost" className="w-full justify-between h-auto py-2 px-3">
              <span className="flex items-center gap-2 text-sm font-medium">
                <CodeIcon className="size-4" />
                Raw Schema
              </span>
              <m.div
                animate={{ rotate: schemaOpen ? 180 : 0 }}
                transition={{ duration: 0.2 }}
              >
                <ChevronDownIcon className="size-4" />
              </m.div>
            </Button>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <m.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.2 }}
              className="mt-2"
            >
              <Card className="relative">
                <Button
                  variant="ghost"
                  size="sm"
                  className="absolute top-2 right-2 h-8 w-8 p-0"
                  onClick={handleCopySchema}
                >
                  {copied ? (
                    <CheckIcon className="size-4 text-green-500" />
                  ) : (
                    <CopyIcon className="size-4" />
                  )}
                </Button>
                <pre className="font-mono text-xs p-4 overflow-auto max-h-80 text-muted-foreground">
                  {JSON.stringify(schema, null, 2)}
                </pre>
              </Card>
            </m.div>
          </CollapsibleContent>
        </Collapsible>
      </m.div>
    </ScrollArea>
  )
}
