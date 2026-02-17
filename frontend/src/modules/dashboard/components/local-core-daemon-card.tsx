// Input: DaemonService bindings, daemon status hook, UI components, motion, toast
// Output: LocalCoreDaemonCard component for local daemon status and start flow
// Position: Dashboard component for local core daemon onboarding

import { DaemonService } from '@bindings/mcpv/internal/ui/services'
import type { DaemonStatus } from '@bindings/mcpv/internal/ui/services/models'
import {
  Loader2Icon,
  RefreshCwIcon,
  ServerCogIcon,
  ShieldCheckIcon,
  WrenchIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useCallback, useMemo, useState } from 'react'

import {
  AlertDialog,
  AlertDialogClose,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { toastManager } from '@/components/ui/toast'
import { useDaemonStatus } from '@/hooks/use-daemon-status'
import { Spring } from '@/lib/spring'

const resolveServiceLabel = (status?: DaemonStatus) => {
  if (status?.serviceName) return status.serviceName
  return 'mcpv'
}

const resolveConfigLabel = (status?: DaemonStatus) => {
  if (status?.configPath) return status.configPath
  return 'Default runtime.yaml'
}

const resolveRPCLabel = (status?: DaemonStatus) => {
  if (status?.rpcAddress) return status.rpcAddress
  return 'Default address'
}

const resolveLogLabel = (status?: DaemonStatus) => {
  if (status?.logPath) return status.logPath
  return 'System default'
}

export function LocalCoreDaemonCard() {
  const {
    daemonStatus,
    error,
    isLoading,
    mutate,
    refreshDaemonStatus,
  } = useDaemonStatus()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [isWorking, setIsWorking] = useState(false)

  const state = useMemo(() => {
    if (isLoading) {
      return {
        label: 'Checking',
        variant: 'secondary' as const,
        description: 'Checking local daemon status...',
        actionable: false,
      }
    }
    if (error) {
      return {
        label: 'Unavailable',
        variant: 'error' as const,
        description: 'Unable to reach the system service manager.',
        actionable: false,
      }
    }
    if (daemonStatus?.running) {
      return {
        label: 'Running',
        variant: 'success' as const,
        description: 'Local core service is active.',
        actionable: false,
      }
    }
    if (daemonStatus?.installed) {
      return {
        label: 'Stopped',
        variant: 'warning' as const,
        description: 'Service is installed but not running.',
        actionable: true,
      }
    }
    return {
      label: 'Not installed',
      variant: 'secondary' as const,
      description: 'Install a user-level service to keep core running.',
      actionable: true,
    }
  }, [daemonStatus, error, isLoading])

  const actionLabel = daemonStatus?.running
    ? 'Running'
    : daemonStatus?.installed
      ? 'Start service'
      : 'Install & Start'

  const handleDialogChange = useCallback((nextOpen: boolean) => {
    if (!nextOpen && isWorking) {
      return
    }
    setDialogOpen(nextOpen)
  }, [isWorking])

  const handleEnsureRunning = useCallback(async () => {
    if (isWorking) return
    setIsWorking(true)
    try {
      const nextStatus = await DaemonService.EnsureRunning({ allowStart: true })
      await mutate(nextStatus, { revalidate: false })
      toastManager.add({
        type: 'success',
        title: 'Local daemon started',
        description: 'mcpv is now running as a system service.',
      })
      setDialogOpen(false)
    }
    catch (err) {
      const message = err instanceof Error ? err.message : 'Unable to start the local daemon'
      toastManager.add({
        type: 'error',
        title: 'Daemon start failed',
        description: message,
      })
    }
    finally {
      setIsWorking(false)
    }
  }, [isWorking, mutate])

  const handleRefresh = useCallback(() => {
    if (isLoading) return
    refreshDaemonStatus()
  }, [isLoading, refreshDaemonStatus])

  return (
    <m.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.35)}
    >
      <Card className="relative overflow-hidden">
        <CardHeader className="relative">
          <div className="flex items-start justify-between gap-4">
            <div className="flex items-start gap-3">
              <div className="flex size-11 items-center justify-center rounded-xl border bg-background/80 shadow-sm">
                <ServerCogIcon className="size-5 text-muted-foreground" />
              </div>
              <div className="space-y-1">
                <CardTitle className="text-base">Local Core Daemon</CardTitle>
                <CardDescription>
                  Run mcpv continuously with a user-level system service.
                </CardDescription>
              </div>
            </div>
            {isLoading
              ? (
                  <Skeleton className="h-5 w-20" />
                )
              : (
                  <Badge variant={state.variant}>{state.label}</Badge>
                )}
          </div>
        </CardHeader>
        <CardContent className="relative space-y-4">
          <p className="text-sm text-muted-foreground">
            {state.description}
          </p>
          <div className="grid gap-2 rounded-lg border border-border/60 bg-muted/30 p-3 text-xs">
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Service</span>
              <span className="font-mono text-foreground">{resolveServiceLabel(daemonStatus)}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Config</span>
              <span className="font-mono text-foreground">{resolveConfigLabel(daemonStatus)}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">RPC</span>
              <span className="font-mono text-foreground">{resolveRPCLabel(daemonStatus)}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Logs</span>
              <span className="font-mono text-foreground">{resolveLogLabel(daemonStatus)}</span>
            </div>
          </div>
        </CardContent>
        <CardFooter className="relative flex flex-wrap items-center justify-between gap-2 border-t">
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={handleRefresh}
            disabled={isLoading || isWorking}
          >
            <RefreshCwIcon className="size-4" />
          </Button>
          <Button
            size="sm"
            onClick={() => setDialogOpen(true)}
            disabled={!state.actionable || isWorking}
          >
            {isWorking ? (
              <>
                <Loader2Icon className="size-4 animate-spin" />
                Working...
              </>
            ) : (
              actionLabel
            )}
          </Button>
        </CardFooter>
      </Card>

      <AlertDialog open={dialogOpen} onOpenChange={handleDialogChange}>
        <AlertDialogContent className="overflow-hidden">
          <AlertDialogHeader className="relative">
            <AlertDialogTitle>Start local core service?</AlertDialogTitle>
            <AlertDialogDescription>
              This installs a user-level service (systemd/launchd) and starts mcpv in the background.
              You can close the UI and the core will stay running.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="relative space-y-3 px-6">
            <div className="grid gap-2 rounded-lg border border-border/60 bg-muted/30 p-3 text-xs">
              <div className="flex items-center justify-between gap-3">
                <span className="text-muted-foreground">Service</span>
                <span className="font-mono text-foreground">{resolveServiceLabel(daemonStatus)}</span>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span className="text-muted-foreground">Config</span>
                <span className="font-mono text-foreground">{resolveConfigLabel(daemonStatus)}</span>
              </div>
            </div>
            <div className="grid gap-2 text-sm text-muted-foreground">
              <div className="flex items-start gap-2">
                <ShieldCheckIcon className="mt-0.5 size-4 text-emerald-500/80" />
                <span>Runs with your user session and restarts on failure.</span>
              </div>
              <div className="flex items-start gap-2">
                <WrenchIcon className="mt-0.5 size-4 text-sky-500/80" />
                <span>Uses the same runtime configuration as this app.</span>
              </div>
            </div>
          </div>
          <AlertDialogFooter variant="bare" className="relative">
            <AlertDialogClose
              render={(
                <Button variant="ghost" size="sm" disabled={isWorking}>
                  Not now
                </Button>
              )}
            />
            <Button onClick={handleEnsureRunning} size="sm" disabled={isWorking}>
              {isWorking ? (
                <>
                  <Loader2Icon className="size-4 animate-spin" />
                  Starting...
                </>
              ) : (
                actionLabel
              )}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </m.div>
  )
}
