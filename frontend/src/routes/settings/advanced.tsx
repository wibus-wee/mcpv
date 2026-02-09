// Input: TanStack Router, analytics toggle, debug utilities
// Output: Advanced settings page with telemetry toggle
// Position: /settings/advanced route

import { SystemService } from '@bindings/mcpv/internal/ui/services'
import type { UpdateCheckOptions } from '@bindings/mcpv/internal/ui/types'
import { createFileRoute } from '@tanstack/react-router'
import { useCallback, useMemo } from 'react'
import useSWR from 'swr'

import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { toastManager } from '@/components/ui/toast'
import {
  toggleAnalytics,
  useAnalyticsEnabledValue,
} from '@/lib/analytics'

export const Route = createFileRoute('/settings/advanced')({
  component: AdvancedSettingsPage,
})

function AdvancedSettingsPage() {
  const analyticsEnabled = useAnalyticsEnabledValue()
  const {
    data: updateOptions,
    error: updateError,
    isLoading: updateLoading,
    mutate: mutateUpdateOptions,
  } = useSWR<UpdateCheckOptions>(
    'update-check-options',
    () => SystemService.GetUpdateCheckOptions(),
    { revalidateOnFocus: false },
  )

  const effectiveUpdateOptions = useMemo(
    () => updateOptions ?? { intervalHours: 24, includePrerelease: false },
    [updateOptions],
  )

  const handlePrereleaseToggle = useCallback(async (checked: boolean) => {
    try {
      const next = await SystemService.SetUpdateCheckOptions({
        ...effectiveUpdateOptions,
        includePrerelease: checked,
      })
      mutateUpdateOptions(next, { revalidate: false })
      toastManager.add({
        type: 'success',
        title: 'Update preference saved',
        description: checked
          ? 'Pre-release updates are now enabled.'
          : 'Only stable releases will be shown.',
      })
    }
    catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Update preference failed',
        description: err instanceof Error ? err.message : 'Unable to update settings',
      })
    }
  }, [effectiveUpdateOptions, mutateUpdateOptions])

  const updateDescription = useMemo(() => {
    const intervalHours = effectiveUpdateOptions.intervalHours || 24
    return `Checks for new releases every ${intervalHours} hours.`
  }, [effectiveUpdateOptions.intervalHours])

  return (
    <div className="space-y-3 p-3">
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
          <CardTitle className="text-sm">Updates</CardTitle>
          <CardDescription className="text-xs">
            {updateDescription}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
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
              checked={effectiveUpdateOptions.includePrerelease}
              disabled={updateLoading || !!updateError}
              id="updates-prerelease"
              onCheckedChange={handlePrereleaseToggle}
            />
          </div>
          {updateError && (
            <p className="text-xs text-destructive">
              Unable to load update preferences.
            </p>
          )}
          {import.meta.env.DEV && (
            <p className="text-xs text-muted-foreground">
              Update checks are disabled in development builds.
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            Debug & Diagnostics
            <Badge variant="secondary" size="sm">
              Coming Soon
            </Badge>
          </CardTitle>
          <CardDescription className="text-xs">
            Debug logs and diagnostic tools will be available here.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Advanced debugging features are currently under development.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
