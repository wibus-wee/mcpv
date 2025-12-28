// Input: Server data, tools list, runtime status
// Output: ServerDetailsSheet with nested tool sheets
// Position: Main sheet for server details with tools list

import { useState } from 'react'
import { WrenchIcon } from 'lucide-react'

import type { ToolEntry } from '@bindings/mcpd/internal/ui'

import { Card } from '@/components/ui/card'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetPanel, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { ServerRuntimeDetails } from '@/modules/config/components/server-runtime-status'
import { useRuntimeStatus } from '@/modules/config/hooks'
import { cn } from '@/lib/utils'

import { ToolDetailsSheet } from './tool-details-sheet'

interface ServerDetailsSheetProps {
  specKey: string
  serverName: string
  tools: ToolEntry[]
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ServerDetailsSheet({
  specKey,
  serverName,
  tools,
  open,
  onOpenChange
}: ServerDetailsSheetProps) {
  const [selectedTool, setSelectedTool] = useState<ToolEntry | null>(null)
  const [toolSheetOpen, setToolSheetOpen] = useState(false)
  const { data: runtimeStatus } = useRuntimeStatus()

  const serverStatus = runtimeStatus?.find(s => s.specKey === specKey)

  const handleToolClick = (tool: ToolEntry) => {
    setSelectedTool(tool)
    setToolSheetOpen(true)
  }

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent side="right" className="max-w-[40vw]">
          <SheetHeader>
            <SheetTitle>{serverName}</SheetTitle>
            <SheetDescription>Server tools and runtime status</SheetDescription>
          </SheetHeader>
          <SheetPanel>
            <div className="space-y-6">
              <div>
                <h4 className="text-sm font-semibold mb-3">
                  Tools ({tools.length})
                </h4>
                {tools.length > 0 ? (
                  <div className="space-y-2">
                    {tools.map(tool => (
                      <Card
                        key={tool.name}
                        className={cn(
                          'p-4 cursor-pointer transition-colors',
                          'hover:bg-muted/50'
                        )}
                        onClick={() => handleToolClick(tool)}
                      >
                        <div className="flex items-start gap-3">
                          <WrenchIcon className="size-4 text-muted-foreground mt-0.5 shrink-0" />
                          <div className="flex-1 min-w-0">
                            <h5 className="font-medium text-sm">{tool.name}</h5>
                            <p className="text-xs text-muted-foreground mt-1 line-clamp-2">
                              {(() => {
                                try {
                                  const schema = JSON.parse(tool.toolJson as string)
                                  return schema.description || 'No description'
                                } catch {
                                  return 'No description'
                                }
                              })()}
                            </p>
                          </div>
                        </div>
                      </Card>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">No tools available</p>
                )}
              </div>

              {serverStatus && (
                <div>
                  <h4 className="text-sm font-semibold mb-3">Runtime Status</h4>
                  <Card className="p-4">
                    <ServerRuntimeDetails status={serverStatus} />
                  </Card>
                </div>
              )}

              <div>
                <h4 className="text-sm font-semibold mb-3">Configuration</h4>
                <Card className="p-4">
                  <dl className="space-y-2 text-sm">
                    <div>
                      <dt className="text-muted-foreground">Server Key</dt>
                      <dd className="font-mono text-xs mt-1 break-all">{specKey}</dd>
                    </div>
                  </dl>
                </Card>
              </div>
            </div>
          </SheetPanel>
        </SheetContent>
      </Sheet>

      {selectedTool && (
        <ToolDetailsSheet
          tool={selectedTool}
          open={toolSheetOpen}
          onOpenChange={setToolSheetOpen}
        />
      )}
    </>
  )
}
