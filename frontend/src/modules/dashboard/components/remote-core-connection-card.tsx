// Input: core connection hook, router navigation, UI components, motion
// Output: RemoteCoreConnectionCard component for core connection summary
// Position: Dashboard component for core connection status and quick access

import { useRouter } from '@tanstack/react-router'
import { CloudIcon, LinkIcon, ShieldCheckIcon, ShieldOffIcon } from 'lucide-react'
import { m } from 'motion/react'

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
import { useCoreConnectionMode } from '@/hooks/use-core-connection'
import { Spring } from '@/lib/spring'

export function RemoteCoreConnectionCard() {
  const router = useRouter()
  const { settings, isRemote, isLoading } = useCoreConnectionMode()

  const authLabel = settings.authMode === 'disabled'
    ? 'No auth'
    : settings.authMode === 'mtls'
      ? 'mTLS'
      : 'Token'
  const tlsLabel = settings.tlsEnabled ? 'TLS on' : 'TLS off'
  const addressLabel = settings.rpcAddress?.trim() || 'Not configured'

  return (
    <m.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={Spring.smooth(0.35, 0.05)}
    >
      <Card className="relative overflow-hidden">
        <CardHeader className="relative">
          <div className="flex items-start justify-between gap-4">
            <div className="flex items-start gap-3">
              <div className="flex size-11 items-center justify-center rounded-xl border bg-background/80 shadow-sm">
                <CloudIcon className="size-5 text-muted-foreground" />
              </div>
              <div className="space-y-1">
                <CardTitle className="text-base">Core Connection</CardTitle>
                <CardDescription>
                  Connect this UI to a remote mcpv Core over RPC.
                </CardDescription>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {isLoading
                ? (
                    <Skeleton className="h-5 w-20" />
                  )
                : (
                    <>
                      <Badge variant={isRemote ? 'info' : 'secondary'}>
                        {isRemote ? 'Remote' : 'Local'}
                      </Badge>
                      <Badge variant={settings.authMode === 'disabled' ? 'secondary' : 'success'}>
                        {authLabel}
                      </Badge>
                    </>
                  )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="relative space-y-4">
          <p className="text-sm text-muted-foreground">
            {isRemote
              ? 'Remote mode is active. Local core controls remain available.'
              : 'Use remote mode to connect to an external core.'}
          </p>
          <div className="grid gap-2 rounded-lg border border-border/60 bg-muted/30 p-3 text-xs">
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">RPC address</span>
              <span className="font-mono text-foreground">{addressLabel}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">TLS</span>
              <span className="font-mono text-foreground">{tlsLabel}</span>
            </div>
          </div>
        </CardContent>
        <CardFooter className="relative flex flex-wrap items-center justify-between gap-2 border-t">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            {settings.authMode === 'disabled'
              ? (
                  <>
                    <ShieldOffIcon className="size-3 text-muted-foreground" />
                    No authentication configured.
                  </>
                )
              : (
                  <>
                    <ShieldCheckIcon className="size-3 text-emerald-500/80" />
                    Authentication enabled.
                  </>
                )}
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => router.navigate({ to: '/settings/core-connection' })}
          >
            <LinkIcon className="size-4" />
            Manage Connection
          </Button>
        </CardFooter>
      </Card>
    </m.div>
  )
}
