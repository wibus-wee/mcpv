import { ConfigService, McpTransferService } from '@bindings/mcpv/internal/ui/services'
import type { ImportMcpServersRequest, McpTransferIssue } from '@bindings/mcpv/internal/ui/types'
import {
  AlertCircleIcon,
  ArrowDownToLineIcon,
  CheckCircle2Icon,
  ChevronDownIcon,
  InfoIcon,
  RefreshCwIcon,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { memo, useCallback, useEffect, useMemo, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Spinner } from '@/components/ui/spinner'
import { TabsContent } from '@/components/ui/tabs'

import { reloadConfig } from '../../lib/reload-config'
import type { MergedServer, TransferSource, TransferState, TransferSummary } from './types'

const transferSources: TransferSource[] = ['claude', 'codex', 'gemini']

const sourceLabels: Record<TransferSource, string> = {
  claude: 'Claude',
  codex: 'Codex',
  gemini: 'Gemini',
}

const buildInitialTransferState = (): Record<TransferSource, TransferState> => ({
  claude: { status: 'idle' },
  codex: { status: 'idle' },
  gemini: { status: 'idle' },
})

const formatTransferError = (err: unknown) => {
  if (err instanceof Error) {
    return err.message
  }
  return String(err ?? 'Preview failed.')
}

const classifyTransferError = (message: string): TransferState['status'] => {
  const normalized = message.toLowerCase()
  if (normalized.includes('not found') || normalized.includes('not_found')) {
    return 'missing'
  }
  return 'error'
}

type IdeImportTabProps = {
  open: boolean
  isActive: boolean
  isWritable: boolean
  existingServerNames: Set<string>
  mutateServers: () => Promise<unknown>
  onFooterChange: (content: ReactNode | null) => void
  onCountChange: (count: number) => void
}

type TransferStatusRowProps = {
  previewing: boolean
  mergedCount: number
  sourceCount: number
}

const TransferStatusRow = memo(({ previewing, mergedCount, sourceCount }: TransferStatusRowProps) => {
  if (previewing) {
    return (
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <Spinner className="size-3.5" />
        <span>Scanning IDE configs...</span>
      </div>
    )
  }

  if (mergedCount > 0) {
    return (
      <span className="text-xs text-muted-foreground">
        Found {mergedCount} server{mergedCount !== 1 && 's'} from {sourceCount} source{sourceCount !== 1 && 's'}
      </span>
    )
  }

  return (
    <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
      <InfoIcon className="size-3.5" />
      No MCP configs found in Claude, Codex, or Gemini
    </span>
  )
})

TransferStatusRow.displayName = 'TransferStatusRow'

type IdeServerListProps = {
  servers: MergedServer[]
  selectedCount: number
  existingServerNames: Set<string>
  onToggleServer: (name: string, source: TransferSource) => void
  onSelectAll: (selected: boolean) => void
}

const IdeServerList = memo(({
  servers,
  selectedCount,
  existingServerNames,
  onToggleServer,
  onSelectAll,
}: IdeServerListProps) => {
  const allSelected = selectedCount > 0 && selectedCount === servers.length
  const indeterminate = selectedCount > 0 && selectedCount < servers.length

  return (
    <div className="flex flex-col gap-2 flex-1 min-h-0">
      <div className="flex items-center justify-between px-1">
        <label className="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer">
          <Checkbox
            checked={allSelected}
            indeterminate={indeterminate}
            onCheckedChange={checked => onSelectAll(Boolean(checked))}
          />
          {selectedCount > 0 ? `${selectedCount} selected` : 'Select all'}
        </label>
      </div>
      <ScrollArea className="flex-1 -mx-1 px-1">
        <div className="space-y-2 pb-2">
          {servers.map(server => (
            <label
              key={`${server.source}-${server.name}`}
              className="flex items-start gap-3 rounded-lg border bg-card/50 p-3 cursor-pointer hover:bg-accent/50 transition-colors"
            >
              <Checkbox
                checked={server.selected}
                onCheckedChange={() => onToggleServer(server.name, server.source)}
                className="mt-0.5"
              />
              <div className="flex-1 min-w-0 space-y-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium truncate">{server.name}</span>
                  <Badge variant="outline" size="sm" className="shrink-0">
                    {sourceLabels[server.source]}
                  </Badge>
                  {existingServerNames.has(server.name) && (
                    <Badge variant="warning" size="sm" className="shrink-0">
                      Conflict
                    </Badge>
                  )}
                </div>
                <div className="text-[11px] text-muted-foreground font-mono truncate">
                  {server.transport === 'streamable_http'
                    ? server.http?.endpoint ?? '—'
                    : (server.cmd ?? []).join(' ')}
                </div>
              </div>
            </label>
          ))}
        </div>
      </ScrollArea>
    </div>
  )
})

IdeServerList.displayName = 'IdeServerList'

export function IdeImportTab({
  open,
  isActive,
  isWritable,
  existingServerNames,
  mutateServers,
  onFooterChange,
  onCountChange,
}: IdeImportTabProps) {
  const [transferState, setTransferState] = useState(buildInitialTransferState)
  const [mergedServers, setMergedServers] = useState<MergedServer[]>([])
  const [transferSummary, setTransferSummary] = useState<TransferSummary | null>(null)
  const [transferError, setTransferError] = useState<string | null>(null)
  const [isTransferImporting, setIsTransferImporting] = useState(false)

  const transferPreviewing = useMemo(() =>
    transferSources.some(source => transferState[source].status === 'loading'), [transferState])

  const allIssues = useMemo(() => {
    const collected: Array<McpTransferIssue & { source: TransferSource }> = []
    transferSources.forEach((source) => {
      const { preview } = transferState[source]
      if (preview?.issues) {
        preview.issues.forEach(issue => collected.push({ ...issue, source }))
      }
    })
    return collected
  }, [transferState])

  const selectedCount = useMemo(() =>
    mergedServers.filter(server => server.selected).length, [mergedServers])

  const sourceCount = useMemo(() => {
    if (mergedServers.length === 0) {
      return 0
    }
    return new Set(mergedServers.map(server => server.source)).size
  }, [mergedServers])

  const canTransferImport = isWritable
    && !isTransferImporting
    && selectedCount > 0
    && !transferPreviewing

  const refreshTransferPreviews = useCallback(async () => {
    setTransferError(null)
    setTransferSummary(null)
    setTransferState((prev) => {
      const next = { ...prev }
      transferSources.forEach((source) => {
        next[source] = { status: 'loading' }
      })
      return next
    })

    const results = await Promise.allSettled(
      transferSources.map(async source => ({
        source,
        preview: await McpTransferService.Preview({ source }),
      })),
    )

    const newState = buildInitialTransferState()
    const newMergedServers: MergedServer[] = []

    results.forEach((result, index) => {
      const source = transferSources[index]
      if (result.status === 'fulfilled') {
        const { preview } = result.value
        const hasServers = (preview.servers?.length ?? 0) > 0
        const hasIssues = (preview.issues?.length ?? 0) > 0
        newState[source] = {
          status: hasServers || hasIssues ? 'ready' : 'empty',
          preview,
        }
        preview.servers?.forEach((server) => {
          const isConflict = existingServerNames.has(server.name)
          newMergedServers.push({
            ...server,
            source,
            selected: !isConflict,
          })
        })
        return
      }

      const message = formatTransferError(result.reason)
      newState[source] = {
        status: classifyTransferError(message),
        error: message,
      }
    })

    setTransferState(newState)
    setMergedServers(newMergedServers)
  }, [existingServerNames])

  const handleToggleServer = useCallback((name: string, source: TransferSource) => {
    setMergedServers(current =>
      current.map(server =>
        server.name === name && server.source === source
          ? { ...server, selected: !server.selected }
          : server,
      ),
    )
  }, [])

  const handleSelectAll = useCallback((selected: boolean) => {
    setMergedServers(current =>
      current.map(server => ({ ...server, selected })),
    )
  }, [])

  const handleTransferImport = useCallback(async () => {
    if (!canTransferImport) {
      return
    }

    const selectedServers = mergedServers.filter(server => server.selected)
    if (selectedServers.length === 0) {
      return
    }

    setIsTransferImporting(true)
    setTransferError(null)
    setTransferSummary(null)

    const payload: ImportMcpServersRequest = {
      servers: selectedServers.map(({ source, selected, ...server }) => ({
        ...server,
        name: server.name.trim(),
      })),
    }

    try {
      await ConfigService.ImportMcpServers(payload)
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setTransferError(`Reload failed: ${reloadResult.message}`)
      }
      await mutateServers()
      setTransferSummary({ imported: selectedServers.length, skipped: 0 })
      setMergedServers(current =>
        current.filter(server => !server.selected),
      )
    }
    catch (err) {
      setTransferError(formatTransferError(err))
    }
    finally {
      setIsTransferImporting(false)
    }
  }, [canTransferImport, mergedServers, mutateServers])

  useEffect(() => {
    if (!open) {
      setTransferState(buildInitialTransferState())
      setMergedServers([])
      setTransferSummary(null)
      setTransferError(null)
      setIsTransferImporting(false)
      return
    }
    void refreshTransferPreviews()
  }, [open, refreshTransferPreviews])

  useEffect(() => {
    onCountChange(mergedServers.length)
  }, [mergedServers.length, onCountChange])

  const importLabel = useMemo(() => {
    if (selectedCount > 0) {
      return `Import ${selectedCount} server${selectedCount !== 1 ? 's' : ''}`
    }
    return 'Import servers'
  }, [selectedCount])

  const footerAction = useMemo(() => {
    return (
      <Button
        variant="default"
        onClick={handleTransferImport}
        disabled={!canTransferImport}
      >
        {isTransferImporting
          ? (
              <>
                <Spinner className="size-4" />
                Importing...
              </>
            )
          : (
              <>
                <ArrowDownToLineIcon className="size-4" />
                {importLabel}
              </>
            )}
      </Button>
    )
  }, [canTransferImport, handleTransferImport, importLabel, isTransferImporting])

  useEffect(() => {
    if (!open || !isActive) {
      return
    }
    onFooterChange(footerAction)
  }, [footerAction, isActive, onFooterChange, open])

  return (
    <TabsContent value="ide" keepMounted className="flex-1 space-y-4 mt-4 overflow-hidden">
      <span className="block mt-1 text-[11px] text-muted-foreground/70">
        This only copies configs to mcpv — your original files won't be modified.
      </span>
      <div className="flex items-center justify-between gap-2">
        <TransferStatusRow
          previewing={transferPreviewing}
          mergedCount={mergedServers.length}
          sourceCount={sourceCount}
        />
        <Button
          variant="ghost"
          size="sm"
          onClick={refreshTransferPreviews}
          disabled={transferPreviewing}
          className="h-7 px-2"
        >
          <RefreshCwIcon className="size-3.5" />
          Refresh
        </Button>
      </div>

      {transferSummary && (
        <div className="flex items-center gap-2 rounded-md bg-success/10 px-3 py-2 text-xs text-success">
          <CheckCircle2Icon className="size-3.5" />
          <span>Imported {transferSummary.imported} server{transferSummary.imported !== 1 && 's'}</span>
          {transferSummary.skipped > 0 && (
            <span className="text-muted-foreground">({transferSummary.skipped} skipped)</span>
          )}
        </div>
      )}

      {transferError && (
        <div className="flex items-center gap-2 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <AlertCircleIcon className="size-3.5" />
          <span>{transferError}</span>
        </div>
      )}

      {mergedServers.length > 0 && (
        <IdeServerList
          servers={mergedServers}
          selectedCount={selectedCount}
          existingServerNames={existingServerNames}
          onToggleServer={handleToggleServer}
          onSelectAll={handleSelectAll}
        />
      )}

      {allIssues.length > 0 && (
        <Collapsible>
          <CollapsibleTrigger className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors group">
            <ChevronDownIcon className="size-3.5 transition-transform group-data-[state=open]:rotate-180" />
            <span>{allIssues.length} issue{allIssues.length !== 1 && 's'} found</span>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div className="mt-2 space-y-1 rounded-md bg-muted/50 p-2">
              {allIssues.map(issue => (
                <div key={`${issue.source}-${issue.kind}-${issue.name ?? ''}-${issue.message}`} className="flex items-start gap-2 text-[11px] text-muted-foreground">
                  <span className="shrink-0">[{sourceLabels[issue.source]}]</span>
                  <span>{issue.name ? `${issue.name}: ` : ''}{issue.message}</span>
                </div>
              ))}
            </div>
          </CollapsibleContent>
        </Collapsible>
      )}
    </TabsContent>
  )
}
