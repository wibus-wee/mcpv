import { ConfigService } from '@bindings/mcpv/internal/ui/services'
import type { ImportMcpServersRequest } from '@bindings/mcpv/internal/ui/types'
import { AlertCircleIcon, ChevronDownIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { memo, useCallback, useEffect, useMemo, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { TabsContent } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import type { ImportServerDraft } from '../../lib/mcp-import'
import { parseMcpServersJson } from '../../lib/mcp-import'
import { reloadConfig } from '../../lib/reload-config'

type JsonImportTabProps = {
  open: boolean
  isActive: boolean
  isWritable: boolean
  existingServerNames: Set<string>
  mutateServers: () => Promise<unknown>
  onClose: () => void
  onFooterChange: (content: ReactNode | null) => void
  onCountChange: (count: number) => void
}

type JsonServerListProps = {
  servers: ImportServerDraft[]
  onNameChange: (id: string, value: string) => void
}

const JsonServerList = memo(({ servers, onNameChange }: JsonServerListProps) => {
  return (
    <ScrollArea className="flex-1 -mx-1 px-1">
      <div className="space-y-2 pb-2">
        {servers.map(server => (
          <div
            key={server.id}
            className="rounded-lg border bg-card/50 p-3 space-y-2"
          >
            <Input
              value={server.name}
              onChange={event => onNameChange(server.id, event.target.value)}
              placeholder="Server name"
              className="font-mono text-xs h-8"
            />
            <div className="text-[11px] text-muted-foreground font-mono truncate">
              {server.transport === 'streamable_http'
                ? server.http?.endpoint ?? ''
                : server.cmd.join(' ')}
            </div>
            {(server.cwd || Object.keys(server.env).length > 0) && (
              <div className="flex flex-wrap gap-1.5 text-[10px] text-muted-foreground">
                {server.cwd && (
                  <span className="rounded bg-muted px-1.5 py-0.5 font-mono">
                    cwd: {server.cwd}
                  </span>
                )}
                {Object.keys(server.env).length > 0 && (
                  <span className="rounded bg-muted px-1.5 py-0.5 font-mono">
                    {Object.keys(server.env).length} env vars
                  </span>
                )}
              </div>
            )}
          </div>
        ))}
      </div>
    </ScrollArea>
  )
})

JsonServerList.displayName = 'JsonServerList'

export function JsonImportTab({
  open,
  isActive,
  isWritable,
  existingServerNames,
  mutateServers,
  onClose,
  onFooterChange,
  onCountChange,
}: JsonImportTabProps) {
  const [rawInput, setRawInput] = useState('')
  const [servers, setServers] = useState<ImportServerDraft[]>([])
  const [parseErrors, setParseErrors] = useState<string[]>([])
  const [applyError, setApplyError] = useState<string | null>(null)
  const [isApplying, setIsApplying] = useState(false)

  const issues = useMemo(() => {
    if (servers.length === 0) {
      return [] as string[]
    }

    const seen = new Set<string>()
    const duplicates = new Set<string>()
    const conflicts = new Set<string>()
    let hasMissing = false

    servers.forEach((server) => {
      const name = server.name.trim()
      if (!name) {
        hasMissing = true
        return
      }
      if (seen.has(name)) {
        duplicates.add(name)
      }
      else {
        seen.add(name)
      }
      if (existingServerNames.has(name)) {
        conflicts.add(name)
      }
    })

    const nextIssues: string[] = []
    if (hasMissing) {
      nextIssues.push('Server names cannot be empty.')
    }
    if (duplicates.size > 0) {
      nextIssues.push(`Duplicate names: ${Array.from(duplicates).join(', ')}`)
    }
    if (conflicts.size > 0) {
      nextIssues.push(`Conflicts: ${Array.from(conflicts).join(', ')}`)
    }
    return nextIssues
  }, [existingServerNames, servers])

  const canApply = useMemo(() => {
    return isWritable
      && parseErrors.length === 0
      && servers.length > 0
      && issues.length === 0
      && !isApplying
  }, [isApplying, isWritable, issues.length, parseErrors.length, servers.length])

  const handleParse = useCallback(() => {
    const result = parseMcpServersJson(rawInput)
    setServers(result.servers)
    setParseErrors(result.errors)
    setApplyError(null)
  }, [rawInput])

  const handleNameChange = useCallback((id: string, value: string) => {
    setServers(current =>
      current.map(server => (server.id === id ? { ...server, name: value } : server)),
    )
  }, [])

  const handleApply = useCallback(async () => {
    if (!canApply) {
      return
    }

    setIsApplying(true)
    setApplyError(null)

    const payload: ImportMcpServersRequest = {
      servers: servers.map(server => ({
        name: server.name.trim(),
        transport: server.transport,
        cmd: server.cmd,
        env: server.env,
        cwd: server.cwd,
        ...(server.http ? { http: server.http } : {}),
      })),
    }

    try {
      await ConfigService.ImportMcpServers(payload)
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setApplyError(`Reload failed: ${reloadResult.message}`)
        return
      }
      await mutateServers()
      onClose()
    }
    catch (err) {
      const message = err instanceof Error ? err.message : 'Import failed.'
      setApplyError(message)
    }
    finally {
      setIsApplying(false)
    }
  }, [canApply, mutateServers, onClose, servers])

  useEffect(() => {
    if (!open) {
      setRawInput('')
      setServers([])
      setParseErrors([])
      setApplyError(null)
      setIsApplying(false)
    }
  }, [open])

  useEffect(() => {
    onCountChange(servers.length)
  }, [onCountChange, servers.length])

  const footerAction = useMemo(() => {
    return (
      <Button variant="default" onClick={handleApply} disabled={!canApply}>
        {isApplying ? 'Saving...' : 'Apply to config'}
      </Button>
    )
  }, [canApply, handleApply, isApplying])

  useEffect(() => {
    if (!open || !isActive) {
      return
    }
    onFooterChange(footerAction)
  }, [footerAction, isActive, onFooterChange, open])

  return (
    <TabsContent value="json" keepMounted className="flex-1 space-y-4 mt-4 overflow-hidden flex flex-col">
      <div className="space-y-2">
        <Textarea
          value={rawInput}
          onChange={event => setRawInput(event.target.value)}
          placeholder={`Paste mcpServers JSON or a command line:\nnpx -y @example/mcp-server --api-key YOUR_KEY`}
          className="min-h-28 font-mono text-xs resize-none"
        />
        <div className="flex items-center gap-2">
          <Button variant="default" size="sm" onClick={handleParse}>
            Parse
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setRawInput('')
              setServers([])
              setParseErrors([])
            }}
            disabled={!rawInput && servers.length === 0}
          >
            Clear
          </Button>
        </div>
      </div>

      {parseErrors.length > 0 && (
        <div className="flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <AlertCircleIcon className="size-3.5 shrink-0 mt-0.5" />
          <div className="space-y-0.5">
            {parseErrors.map(error => (
              <div key={error}>{error}</div>
            ))}
          </div>
        </div>
      )}

      {applyError && (
        <div className="flex items-center gap-2 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <AlertCircleIcon className="size-3.5" />
          <span>{applyError}</span>
        </div>
      )}

      {servers.length > 0 && (
        <JsonServerList servers={servers} onNameChange={handleNameChange} />
      )}

      {issues.length > 0 && (
        <Collapsible defaultOpen>
          <CollapsibleTrigger className="flex items-center gap-1.5 text-xs text-warning hover:text-warning/80 transition-colors group">
            <ChevronDownIcon className="size-3.5 transition-transform group-data-[state=open]:rotate-180" />
            <span>Resolve {issues.length} issue{issues.length !== 1 && 's'} before importing</span>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div className="mt-2 space-y-1 rounded-md bg-warning/10 p-2">
              {issues.map(issue => (
                <div key={issue} className="text-[11px] text-warning">
                  {issue}
                </div>
              ))}
            </div>
          </CollapsibleContent>
        </Collapsible>
      )}
    </TabsContent>
  )
}
