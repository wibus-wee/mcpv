// Input: TanStack Router, SystemService/DebugService bindings, analytics toggle, settings hooks
// Output: Advanced settings page with telemetry, updates, tray, and diagnostics tools
// Position: /settings/advanced route

import { DebugService, SystemService } from '@bindings/mcpv/internal/ui/services'
import type { DiagnosticsExportOptions, UpdateCheckResult, UpdateDownloadProgress, UpdateInstallProgress } from '@bindings/mcpv/internal/ui/types'
import { createFileRoute } from '@tanstack/react-router'
import { ClipboardCopyIcon, DownloadIcon, FileDownIcon, RefreshCwIcon } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { toastManager } from '@/components/ui/toast'
import { AnalyticsEvents, toggleAnalytics, track, useAnalyticsEnabledValue } from '@/lib/analytics'
import { formatRelativeTime } from '@/lib/time'
import { useTraySettings } from '@/modules/settings/hooks/use-tray-settings'
import { useUpdateSettings } from '@/modules/settings/hooks/use-update-settings'

const formatVersionLabel = (version?: string | null) => {
  if (!version) return 'unknown'
  return version.startsWith('v') ? version : `v${version}`
}

const formatBytes = (value: number | null | undefined) => {
  const bytes = typeof value === 'number' && Number.isFinite(value) ? value : 0
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = bytes
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  return `${size.toFixed(size >= 10 ? 0 : 1)} ${units[unitIndex]}`
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

export const Route = createFileRoute('/settings/advanced')({
  component: AdvancedSettingsPage,
})

function AdvancedSettingsPage() {
  const analyticsEnabled = useAnalyticsEnabledValue()
  const {
    settings: updateSettings,
    error: updateError,
    isLoading: updateLoading,
    updateSettings: updateUpdateSettings,
  } = useUpdateSettings()

  const [manualCheckResult, setManualCheckResult] = useState<UpdateCheckResult | null>(null)
  const [manualCheckError, setManualCheckError] = useState<string | null>(null)
  const [manualCheckedAt, setManualCheckedAt] = useState<string | null>(null)
  const [isChecking, setIsChecking] = useState(false)
  const [downloadProgress, setDownloadProgress] = useState<UpdateDownloadProgress | null>(null)
  const [isDownloadStarting, setIsDownloadStarting] = useState(false)
  const [installProgress, setInstallProgress] = useState<UpdateInstallProgress | null>(null)
  const [isInstallStarting, setIsInstallStarting] = useState(false)
  const [debugData, setDebugData] = useState<string | null>(null)
  const [snapshotCopiedAt, setSnapshotCopiedAt] = useState<string | null>(null)
  const [isSnapshotExporting, setIsSnapshotExporting] = useState(false)
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

  const {
    settings: traySettings,
    error: trayError,
    isLoading: trayLoading,
    updateSettings: updateTraySettings,
  } = useTraySettings()

  const installStartedRef = useRef<string | null>(null)
  const restartNotifiedRef = useRef<string | null>(null)

  const isMac = useMemo(() => {
    if (typeof navigator === 'undefined') return false
    return /Mac|iPhone|iPad|iPod/.test(navigator.platform)
  }, [])

  const handlePrereleaseToggle = useCallback(async (checked: boolean) => {
    try {
      await updateUpdateSettings({
        ...updateSettings,
        includePrerelease: checked,
      })
    }
    catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Update preference failed',
        description: err instanceof Error ? err.message : 'Unable to update settings',
      })
    }
  }, [updateSettings, updateUpdateSettings])

  const handleOpenRelease = useCallback(async (url: string) => {
    const opened = window.open(url, '_blank', 'noopener,noreferrer')
    if (opened) return
    try {
      await navigator.clipboard.writeText(url)
      toastManager.add({
        type: 'success',
        title: 'Link copied',
        description: 'Download link copied to clipboard',
      })
    }
    catch {
      toastManager.add({
        type: 'error',
        title: 'Open failed',
        description: 'Unable to open the download link',
      })
    }
  }, [])

  const handleCheckNow = useCallback(async () => {
    if (isChecking) return
    setIsChecking(true)
    setManualCheckError(null)
    try {
      const result = await SystemService.CheckForUpdates()
      setManualCheckResult(result)
      setManualCheckedAt(new Date().toISOString())
    }
    catch (err) {
      const message = err instanceof Error ? err.message : 'Unable to check for updates'
      setManualCheckError(message)
      toastManager.add({
        type: 'error',
        title: 'Update check failed',
        description: message,
      })
    }
    finally {
      setIsChecking(false)
    }
  }, [isChecking])

  const manualLatest = manualCheckResult?.latest ?? null
  const manualUpdateAvailable = manualCheckResult?.updateAvailable ?? false

  const handleDownloadUpdate = useCallback(async () => {
    if (!manualLatest?.url || isDownloadStarting) {
      return
    }
    setIsDownloadStarting(true)
    try {
      const progress = await SystemService.StartUpdateDownload({ releaseUrl: manualLatest.url })
      setDownloadProgress(progress)
    }
    catch (err) {
      const message = err instanceof Error ? err.message : 'Unable to start download'
      toastManager.add({
        type: 'error',
        title: 'Download failed',
        description: message,
      })
    }
    finally {
      setIsDownloadStarting(false)
    }
  }, [isDownloadStarting, manualLatest?.url])

  const handleStartInstall = useCallback(async (filePath: string) => {
    if (!filePath || isInstallStarting) {
      return
    }
    setIsInstallStarting(true)
    try {
      const progress = await SystemService.StartUpdateInstall({ filePath })
      setInstallProgress(progress)
    }
    catch (err) {
      const message = err instanceof Error ? err.message : 'Unable to start install'
      toastManager.add({
        type: 'error',
        title: 'Install failed',
        description: message,
      })
    }
    finally {
      setIsInstallStarting(false)
    }
  }, [isInstallStarting])

  useEffect(() => {
    if (downloadProgress?.status !== 'completed') return
    const filePath = downloadProgress.filePath
    if (!filePath) return
    if (installStartedRef.current === filePath) return
    installStartedRef.current = filePath
    void handleStartInstall(filePath)
  }, [downloadProgress?.filePath, downloadProgress?.status, handleStartInstall])

  const handleExportDebug = useCallback(async () => {
    if (isSnapshotExporting) {
      return
    }
    setIsSnapshotExporting(true)
    try {
      const result = await DebugService.ExportDebugSnapshot()
      const payload = JSON.stringify(result.snapshot, null, 2)
      try {
        await navigator.clipboard.writeText(payload)
        toastManager.add({
          type: 'success',
          title: 'Debug snapshot copied',
          description: 'Copied to clipboard',
        })
        setSnapshotCopiedAt(new Date().toISOString())
        track(AnalyticsEvents.DEBUG_SNAPSHOT_EXPORT, { result: 'clipboard' })
      }
      catch {
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
      setIsSnapshotExporting(false)
    }
  }, [isSnapshotExporting])

  const handleExportDiagnostics = useCallback(async () => {
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
  }, [diagnosticsOptions, isDiagnosticsExporting])

  const handleDiagnosticsOpenChange = (open: boolean) => {
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
      setSnapshotCopiedAt(new Date().toISOString())
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

  const updateDiagnosticsOption = <K extends keyof DiagnosticsExportOptions>(
    key: K,
    value: DiagnosticsExportOptions[K],
  ) => {
    setDiagnosticsOptions(prev => ({ ...prev, [key]: value }))
  }

  const handleTrayUpdate = useCallback(async (next: typeof traySettings) => {
    try {
      await updateTraySettings(next)
    }
    catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Tray preference failed',
        description: err instanceof Error ? err.message : 'Unable to update tray settings',
      })
    }
  }, [updateTraySettings])

  const handleTrayToggle = useCallback((checked: boolean) => {
    handleTrayUpdate({
      ...traySettings,
      enabled: checked,
    })
  }, [handleTrayUpdate, traySettings])

  const handleHideDockToggle = useCallback((checked: boolean) => {
    handleTrayUpdate({
      ...traySettings,
      hideDock: checked,
    })
  }, [handleTrayUpdate, traySettings])

  const handleStartHiddenToggle = useCallback((checked: boolean) => {
    handleTrayUpdate({
      ...traySettings,
      startHidden: checked,
    })
  }, [handleTrayUpdate, traySettings])

  const updateDescription = useMemo(() => {
    const intervalHours = updateSettings.intervalHours || 24
    return `Checks for new releases every ${intervalHours} hours.`
  }, [updateSettings.intervalHours])

  const manualStatus = useMemo(() => {
    if (isChecking) {
      return { label: 'Checking', variant: 'info' as const }
    }
    if (manualCheckError) {
      return { label: 'Failed', variant: 'error' as const }
    }
    if (!manualCheckResult) {
      return { label: 'Not checked', variant: 'outline' as const }
    }
    if (manualUpdateAvailable && manualLatest) {
      return { label: 'Update available', variant: 'warning' as const }
    }
    return { label: 'Up to date', variant: 'success' as const }
  }, [isChecking, manualCheckError, manualCheckResult, manualLatest, manualUpdateAvailable])

  const manualStatusDescription = useMemo(() => {
    if (isChecking) {
      return 'Checking GitHub releases for new builds.'
    }
    if (manualCheckError) {
      return manualCheckError
    }
    if (!manualCheckResult) {
      return 'Run a manual check to verify the latest release.'
    }
    if (manualUpdateAvailable && manualLatest) {
      const latestLabel = formatVersionLabel(manualLatest.version)
      const currentLabel = formatVersionLabel(manualCheckResult.currentVersion)
      return `Latest ${latestLabel} is available. Current ${currentLabel}.`
    }
    const currentLabel = formatVersionLabel(manualCheckResult.currentVersion)
    return currentLabel === 'unknown'
      ? 'You are up to date.'
      : `You are up to date (current ${currentLabel}).`
  }, [isChecking, manualCheckError, manualCheckResult, manualLatest, manualUpdateAvailable])

  const isDownloadActive = downloadProgress?.status === 'resolving' || downloadProgress?.status === 'downloading'

  const downloadStatus = useMemo(() => {
    if (!downloadProgress) return null
    if (downloadProgress.status === 'resolving') {
      return { label: 'Preparing', variant: 'info' as const, description: 'Resolving update asset...' }
    }
    if (downloadProgress.status === 'downloading') {
      const bytesLabel = formatBytes(downloadProgress.bytes)
      const totalLabel = downloadProgress.total > 0 ? ` of ${formatBytes(downloadProgress.total)}` : ''
      return { label: 'Downloading', variant: 'info' as const, description: `${bytesLabel}${totalLabel}` }
    }
    if (downloadProgress.status === 'completed') {
      const name = downloadProgress.fileName ? `Saved ${downloadProgress.fileName}.` : 'Download complete.'
      return { label: 'Downloaded', variant: 'success' as const, description: name }
    }
    if (downloadProgress.status === 'failed') {
      return { label: 'Failed', variant: 'error' as const, description: downloadProgress.message || 'Download failed.' }
    }
    return null
  }, [downloadProgress])

  const downloadPercent = useMemo(() => {
    if (!downloadProgress) return 0
    if (downloadProgress.percent > 0) return Math.min(100, downloadProgress.percent)
    if (downloadProgress.total > 0) {
      return Math.min(100, (downloadProgress.bytes / downloadProgress.total) * 100)
    }
    return downloadProgress.status === 'completed' ? 100 : 0
  }, [downloadProgress])

  const isInstallActive = installProgress?.status === 'preparing'
    || installProgress?.status === 'extracting'
    || installProgress?.status === 'validating'
    || installProgress?.status === 'replacing'
    || installProgress?.status === 'cleaning'
    || installProgress?.status === 'restarting'

  const installStatus = useMemo(() => {
    if (!installProgress) return null
    if (installProgress.status === 'preparing') {
      return { label: 'Preparing', variant: 'info' as const, description: 'Preparing installer...' }
    }
    if (installProgress.status === 'extracting') {
      return { label: 'Extracting', variant: 'info' as const, description: 'Extracting update bundle...' }
    }
    if (installProgress.status === 'validating') {
      return { label: 'Validating', variant: 'info' as const, description: 'Validating application bundle...' }
    }
    if (installProgress.status === 'replacing') {
      return { label: 'Installing', variant: 'info' as const, description: 'Replacing application...' }
    }
    if (installProgress.status === 'cleaning') {
      return { label: 'Finalizing', variant: 'info' as const, description: 'Finalizing update...' }
    }
    if (installProgress.status === 'restarting') {
      return { label: 'Restarting', variant: 'warning' as const, description: 'Restarting into the new version...' }
    }
    if (installProgress.status === 'completed') {
      return { label: 'Installed', variant: 'success' as const, description: 'Update installed.' }
    }
    if (installProgress.status === 'failed') {
      return { label: 'Failed', variant: 'error' as const, description: installProgress.message || 'Install failed.' }
    }
    return null
  }, [installProgress])

  const installPercent = useMemo(() => {
    if (!installProgress) return 0
    if (installProgress.percent > 0) {
      return Math.min(100, installProgress.percent)
    }
    return installProgress.status === 'completed' ? 100 : 0
  }, [installProgress])

  useEffect(() => {
    let cancelled = false
    SystemService.GetUpdateDownloadProgress()
      .then((progress) => {
        if (cancelled) return
        if (progress.status && progress.status !== 'idle') {
          setDownloadProgress(progress)
        }
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    SystemService.GetUpdateInstallProgress()
      .then((progress) => {
        if (cancelled) return
        if (progress.status && progress.status !== 'idle') {
          setInstallProgress(progress)
        }
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!downloadProgress) return
    if (downloadProgress.status !== 'resolving' && downloadProgress.status !== 'downloading') return
    let cancelled = false
    const poll = async () => {
      try {
        const progress = await SystemService.GetUpdateDownloadProgress()
        if (!cancelled) {
          setDownloadProgress(progress)
        }
      }
      catch {
      }
    }
    poll()
    const intervalId = window.setInterval(poll, 1000)
    return () => {
      cancelled = true
      window.clearInterval(intervalId)
    }
  }, [downloadProgress?.status])

  useEffect(() => {
    if (!installProgress) return
    if (!isInstallActive) return
    let cancelled = false
    const poll = async () => {
      try {
        const progress = await SystemService.GetUpdateInstallProgress()
        if (!cancelled) {
          setInstallProgress(progress)
        }
      }
      catch {
      }
    }
    poll()
    const intervalId = window.setInterval(poll, 1000)
    return () => {
      cancelled = true
      window.clearInterval(intervalId)
    }
  }, [installProgress?.status, isInstallActive])

  useEffect(() => {
    if (installProgress?.status !== 'restarting') return
    const key = installProgress.filePath ?? 'restarting'
    if (restartNotifiedRef.current === key) return
    restartNotifiedRef.current = key
    toastManager.add({
      type: 'info',
      title: 'Restarting',
      description: 'Installing update and restarting the app...',
    })
  }, [installProgress?.filePath, installProgress?.status])

  const diagnosticsPreview = useMemo(() => {
    if (!diagnosticsData) return null
    const limit = 1400
    if (diagnosticsData.length <= limit) {
      return diagnosticsData
    }
    return `${diagnosticsData.slice(0, limit)}\n\n... (truncated)`
  }, [diagnosticsData])

  const diagnosticsModeLabel = diagnosticsOptions.mode === 'deep' ? 'Deep' : 'Safe'
  const diagnosticsLogLabel = diagnosticsOptions.logLevel ?? 'info'
  const diagnosticsSizeLabel = diagnosticsMeta ? `${Math.round(diagnosticsMeta.size / 1024)} KB` : '—'
  const diagnosticsExportedLabel = diagnosticsMeta?.generatedAt
    ? formatRelativeTime(diagnosticsMeta.generatedAt)
    : 'Not exported'

  return (
    <div className="space-y-3 p-3">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Updates</CardTitle>
          <CardDescription className="text-xs">
            {updateDescription}
          </CardDescription>
          <CardAction>
            <Button
              size="sm"
              variant="secondary"
              onClick={handleCheckNow}
              disabled={isChecking}
            >
              {isChecking ? (
                <Spinner className="size-3.5" />
              ) : (
                <RefreshCwIcon className="size-3.5" />
              )}
              {isChecking ? 'Checking...' : 'Check now'}
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="rounded-lg border border-dashed bg-muted/20 p-3">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">Manual check</span>
                  <Badge variant={manualStatus.variant} size="sm" className="gap-1">
                    {manualStatus.label}
                  </Badge>
                </div>
                <p className={manualCheckError ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}>
                  {manualStatusDescription}
                </p>
                {manualCheckedAt && (
                  <p className="text-xs text-muted-foreground">
                    Last checked {formatRelativeTime(manualCheckedAt)}.
                  </p>
                )}
                {manualUpdateAvailable && manualLatest?.publishedAt && (
                  <p className="text-xs text-muted-foreground">
                    Released {formatRelativeTime(manualLatest.publishedAt)}.
                  </p>
                )}
              </div>
              {manualUpdateAvailable && manualLatest?.url && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => void handleDownloadUpdate()}
                  disabled={isDownloadStarting || isDownloadActive || isInstallStarting || isInstallActive}
                >
                  {isDownloadStarting || isDownloadActive ? (
                    <Spinner className="size-4" />
                  ) : (
                    <DownloadIcon className="size-4" />
                  )}
                    {isDownloadStarting || isDownloadActive ? 'Downloading...' : 'Download'}
                  </Button>
                )}
            </div>
          </div>
          {downloadStatus && (
            <div className="rounded-lg border border-dashed bg-muted/20 p-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">Download</span>
                    <Badge variant={downloadStatus.variant} size="sm" className="gap-1">
                      {downloadStatus.label}
                    </Badge>
                  </div>
                  <p className={downloadProgress?.status === 'failed' ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}>
                    {downloadStatus.description}
                  </p>
                  {downloadProgress?.filePath && downloadProgress.status === 'completed' && (
                    <p className="text-xs text-muted-foreground">
                      {downloadProgress.filePath}
                    </p>
                  )}
                </div>
              </div>
              <div className="mt-3">
                <Progress value={downloadPercent} />
              </div>
            </div>
          )}
          {installStatus && (
            <div className="rounded-lg border border-dashed bg-muted/20 p-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">Install</span>
                    <Badge variant={installStatus.variant} size="sm" className="gap-1">
                      {installStatus.label}
                    </Badge>
                  </div>
                  <p className={installProgress?.status === 'failed' ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}>
                    {installStatus.description}
                  </p>
                  {installProgress?.appPath && (
                    <p className="text-xs text-muted-foreground">
                      {installProgress.appPath}
                    </p>
                  )}
                </div>
              </div>
              <div className="mt-3">
                <Progress value={installPercent} />
              </div>
            </div>
          )}
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="updates-prerelease" className="text-sm font-medium">
                Include pre-release versions
              </Label>
              <p className="text-xs text-muted-foreground">
                Enable this to receive early access builds with new features.
              </p>
            </div>
            <Switch
              checked={updateSettings.includePrerelease}
              disabled={updateLoading || !!updateError}
              id="updates-prerelease"
              onCheckedChange={handlePrereleaseToggle}
            />
          </div>
          {updateError ? (
            <p className="text-xs text-destructive">
              Unable to load update preferences.
            </p>
          ) : null}
          {import.meta.env.DEV && (
            <p className="text-xs text-muted-foreground">
              Update checks are disabled in development builds.
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Menu Bar & Tray</CardTitle>
          <CardDescription className="text-xs">
            Enable the menu bar tray for quick access to mcpv.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="tray-enabled" className="text-sm font-medium">
                Enable tray icon
              </Label>
              <p className="text-xs text-muted-foreground">
                Show a tray icon and keep the app running in the background.
              </p>
            </div>
            <Switch
              checked={traySettings.enabled}
              disabled={trayLoading}
              id="tray-enabled"
              onCheckedChange={handleTrayToggle}
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="tray-hide-dock" className="text-sm font-medium">
                Hide Dock icon when tray is enabled
              </Label>
              <p className="text-xs text-muted-foreground">
                Keep mcpv out of the Dock while the tray icon is active.
              </p>
            </div>
            <Switch
              checked={traySettings.hideDock}
              disabled={!traySettings.enabled || trayLoading || !isMac}
              id="tray-hide-dock"
              onCheckedChange={handleHideDockToggle}
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="tray-start-hidden" className="text-sm font-medium">
                Start hidden
              </Label>
              <p className="text-xs text-muted-foreground">
                Launch directly to the tray without showing the main window.
              </p>
            </div>
            <Switch
              checked={traySettings.startHidden}
              disabled={!traySettings.enabled || trayLoading}
              id="tray-start-hidden"
              onCheckedChange={handleStartHiddenToggle}
            />
          </div>

          {!isMac && (
            <p className="text-xs text-muted-foreground">
              Dock visibility controls are only available on macOS.
            </p>
          )}
          {trayError ? (
            <p className="text-xs text-destructive">
              Unable to load tray settings.
            </p>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Telemetry</CardTitle>
          <CardDescription className="text-xs">
            Help improve mcpv by sending anonymous usage data.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="analytics-toggle" className="text-sm font-medium">
                Send anonymous usage data
              </Label>
              <p className="text-xs text-muted-foreground">
                We collect anonymous usage statistics to improve the app. No personal data is collected.
              </p>
            </div>
            <Switch
              checked={analyticsEnabled}
              id="analytics-toggle"
              onCheckedChange={toggleAnalytics}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            Debug & Diagnostics
          </CardTitle>
          <CardDescription className="text-xs">
            Collect snapshots and export diagnostics bundles to troubleshoot issues.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 xl:grid-cols-[1.2fr_0.8fr]">
            <div className="space-y-4">
              <div className="rounded-xl border bg-muted/10 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="space-y-1">
                    <p className="text-sm font-semibold text-foreground">Debug snapshot</p>
                    <p className="text-xs text-muted-foreground">
                      Capture the current state and copy it directly to the clipboard.
                    </p>
                  </div>
                  <Button
                    size="sm"
                    onClick={() => void handleExportDebug()}
                    disabled={isSnapshotExporting}
                  >
                    {isSnapshotExporting ? (
                      <Spinner className="size-3.5" />
                    ) : (
                      <ClipboardCopyIcon className="size-3.5" />
                    )}
                    {isSnapshotExporting ? 'Copying...' : 'Copy snapshot'}
                  </Button>
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <Badge variant="secondary" size="sm">
                    Clipboard first
                  </Badge>
                  <span>
                    {snapshotCopiedAt
                      ? `Last copied ${formatRelativeTime(snapshotCopiedAt)}.`
                      : 'No snapshots copied yet.'}
                  </span>
                </div>
              </div>

              <div className="rounded-xl border bg-muted/10 p-4 space-y-3">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="space-y-1">
                    <p className="text-sm font-semibold text-foreground">Diagnostics bundle</p>
                    <p className="text-xs text-muted-foreground">
                      Export logs, metrics, and event history into a shareable report.
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      size="sm"
                      onClick={() => void handleExportDiagnostics()}
                      disabled={isDiagnosticsExporting}
                    >
                      {isDiagnosticsExporting ? (
                        <Spinner className="size-3.5" />
                      ) : (
                        <FileDownIcon className="size-3.5" />
                      )}
                      {isDiagnosticsExporting ? 'Exporting...' : 'Export bundle'}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => handleDiagnosticsOpenChange(true)}
                    >
                      Customize
                    </Button>
                  </div>
                </div>
                <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                  <Badge variant="outline" size="sm">
                    Mode: {diagnosticsModeLabel}
                  </Badge>
                  <Badge variant="outline" size="sm">
                    Logs: {diagnosticsLogLabel}
                  </Badge>
                  <Badge variant="outline" size="sm">
                    Snapshot {diagnosticsOptions.includeSnapshot ? 'On' : 'Off'}
                  </Badge>
                </div>
                {diagnosticsOptions.mode === 'deep' ? (
                  <Alert variant="warning">
                    <AlertTitle>Deep export may include secrets</AlertTitle>
                    <AlertDescription>
                      Use deep mode only when needed. Review the bundle before sharing.
                    </AlertDescription>
                  </Alert>
                ) : null}
              </div>
            </div>

            <div className="flex h-full flex-col gap-3 rounded-xl border bg-background/40 p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                  <p className="text-sm font-semibold">Latest bundle</p>
                  <p className="text-xs text-muted-foreground">
                    Preview and share the most recent diagnostics export.
                  </p>
                </div>
                <Badge variant={diagnosticsMeta ? 'secondary' : 'outline'} size="sm">
                  {diagnosticsExportedLabel}
                </Badge>
              </div>
              <div className="grid gap-1 text-xs text-muted-foreground">
                <div className="flex items-center justify-between">
                  <span>Size</span>
                  <span>{diagnosticsSizeLabel}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Redaction</span>
                  <span>{diagnosticsModeLabel}</span>
                </div>
              </div>
              <div className="flex-1 overflow-hidden">
                {diagnosticsPreview ? (
                  <pre className="h-full rounded-lg bg-muted/40 p-3 text-[11px] leading-relaxed overflow-auto">
                    <code>{diagnosticsPreview}</code>
                  </pre>
                ) : (
                  <div className="flex h-full min-h-32 items-center justify-center rounded-lg border border-dashed border-muted-foreground/30 text-xs text-muted-foreground">
                    Run an export to generate a preview.
                  </div>
                )}
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleCopyDiagnostics}
                  disabled={!diagnosticsData}
                >
                  Copy
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleDownloadDiagnostics}
                  disabled={!diagnosticsData}
                >
                  Download
                </Button>
                <Button size="sm" onClick={() => handleDiagnosticsOpenChange(true)}>
                  Open exporter
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

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
              Configure and export a diagnostics bundle for troubleshooting.
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

                {diagnosticsOptions.mode === 'deep' ? (
                  <Alert variant="warning">
                    <AlertTitle>Deep export may include secrets</AlertTitle>
                    <AlertDescription>
                      Use deep mode only when needed. Review the bundle before sharing.
                    </AlertDescription>
                  </Alert>
                ) : null}
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
    </div>
  )
}
