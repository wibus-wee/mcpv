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
import { useTraySettings } from '@/modules/settings/hooks/use-tray-settings'

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

  const {
    settings: traySettings,
    error: trayError,
    isLoading: trayLoading,
    updateSettings: updateTraySettings,
  } = useTraySettings()

  const isMac = useMemo(() => {
    if (typeof navigator === 'undefined') return false
    return /Mac|iPhone|iPad|iPod/.test(navigator.platform)
  }, [])

  const handlePrereleaseToggle = useCallback(async (checked: boolean) => {
    try {
      const next = await SystemService.SetUpdateCheckOptions({
        ...effectiveUpdateOptions,
        includePrerelease: checked,
      })
      mutateUpdateOptions(next, { revalidate: false })
    }
    catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Update preference failed',
        description: err instanceof Error ? err.message : 'Unable to update settings',
      })
    }
  }, [effectiveUpdateOptions, mutateUpdateOptions])

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
          {trayError && (
            <p className="text-xs text-destructive">
              Unable to load tray settings.
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
