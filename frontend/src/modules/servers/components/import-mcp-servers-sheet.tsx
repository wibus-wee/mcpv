// Input: MCP JSON payload, IDE config previews, server list, config mode
// Output: ImportMcpServersSheet component - combined JSON + IDE import flow
// Position: Config header action entry

import { FileUpIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useCallback, useEffect, useMemo, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetPanel,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { useConfigMode, useServers } from '../hooks'
import { IdeImportTab } from './import-mcp-servers/ide-import-tab'
import { JsonImportTab } from './import-mcp-servers/json-import-tab'

type ImportTab = 'ide' | 'json'

export const ImportMcpServersSheet = () => {
  const { data: configMode } = useConfigMode()
  const { data: serversList, mutate: mutateServers } = useServers()

  const [open, setOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<ImportTab>('ide')
  const [footerContent, setFooterContent] = useState<ReactNode | null>(null)
  const [jsonCount, setJsonCount] = useState(0)
  const [ideCount, setIdeCount] = useState(0)

  const isWritable = configMode?.isWritable ?? false

  const existingServerNames = useMemo(() => {
    return new Set((serversList ?? []).map(server => server.name))
  }, [serversList])

  const handleTabChange = useCallback((value: string) => {
    setActiveTab(value as ImportTab)
  }, [])

  const handleFooterChange = useCallback((content: ReactNode | null) => {
    setFooterContent(content)
  }, [])

  const handleJsonCountChange = useCallback((count: number) => {
    setJsonCount(count)
  }, [])

  const handleIdeCountChange = useCallback((count: number) => {
    setIdeCount(count)
  }, [])

  const handleClose = useCallback(() => {
    setOpen(false)
  }, [])

  useEffect(() => {
    if (!open) {
      setActiveTab('ide')
      setFooterContent(null)
      setJsonCount(0)
      setIdeCount(0)
    }
  }, [open])

  useEffect(() => {
    setFooterContent(null)
  }, [activeTab])

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger>
        <Button
          variant="secondary"
          size="sm"
          disabled={!isWritable}
          title={isWritable ? 'Import MCP servers' : 'Configuration is not writable'}
        >
          <FileUpIcon className="size-4" />
          Import MCP Servers
        </Button>
      </SheetTrigger>
      <SheetContent side="right">
        <SheetHeader>
          <SheetTitle>Import MCP servers</SheetTitle>
          <SheetDescription>
            Import from local IDE configs or paste JSON to add servers.
          </SheetDescription>
        </SheetHeader>
        <SheetPanel>
          <Tabs value={activeTab} onValueChange={handleTabChange} className="flex flex-col h-full">
            <TabsList className="w-full">
              <TabsTrigger value="json" className="flex-1">
                Paste JSON
                {jsonCount > 0 && (
                  <Badge variant="secondary" size="sm" className="ml-1.5">
                    {jsonCount}
                  </Badge>
                )}
              </TabsTrigger>
              <TabsTrigger value="ide" className="flex-1">
                From IDE
                {ideCount > 0 && (
                  <Badge variant="secondary" size="sm" className="ml-1.5">
                    {ideCount}
                  </Badge>
                )}
              </TabsTrigger>
            </TabsList>

            <JsonImportTab
              open={open}
              isActive={activeTab === 'json'}
              isWritable={isWritable}
              existingServerNames={existingServerNames}
              mutateServers={mutateServers}
              onClose={handleClose}
              onFooterChange={handleFooterChange}
              onCountChange={handleJsonCountChange}
            />
            <IdeImportTab
              open={open}
              isActive={activeTab === 'ide'}
              isWritable={isWritable}
              existingServerNames={existingServerNames}
              mutateServers={mutateServers}
              onFooterChange={handleFooterChange}
              onCountChange={handleIdeCountChange}
            />
          </Tabs>
        </SheetPanel>
        <SheetFooter>
          <Button variant="ghost" onClick={handleClose}>
            Close
          </Button>
          {footerContent}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
