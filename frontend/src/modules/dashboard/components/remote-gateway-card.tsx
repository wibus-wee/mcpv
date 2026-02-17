// Input: UI settings hook, gateway config helpers, router navigation, UI components
// Output: RemoteGatewayCard component for remote access configuration
// Position: Dashboard component for remote gateway guidance

import { useRouter } from '@tanstack/react-router'
import { GlobeIcon, ShieldCheckIcon, ShieldOffIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useMemo } from 'react'

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
import { useUISettings } from '@/hooks/use-ui-settings'
import { Spring } from '@/lib/spring'
import {
  buildEndpointPreview,
  GATEWAY_SECTION_KEY,
  toGatewayFormState,
} from '@/modules/settings/lib/gateway-config'

export function RemoteGatewayCard() {
  const router = useRouter()
  const { sections, isLoading, error } = useUISettings()

  const gatewaySettings = useMemo(
    () => toGatewayFormState(sections[GATEWAY_SECTION_KEY]),
    [sections],
  )

  const endpoint = useMemo(
    () => buildEndpointPreview(gatewaySettings.httpAddr, gatewaySettings.httpPath),
    [gatewaySettings.httpAddr, gatewaySettings.httpPath],
  )

  const accessBadge = gatewaySettings.accessMode === 'network'
    ? { label: 'Network', variant: 'info' as const }
    : { label: 'Local only', variant: 'secondary' as const }

  const enabledBadge = gatewaySettings.enabled
    ? { label: 'Enabled', variant: 'success' as const }
    : { label: 'Disabled', variant: 'secondary' as const }

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
                <GlobeIcon className="size-5 text-muted-foreground" />
              </div>
              <div className="space-y-1">
                <CardTitle className="text-base">Remote Gateway</CardTitle>
                <CardDescription>
                  Let remote agents reach this core via the HTTP gateway.
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
                      <Badge variant={enabledBadge.variant}>{enabledBadge.label}</Badge>
                      <Badge variant={accessBadge.variant}>{accessBadge.label}</Badge>
                    </>
                  )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="relative space-y-4">
          <p className="text-sm text-muted-foreground">
            Expose a secure endpoint so teammates can connect from another machine.
          </p>
          <div className="grid gap-2 rounded-lg border border-border/60 bg-muted/30 p-3 text-xs">
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Endpoint</span>
              <span className="font-mono text-foreground">{endpoint}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Token</span>
              <span className="font-mono text-foreground">
                {gatewaySettings.httpToken ? 'Configured' : 'Not set'}
              </span>
            </div>
          </div>
          {error && (
            <p className="text-xs text-destructive">
              Unable to load gateway settings. Check the app logs.
            </p>
          )}
        </CardContent>
        <CardFooter className="relative flex flex-wrap items-center justify-between gap-2 border-t">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            {gatewaySettings.accessMode === 'network'
              ? (
                  <>
                    <ShieldCheckIcon className="size-3 text-emerald-500/80" />
                    Token required for remote access.
                  </>
                )
              : (
                  <>
                    <ShieldOffIcon className="size-3 text-muted-foreground" />
                    Local-only mode limits remote clients.
                  </>
                )}
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => router.navigate({ to: '/settings/gateway' })}
          >
            Open Gateway Settings
          </Button>
        </CardFooter>
      </Card>
    </m.div>
  )
}
