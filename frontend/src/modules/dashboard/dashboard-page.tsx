// Input: Dashboard components, tabs/alerts/buttons, core/app hooks, analytics
// Output: DashboardPage component - main dashboard view with insights
// Position: Main dashboard page in dashboard module

import { DebugService } from '@bindings/mcpv/internal/ui/services'
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

import { ConnectIdeSheet } from '@/components/common/connect-ide-sheet'
import { UniversalEmptyState } from '@/components/common/universal-empty-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
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
import { Separator } from '@/components/ui/separator'
import { toastManager } from '@/components/ui/toast'
import { useCoreActions, useCoreState } from '@/hooks/use-core-state'
import { AnalyticsEvents, track } from '@/lib/analytics'
import { Spring } from '@/lib/spring'

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

        <Button
          variant="outline"
          size="sm"
          onClick={handleExportDebug}
          disabled={isExporting}
        >
          <FileDownIcon className="size-4" />
          {isExporting ? 'Copying...' : 'Copy Debug'}
        </Button>

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
