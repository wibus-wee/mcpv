// Input: Dashboard components, cards/buttons, core/app hooks, analytics, toast
// Output: DashboardPage component - main dashboard view with insights
// Position: Main dashboard page in dashboard module

import {
  AlertCircleIcon,
  Loader2Icon,
  PlayIcon,
  RefreshCwIcon,
  ServerIcon,
  ShieldCheckIcon,
  SquareIcon,
  ZapIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useCallback, useEffect, useRef } from 'react'

import { ConnectIdeSheet } from '@/components/common/connect-agent-sheet'
import { UniversalEmptyState } from '@/components/common/universal-empty-state'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { toastManager } from '@/components/ui/toast'
import { useCoreActions, useCoreState } from '@/hooks/use-core-state'
import { AnalyticsEvents, track } from '@/lib/analytics'
import { Spring } from '@/lib/spring'

import { ActiveClientsPanel } from './components/active-clients-panel'
import { ActivityInsights } from './components/activity-insights'
import { BootstrapProgressPanel } from './components/bootstrap-progress'
import { LocalCoreDaemonCard } from './components/local-core-daemon-card'
import { RemoteGatewayCard } from './components/remote-gateway-card'
import { ServerHealthOverview } from './components/server-health-overview'
import { StatusCards } from './components/status-cards'
import { useAppInfo, useBootstrapProgress } from './hooks'

function DashboardHeader() {
  const { appInfo } = useAppInfo()
  const { coreStatus } = useCoreState()
  const { refreshCoreState, restartCore, startCore, stopCore } = useCoreActions()

  const appLabel = appInfo?.name
    ? `${appInfo.name} · ${appInfo.version === 'dev' ? 'dev' : `${appInfo.version}`} (${appInfo.build})`
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
          variant="ghost"
          size="icon-sm"
          onClick={() => void handleRefreshCoreState()}
        >
          <RefreshCwIcon className="size-4" />
        </Button>

        <ConnectIdeSheet />
      </div>
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

function StoppedCorePanel({ onStartCore }: { onStartCore: () => void }) {
  return (
    <m.div
      initial={{ opacity: 0, y: 12, filter: 'blur(6px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={Spring.smooth(0.4)}
      className="grid gap-6 md:grid-cols-2 lg:grid-cols-3"
    >
      <Card className="relative overflow-hidden">
        <CardHeader className="relative">
          <div className="flex items-start gap-4">
            <div className="flex size-11 items-center justify-center rounded-xl border bg-background/80 shadow-sm">
              <ServerIcon className="size-5 text-muted-foreground" />
            </div>
            <div className="space-y-1">
              <CardTitle className="text-base">Core is not running</CardTitle>
              <CardDescription>
                Start the mcpv core to see your dashboard and manage MCP servers.
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="relative space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <Button onClick={onStartCore} size="sm">
              <PlayIcon className="size-4" />
              Start Core
            </Button>
          </div>
          <div className="grid gap-2 text-xs text-muted-foreground">
            <div className="flex items-start gap-2">
              <ZapIcon className="mt-0.5 size-3 text-sky-500/80" />
              <span>Runs in the foreground with live logs and activity.</span>
            </div>
            <div className="flex items-start gap-2">
              <ShieldCheckIcon className="mt-0.5 size-3 text-emerald-500/80" />
              <span>Need always-on uptime? Enable the local daemon on the right.</span>
            </div>
          </div>
        </CardContent>
      </Card>
      <LocalCoreDaemonCard />
      <RemoteGatewayCard />
    </m.div>
  )
}

export function DashboardPage() {
  const { coreStatus, data: coreState } = useCoreState()
  const { startCore, restartCore } = useCoreActions()
  const errorToastRef = useRef<string | null>(null)

  const handleStartCore = useCallback(async () => {
    try {
      await startCore()
      track(AnalyticsEvents.CORE_START, { result: 'success' })
    }
    catch {
      track(AnalyticsEvents.CORE_START, { result: 'error' })
    }
  }, [startCore])

  const handleRestartCore = useCallback(async () => {
    try {
      await restartCore()
      track(AnalyticsEvents.CORE_RESTART, { result: 'success' })
    }
    catch {
      track(AnalyticsEvents.CORE_RESTART, { result: 'error' })
    }
  }, [restartCore])

  useEffect(() => {
    if (coreStatus !== 'error') {
      errorToastRef.current = null
      return
    }
    const message = coreState?.error || 'The mcpv core encountered an error. Check the logs for details.'
    if (errorToastRef.current === message) {
      return
    }
    errorToastRef.current = message
    toastManager.add({
      type: 'error',
      title: 'Core error',
      description: message,
    })
  }, [coreStatus, coreState?.error])

  if (coreStatus === 'stopped') {
    return (
      <div className="flex flex-1 flex-col p-6 overflow-auto">
        <DashboardHeader />
        <Separator className="my-6" />
        <StoppedCorePanel onStartCore={() => void handleStartCore()} />
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
        <UniversalEmptyState
          icon={AlertCircleIcon}
          title="Core error"
          description={coreState?.error || 'The mcpv core encountered an error. Check the logs for details.'}
          action={{
            label: 'Retry',
            onClick: () => void handleRestartCore(),
          }}
        />
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
