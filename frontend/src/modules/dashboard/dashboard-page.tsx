// Input: Dashboard components, tabs/alerts/buttons, core/app hooks, analytics
// Output: DashboardPage component - main dashboard view with insights
// Position: Main dashboard page in dashboard module

import { DebugService } from '@bindings/mcpv/internal/ui/services'
import type { DiagnosticsExportOptions } from '@bindings/mcpv/internal/ui/types'
import {
  AlertCircleIcon,
  FileDownIcon,
  Loader2Icon,
  PlayIcon,
  RefreshCwIcon,
  ServerIcon,
  SquareIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useCallback, useState } from 'react'

import { ConnectIdeSheet } from '@/components/common/connect-agent-sheet'
import { UniversalEmptyState } from '@/components/common/universal-empty-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button, buttonVariants } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogPanel,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Menu,
  MenuItem,
  MenuPopup,
  MenuSeparator,
  MenuTrigger,
} from '@/components/ui/menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { toastManager } from '@/components/ui/toast'
import { useCoreActions, useCoreState } from '@/hooks/use-core-state'
import { AnalyticsEvents, track } from '@/lib/analytics'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'

import { ActiveClientsPanel } from './components/active-clients-panel'
import { ActivityInsights } from './components/activity-insights'
import { BootstrapProgressPanel } from './components/bootstrap-progress'
import { ServerHealthOverview } from './components/server-health-overview'
import { StatusCards } from './components/status-cards'
import { useAppInfo, useBootstrapProgress } from './hooks'

function DashboardHeader() {
  const { appInfo } = useAppInfo()
  const { coreStatus } = useCoreState()
  const { refreshCoreState, restartCore, startCore, stopCore } = useCoreActions()
  const [isExporting, setIsExporting] = useState(false)
  const [debugData, setDebugData] = useState<string | null>(null)
  const [diagnosticsOpen, setDiagnosticsOpen] = useState(false)
  const [diagnosticsData, setDiagnosticsData] = useState<string | null>(null)
  const [diagnosticsMeta, setDiagnosticsMeta] = useState<{ generatedAt: string, size: number } | null>(null)
  const [isDiagnosticsExporting, setIsDiagnosticsExporting] = useState(false)
  const [diagnosticsOptions, setDiagnosticsOptions] = useState<DiagnosticsExportOptions>({
    mode: 'safe',
    includeSnapshot: true,
    includeMetrics: true,
    includeLogs: true,
    includeEvents: true,
    includeStuck: true,
    logLevel: 'info',
    maxLogEntries: 200,
    maxEventEntries: 2000,
    stuckThresholdMs: 30_000,
  })

  const appLabel = appInfo?.name
    ? `${appInfo.name} · ${appInfo.version === 'dev' ? 'dev' : `v${appInfo.version}`} (${appInfo.build})`
    : 'mcpv'

  const handleStartCore = useCallback(async () => {
    try {
      await startCore()
      track(AnalyticsEvents.CORE_START, { result: 'success' })
    }
    catch {
      track(AnalyticsEvents.CORE_START, { result: 'error' })
    }
  }, [startCore])

  const handleStopCore = useCallback(async () => {
    try {
      await stopCore()
      track(AnalyticsEvents.CORE_STOP, { result: 'success' })
    }
    catch {
      track(AnalyticsEvents.CORE_STOP, { result: 'error' })
    }
  }, [stopCore])

  const handleRestartCore = useCallback(async () => {
    try {
      await restartCore()
      track(AnalyticsEvents.CORE_RESTART, { result: 'success' })
    }
    catch {
      track(AnalyticsEvents.CORE_RESTART, { result: 'error' })
    }
  }, [restartCore])

  const handleRefreshCoreState = useCallback(async () => {
    try {
      await refreshCoreState()
      track(AnalyticsEvents.CORE_STATE_REFRESH, { result: 'success' })
    }
    catch {
      track(AnalyticsEvents.CORE_STATE_REFRESH, { result: 'error' })
    }
  }, [refreshCoreState])

  const handleExportDebug = async () => {
    if (isExporting) {
      return
    }
    setIsExporting(true)
    try {
      const result = await DebugService.ExportDebugSnapshot()
      const payload = JSON.stringify(result.snapshot, null, 2)

      // Try clipboard first
      try {
        await navigator.clipboard.writeText(payload)
        toastManager.add({
          type: 'success',
          title: 'Debug snapshot copied',
          description: 'Copied to clipboard',
        })
        track(AnalyticsEvents.DEBUG_SNAPSHOT_EXPORT, { result: 'clipboard' })
      }
      catch {
        // Fallback: show dialog for manual copy
        setDebugData(payload)
        track(AnalyticsEvents.DEBUG_SNAPSHOT_EXPORT, { result: 'dialog' })
      }
    }
    catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Export failed',
        description: err instanceof Error ? err.message : 'Export failed',
      })
      track(AnalyticsEvents.DEBUG_SNAPSHOT_EXPORT, { result: 'error' })
    }
    finally {
      setIsExporting(false)
    }
  }

  const handleExportDiagnostics = async () => {
    if (isDiagnosticsExporting) {
      return
    }
    setIsDiagnosticsExporting(true)
    try {
      const result = await DebugService.ExportDiagnosticsBundle(diagnosticsOptions)
      const parsedPayload = parseDiagnosticsPayload(result.payload)
      const report = typeof parsedPayload?.report === 'string' ? parsedPayload.report : ''
      const rawPayload = stripDiagnosticsReport(parsedPayload)
      const rawJson = JSON.stringify(rawPayload ?? {}, null, 2)
      const formatted = report ? `${report}\n${rawJson}` : rawJson
      setDiagnosticsData(formatted)
      setDiagnosticsMeta({ generatedAt: result.generatedAt, size: result.size })
      toastManager.add({
        type: 'success',
        title: 'Diagnostics exported',
        description: 'Diagnostics bundle is ready',
      })
    }
    catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Diagnostics export failed',
        description: err instanceof Error ? err.message : 'Export failed',
      })
    }
    finally {
      setIsDiagnosticsExporting(false)
    }
  }

  const handleDiagnosticsOpenChange = (open: boolean) => {
    if (!open) {
      setDiagnosticsData(null)
      setDiagnosticsMeta(null)
    }
    setDiagnosticsOpen(open)
  }

  const handleCopyDiagnostics = async () => {
    if (!diagnosticsData) {
      return
    }
    try {
      await navigator.clipboard.writeText(diagnosticsData)
      toastManager.add({
        type: 'success',
        title: 'Copied',
        description: 'Diagnostics copied to clipboard',
      })
    }
    catch {
      toastManager.add({
        type: 'error',
        title: 'Copy failed',
        description: 'Please copy manually',
      })
    }
  }

  const handleDownloadDiagnostics = () => {
    if (!diagnosticsData || !diagnosticsMeta) {
      return
    }
    const safeTimestamp = diagnosticsMeta.generatedAt.replaceAll(':', '-').replaceAll('.', '-')
    const fileName = `mcpv-diagnostics-${safeTimestamp}.log`
    const blob = new Blob([diagnosticsData], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
  }

  const updateDiagnosticsOption = <K extends keyof DiagnosticsExportOptions>(
    key: K,
    value: DiagnosticsExportOptions[K],
  ) => {
    setDiagnosticsOptions(prev => ({ ...prev, [key]: value }))
  }

  const handleCopyFromDialog = () => {
    if (!debugData) {
      return
    }

    const textarea = document.createElement('textarea')
    textarea.value = debugData
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.append(textarea)
    textarea.select()
    try {
      document.execCommand('copy')
      toastManager.add({
        type: 'success',
        title: 'Copied',
        description: 'Debug snapshot copied to clipboard',
      })
      track(AnalyticsEvents.DEBUG_SNAPSHOT_EXPORT, { result: 'dialog_copy' })
      setDebugData(null)
    }
    catch {
      toastManager.add({
        type: 'error',
        title: 'Copy failed',
        description: 'Please copy manually',
      })
    }
    finally {
      textarea.remove()
    }
  }

  const parseDiagnosticsPayload = (payload: unknown): Record<string, unknown> | null => {
    if (!payload) {
      return null
    }
    if (typeof payload === 'string') {
      try {
        return JSON.parse(payload) as Record<string, unknown>
      }
      catch {
        return null
      }
    }
    if (typeof payload === 'object') {
      return payload as Record<string, unknown>
    }
    return null
  }

  const stripDiagnosticsReport = (payload: Record<string, unknown> | null): Record<string, unknown> | null => {
    if (!payload) {
      return null
    }
    if (!('report' in payload)) {
      return payload
    }
    const { report: _report, ...rest } = payload
    return rest
  }

  return (
    <m.div
      initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={Spring.smooth(0.4)}
      className="flex items-center justify-between"
    >
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-muted-foreground text-sm">{appLabel}</p>
      </div>

      <div className="flex items-center gap-2">
        {coreStatus === 'stopped'
          ? (
            <Button onClick={() => void handleStartCore()} size="sm">
              <PlayIcon className="size-4" />
              Start Core
            </Button>
          )
          : coreStatus === 'starting'
            ? (
              <Button onClick={() => void handleStopCore()} variant="outline" size="sm">
                <SquareIcon className="size-4" />
                Cancel
              </Button>
            )
            : coreStatus === 'stopping'
              ? (
                <Button variant="outline" size="sm" disabled>
                  <Loader2Icon className="size-4 animate-spin" />
                  Stopping...
                </Button>
              )
              : coreStatus === 'running'
                ? (
                  <>
                    <Button onClick={() => void handleStopCore()} variant="outline" size="sm">
                      <SquareIcon className="size-4" />
                      Stop
                    </Button>
                    <Button onClick={() => void handleRestartCore()} variant="outline" size="sm">
                      <RefreshCwIcon className="size-4" />
                      Restart
                    </Button>
                  </>
                )
                : coreStatus === 'error'
                  ? (
                    <>
                      <Button onClick={() => void handleRestartCore()} size="sm">
                        <RefreshCwIcon className="size-4" />
                        Retry
                      </Button>
                      <Button onClick={() => void handleStopCore()} variant="outline" size="sm">
                        <SquareIcon className="size-4" />
                        Stop
                      </Button>
                    </>
                  )
                  : null}

        <Menu>
          <MenuTrigger
            className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}
            disabled={isExporting || isDiagnosticsExporting}
          >
            <FileDownIcon className="size-4" />
            Export
          </MenuTrigger>
          <MenuPopup align="end">
            <MenuItem
              disabled={isExporting}
              onClick={() => void handleExportDebug()}
            >
              Copy debug snapshot
            </MenuItem>
            <MenuSeparator />
            <MenuItem
              disabled={isDiagnosticsExporting}
              onClick={() => handleDiagnosticsOpenChange(true)}
            >
              Export diagnostics bundle
            </MenuItem>
          </MenuPopup>
        </Menu>

        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => void handleRefreshCoreState()}
        >
          <RefreshCwIcon className="size-4" />
        </Button>

        <ConnectIdeSheet />
      </div>

      <Dialog open={!!debugData} onOpenChange={open => !open && setDebugData(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Debug Snapshot</DialogTitle>
            <DialogDescription>
              Copy the debug information below to share with support or for troubleshooting.
            </DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <pre className="rounded-lg bg-muted p-4 text-xs overflow-x-auto">
              <code>{debugData}</code>
            </pre>
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>
              Close
            </DialogClose>
            <Button onClick={handleCopyFromDialog}>
              Copy to Clipboard
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={diagnosticsOpen} onOpenChange={handleDiagnosticsOpenChange}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>Diagnostics Export</DialogTitle>
            <DialogDescription>
              Export a diagnostics bundle to troubleshoot stuck initialization or transport issues.
            </DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <div className="grid gap-6 lg:grid-cols-[1.1fr_0.9fr]">
              <div className="space-y-6">
                <div className="rounded-xl border bg-muted/10 p-4 space-y-4">
                  <div>
                    <p className="text-sm font-semibold text-foreground">Export summary</p>
                    <p className="text-xs text-muted-foreground">
                      Human-readable report first, raw JSON appended.
                    </p>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="space-y-1">
                        <Label htmlFor="diagnostics-mode">Redaction mode</Label>
                      <Select
                        value={diagnosticsOptions.mode ?? 'safe'}
                        onValueChange={value => updateDiagnosticsOption('mode', value ?? undefined)}
                      >
                        <SelectTrigger id="diagnostics-mode">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="safe">Safe (redacted)</SelectItem>
                          <SelectItem value="deep">Deep (may include secrets)</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="diagnostics-log-level">Log level</Label>
                      <Select
                        value={diagnosticsOptions.logLevel ?? 'info'}
                        onValueChange={value => updateDiagnosticsOption('logLevel', value ?? undefined)}
                      >
                        <SelectTrigger id="diagnostics-log-level">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="debug">Debug</SelectItem>
                          <SelectItem value="info">Info</SelectItem>
                          <SelectItem value="notice">Notice</SelectItem>
                          <SelectItem value="warning">Warning</SelectItem>
                          <SelectItem value="error">Error</SelectItem>
                          <SelectItem value="critical">Critical</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>

                <div className="rounded-xl border bg-muted/10 p-4 space-y-4">
                  <p className="text-sm font-semibold text-foreground">Include data</p>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="flex items-center justify-between gap-3 rounded-lg border bg-background px-3 py-2">
                      <Label htmlFor="diagnostics-include-snapshot">Snapshot</Label>
                      <Switch
                        id="diagnostics-include-snapshot"
                        checked={diagnosticsOptions.includeSnapshot === true}
                        onCheckedChange={checked => updateDiagnosticsOption('includeSnapshot', checked === true)}
                      />
                    </div>
                    <div className="flex items-center justify-between gap-3 rounded-lg border bg-background px-3 py-2">
                      <Label htmlFor="diagnostics-include-events">Events</Label>
                      <Switch
                        id="diagnostics-include-events"
                        checked={diagnosticsOptions.includeEvents === true}
                        onCheckedChange={checked => updateDiagnosticsOption('includeEvents', checked === true)}
                      />
                    </div>
                    <div className="flex items-center justify-between gap-3 rounded-lg border bg-background px-3 py-2">
                      <Label htmlFor="diagnostics-include-logs">Logs</Label>
                      <Switch
                        id="diagnostics-include-logs"
                        checked={diagnosticsOptions.includeLogs === true}
                        onCheckedChange={checked => updateDiagnosticsOption('includeLogs', checked === true)}
                      />
                    </div>
                    <div className="flex items-center justify-between gap-3 rounded-lg border bg-background px-3 py-2">
                      <Label htmlFor="diagnostics-include-metrics">Metrics</Label>
                      <Switch
                        id="diagnostics-include-metrics"
                        checked={diagnosticsOptions.includeMetrics === true}
                        onCheckedChange={checked => updateDiagnosticsOption('includeMetrics', checked === true)}
                      />
                    </div>
                    <div className="flex items-center justify-between gap-3 rounded-lg border bg-background px-3 py-2 sm:col-span-2">
                      <Label htmlFor="diagnostics-include-stuck">Stuck analysis</Label>
                      <Switch
                        id="diagnostics-include-stuck"
                        checked={diagnosticsOptions.includeStuck === true}
                        onCheckedChange={checked => updateDiagnosticsOption('includeStuck', checked === true)}
                      />
                    </div>
                  </div>
                </div>

                <div className="rounded-xl border bg-muted/10 p-4 space-y-4">
                  <p className="text-sm font-semibold text-foreground">Limits</p>
                  <div className="grid gap-4 md:grid-cols-3">
                    <div className="space-y-1">
                      <Label htmlFor="diagnostics-max-logs">Max log entries</Label>
                      <Input
                        id="diagnostics-max-logs"
                        type="number"
                        min={0}
                        value={diagnosticsOptions.maxLogEntries ?? 0}
                        onChange={(event) => {
                          const value = Number(event.target.value)
                          updateDiagnosticsOption('maxLogEntries', Number.isNaN(value) ? 0 : value)
                        }}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="diagnostics-max-events">Max event entries</Label>
                      <Input
                        id="diagnostics-max-events"
                        type="number"
                        min={0}
                        value={diagnosticsOptions.maxEventEntries ?? 0}
                        onChange={(event) => {
                          const value = Number(event.target.value)
                          updateDiagnosticsOption('maxEventEntries', Number.isNaN(value) ? 0 : value)
                        }}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="diagnostics-stuck-threshold">Stuck threshold (ms)</Label>
                      <Input
                        id="diagnostics-stuck-threshold"
                        type="number"
                        min={0}
                        value={diagnosticsOptions.stuckThresholdMs ?? 0}
                        onChange={(event) => {
                          const value = Number(event.target.value)
                          updateDiagnosticsOption('stuckThresholdMs', Number.isNaN(value) ? 0 : value)
                        }}
                      />
                    </div>
                  </div>
                </div>

                {diagnosticsOptions.mode === 'deep'
                  ? (
                    <Alert variant="warning">
                      <AlertCircleIcon className="size-4" />
                      <AlertTitle>Deep export may include secrets</AlertTitle>
                      <AlertDescription>
                        Use deep mode only when needed. Review the bundle before sharing.
                      </AlertDescription>
                    </Alert>
                  )
                  : null}
              </div>

              <div className="flex h-full flex-col gap-4 rounded-xl border bg-background/40 p-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-semibold">Preview</p>
                    <p className="text-xs text-muted-foreground">Human-readable report with raw data appended.</p>
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {diagnosticsMeta ? `${Math.round(diagnosticsMeta.size / 1024)} KB` : '--'}
                  </span>
                </div>
                <div className="flex-1 overflow-hidden max-w-80 max-h-115">
                  {diagnosticsData
                    ? (
                      <pre className="h-full rounded-lg bg-muted/40 p-3 text-[11px] leading-relaxed overflow-auto">
                        <code>{diagnosticsData}</code>
                      </pre>
                    )
                    : (
                      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-muted-foreground/30 text-xs text-muted-foreground">
                        Run export to preview the report.
                      </div>
                    )}
                </div>
              </div>
            </div>
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>
              Close
            </DialogClose>
            {diagnosticsData
              ? (
                <>
                  <Button
                    variant="outline"
                    onClick={() => void handleExportDiagnostics()}
                    disabled={isDiagnosticsExporting}
                  >
                    {isDiagnosticsExporting ? 'Exporting...' : 'Export Again'}
                  </Button>
                  <Button variant="outline" onClick={handleCopyDiagnostics}>
                    Copy to Clipboard
                  </Button>
                  <Button onClick={handleDownloadDiagnostics}>
                    Download JSON
                  </Button>
                </>
              )
              : (
                <Button onClick={() => void handleExportDiagnostics()} disabled={isDiagnosticsExporting}>
                  {isDiagnosticsExporting ? 'Exporting...' : 'Export Diagnostics'}
                </Button>
              )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </m.div>
  )
}

function DashboardInsights() {
  return (
    <m.div
      initial={{ opacity: 0, y: 10, filter: 'blur(4px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={Spring.smooth(0.4, 0.1)}
      className="grid gap-4 lg:grid-cols-3"
    >
      <div className="lg:col-span-2 space-y-4">
        <ServerHealthOverview />
      </div>
      <div className="space-y-4">
        <ActiveClientsPanel />
      </div>
      <div className="lg:col-span-3">
        <ActivityInsights />
      </div>
    </m.div>
  )
}

function DashboardContent() {
  return (
    <m.div
      initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={Spring.smooth(0.4)}
      className="space-y-6"
    >
      <StatusCards />
      <DashboardInsights />
    </m.div>
  )
}

function StartingContent() {
  const { state, total } = useBootstrapProgress()

  if (total > 0 || state === 'running') {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-6">
        <BootstrapProgressPanel className="w-full max-w-md" />
      </div>
    )
  }

  return (
    <UniversalEmptyState
      icon={Loader2Icon}
      iconClassName="animate-spin"
      title="Starting Core..."
      description="Please wait while the mcpv core is initializing."
    />
  )
}

export function DashboardPage() {
  const { coreStatus, data: coreState } = useCoreState()
  const { startCore } = useCoreActions()

  const handleStartCore = useCallback(async () => {
    try {
      await startCore()
      track(AnalyticsEvents.CORE_START, { result: 'success' })
    }
    catch {
      track(AnalyticsEvents.CORE_START, { result: 'error' })
    }
  }, [startCore])

  if (coreStatus === 'stopped') {
    return (
      <div className="flex flex-1 flex-col p-6 overflow-auto">
        <DashboardHeader />
        <Separator className="my-6" />
        <UniversalEmptyState
          icon={ServerIcon}
          title="Core is not running"
          description="Start the mcpv core to see your dashboard and manage MCP servers."
          action={{
            label: 'Start Core',
            onClick: () => void handleStartCore(),
          }}
        />
      </div>
    )
  }

  if (coreStatus === 'starting') {
    return (
      <div className="flex flex-1 flex-col p-6 overflow-auto">
        <DashboardHeader />
        <Separator className="my-6" />
        <StartingContent />
      </div>
    )
  }

  if (coreStatus === 'error') {
    return (
      <div className="flex flex-1 flex-col p-6 overflow-auto">
        <DashboardHeader />
        <Separator className="my-6" />
        <m.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={Spring.smooth(0.4)}
        >
          <Alert variant="error">
            <AlertCircleIcon className="size-4" />
            <AlertTitle>Core Error</AlertTitle>
            <AlertDescription>
              {coreState?.error || 'The mcpv core encountered an error. Check the logs for details.'}
            </AlertDescription>
          </Alert>
        </m.div>
      </div>
    )
  }

  return (
    <div className="flex flex-1 flex-col p-6 overflow-scroll">
      <DashboardHeader />
      <Separator className="my-6" />
      <DashboardContent />
    </div>
  )
}
