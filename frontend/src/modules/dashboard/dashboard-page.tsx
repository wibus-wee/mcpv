// Input: Dashboard components, tabs/alerts/buttons, core/app hooks
// Output: DashboardPage component - main dashboard view
// Position: Main dashboard page in dashboard module

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
import { useState } from 'react'

import { DebugService } from '@bindings/mcpd/internal/ui'

import { UniversalEmptyState } from '@/components/common/universal-empty-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toastManager } from '@/components/ui/toast'
import { useCoreActions, useCoreState } from '@/hooks/use-core-state'

import { ConnectIdeSheet } from '@/components/common/connect-ide-sheet'
import {
  BootstrapProgressPanel,
  LogsPanel,
  ResourcesList,
  StatusCards,
  ToolsTable,
} from './components'
import { useAppInfo, useBootstrapProgress } from './hooks'
import { Spring } from '@/lib/spring'

function DashboardHeader() {
  const { appInfo } = useAppInfo()
  const { coreStatus } = useCoreState()
  const {
    refreshCoreState,
    restartCore,
    startCore,
    stopCore,
  } = useCoreActions()
  const [isExporting, setIsExporting] = useState(false)
  const appLabel = appInfo?.name
    ? `${appInfo.name} · ${appInfo.version === "dev" ? "dev" : `v${appInfo.version}`} (${appInfo.build})`
    : 'mcpd'

  const handleExportDebug = async () => {
    if (isExporting) {
      return
    }
    setIsExporting(true)
    try {
      const result = await DebugService.ExportDebugSnapshot()
      toastManager.add({
        type: 'success',
        title: 'Debug snapshot exported',
        description: result.path,
      })
    } catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Export failed',
        description: err instanceof Error ? err.message : 'Export failed',
      })
    } finally {
      setIsExporting(false)
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
        {coreStatus === 'stopped' ? (
          <Button onClick={startCore} size="sm">
            <PlayIcon className="size-4" />
            Start Core
          </Button>
        ) : coreStatus === 'starting'
          ? (
            <Button onClick={stopCore} variant="outline" size="sm">
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
                  <Button onClick={stopCore} variant="outline" size="sm">
                    <SquareIcon className="size-4" />
                    Stop
                  </Button>
                  <Button onClick={restartCore} variant="outline" size="sm">
                    <RefreshCwIcon className="size-4" />
                    Restart
                  </Button>
                </>
              )
              : coreStatus === 'error'
                ? (
                  <>
                    <Button onClick={restartCore} size="sm">
                      <RefreshCwIcon className="size-4" />
                      Retry
                    </Button>
                    <Button onClick={stopCore} variant="outline" size="sm">
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
          {isExporting ? 'Exporting...' : 'Export Debug'}
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => refreshCoreState()}
        >
          <RefreshCwIcon className="size-4" />
        </Button>
        <ConnectIdeSheet />
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

      <Tabs defaultValue="tools">
        <TabsList variant="underline">
          <TabsTrigger value="tools">Tools</TabsTrigger>
          <TabsTrigger value="resources">Resources</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
        </TabsList>
        <TabsContent value="tools" className="mt-4">
          <ToolsTable />
        </TabsContent>
        <TabsContent value="resources" className="mt-4">
          <ResourcesList />
        </TabsContent>
        <TabsContent value="logs" className="mt-4">
          <LogsPanel />
        </TabsContent>
      </Tabs>
    </m.div>
  )
}

/**
 * Content shown while core is starting - displays bootstrap progress.
 */
function StartingContent() {
  const { state, total } = useBootstrapProgress()

  // Show bootstrap panel if there are servers to bootstrap
  if (total > 0 || state === 'running') {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-6">
        <BootstrapProgressPanel className="w-full max-w-md" />
      </div>
    )
  }

  // Fallback for initial loading before bootstrap info is available
  return (
    <UniversalEmptyState
      icon={Loader2Icon}
      iconClassName="animate-spin"
      title="Starting Core..."
      description="Please wait while the mcpd core is initializing."
    />
  )
}

export function DashboardPage() {
  const { coreStatus, data: coreState } = useCoreState()
  const { startCore } = useCoreActions()

  if (coreStatus === 'stopped') {
    return (
      <div className="flex flex-1 flex-col p-6 overflow-auto">
        <DashboardHeader />
        <Separator className="my-6" />
        <UniversalEmptyState
          icon={ServerIcon}
          title="Core is not running"
          description="Start the mcpd core to see your dashboard and manage MCP servers."
          action={{
            label: 'Start Core',
            onClick: startCore,
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
              {coreState?.error || 'The mcpd core encountered an error. Check the logs for details.'}
            </AlertDescription>
          </Alert>
        </m.div>
        <div className="mt-6">
          <LogsPanel />
        </div>
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
