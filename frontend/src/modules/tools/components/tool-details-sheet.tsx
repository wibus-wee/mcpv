// Input: Tool entry with schema
// Output: ToolDetailsSheet component with parameter table
// Position: Nested sheet for tool parameter details

import { useState } from 'react'
import { CodeIcon } from 'lucide-react'

import type { ToolEntry } from '@bindings/mcpd/internal/ui'

import { Badge } from '@/components/ui/badge'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetPanel, SheetTitle } from '@/components/ui/sheet'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

interface ToolDetailsSheetProps {
  tool: ToolEntry
  open: boolean
  onOpenChange: (open: boolean) => void
}

interface ToolSchema {
  name?: string
  description?: string
  inputSchema?: {
    type?: string
    properties?: Record<string, {
      type?: string
      description?: string
    }>
    required?: string[]
  }
}

export function ToolDetailsSheet({ tool, open, onOpenChange }: ToolDetailsSheetProps) {
  const [schemaOpen, setSchemaOpen] = useState(false)

  let schema: ToolSchema = {}
  try {
    schema = JSON.parse(tool.toolJson as string)
  } catch {
    schema = {}
  }

  const properties = schema.inputSchema?.properties || {}
  const required = schema.inputSchema?.required || []

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="max-w-[35vw]">
        <SheetHeader>
          <SheetTitle>{tool.name}</SheetTitle>
          <SheetDescription>
            {schema.description || 'No description available'}
          </SheetDescription>
        </SheetHeader>
        <SheetPanel>
          <div className="space-y-6">
            <div>
              <h4 className="text-sm font-semibold mb-3">Parameters</h4>
              {Object.keys(properties).length > 0 ? (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Type</TableHead>
                      <TableHead>Required</TableHead>
                      <TableHead>Description</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {Object.entries(properties).map(([name, prop]) => (
                      <TableRow key={name}>
                        <TableCell className="font-mono text-xs">{name}</TableCell>
                        <TableCell className="text-xs">{prop.type || 'any'}</TableCell>
                        <TableCell>
                          <Badge variant={required.includes(name) ? 'default' : 'secondary'} className="text-xs">
                            {required.includes(name) ? 'Yes' : 'No'}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {prop.description || '-'}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              ) : (
                <p className="text-sm text-muted-foreground">No parameters</p>
              )}
            </div>

            <Collapsible open={schemaOpen} onOpenChange={setSchemaOpen}>
              <CollapsibleTrigger className="flex items-center gap-2 text-sm font-medium hover:text-primary transition-colors">
                <CodeIcon className="size-4" />
                View Raw Schema
              </CollapsibleTrigger>
              <CollapsibleContent className="mt-2">
                <pre className="font-mono text-xs bg-muted p-4 rounded-lg overflow-auto max-h-96">
                  {JSON.stringify(schema, null, 2)}
                </pre>
              </CollapsibleContent>
            </Collapsible>
          </div>
        </SheetPanel>
      </SheetContent>
    </Sheet>
  )
}
