// Input: TanStack Router, SubAgent components, profile hooks
// Output: Settings page with SubAgent configuration
// Position: Main settings route component

import type { ConfigModeResponse } from '@bindings/mcpd/internal/ui'
import { WailsService } from '@bindings/mcpd/internal/ui'
import { createFileRoute } from '@tanstack/react-router'
import { Suspense } from 'react'
import useSWR from 'swr'
import { SubAgentConfigForm } from '@/modules/settings/components/subagent-config-form'
import { ProfileSubAgentToggle } from '@/modules/settings/components/profile-subagent-toggle'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Alert } from '@/components/ui/alert'

export const Route = createFileRoute('/settings')({
  component: RouteComponent,
})

function RouteComponent() {
  return (
    <div className="container mx-auto py-8 px-4 max-w-4xl">
      <div className="space-y-6">
        {/* Header */}
        <div className="space-y-2">
          <h1 className="text-3xl font-bold tracking-tight">Settings</h1>
          <p className="text-muted-foreground">
            Configure mcpd runtime and profile settings
          </p>
        </div>

        <Separator />

        {/* SubAgent Section */}
        <div className="space-y-4">
          <div className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight">SubAgent Configuration</h2>
            <p className="text-sm text-muted-foreground">
              Configure LLM-based automatic tool filtering and selection
            </p>
          </div>

          <Suspense fallback={<SettingsSkeleton />}>
            <SettingsContent />
          </Suspense>
        </div>
      </div>
    </div>
  )
}

function SettingsContent() {
  return (
    <div className="space-y-6">
      <ConfigPathCard />

      {/* Runtime Configuration */}
      <div className="space-y-4">
        <h3 className="text-lg font-medium">Runtime Configuration</h3>
        <SubAgentConfigForm />
      </div>

      {/* Profile Configuration */}
      <div className="space-y-4">
        <h3 className="text-lg font-medium">Profile Configuration</h3>
        <p className="text-sm text-muted-foreground">
          Enable or disable SubAgent for each profile
        </p>
        <ProfileSubAgentToggles />
      </div>
    </div>
  )
}

function ProfileSubAgentToggles() {
  // TODO: Fetch profiles from backend
  // For now, show a placeholder for the default profile
  const profiles = ['default']

  return (
    <div className="space-y-4">
      {profiles.map((profileName) => (
        <ProfileSubAgentToggle
          key={profileName}
          profileName={profileName}
          initialEnabled={false}
        />
      ))}
    </div>
  )
}

function SettingsSkeleton() {
  return (
    <div className="space-y-6">
      <Card className="p-6">
        <div className="space-y-4">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </div>
      </Card>
    </div>
  )
}

function ConfigPathCard() {
  const { data, error } = useSWR<ConfigModeResponse>(
    'settings-config-mode',
    () => WailsService.GetConfigMode(),
  )

  if (error) {
    return (
      <Alert variant="error">
        <p className="text-sm">Failed to load configuration path.</p>
      </Alert>
    )
  }

  if (!data) {
    return (
      <Card className="p-6">
        <div className="space-y-4">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
        </div>
      </Card>
    )
  }

  const modeLabel = data.mode || 'unknown'
  const pathLabel = data.path || 'Unavailable'
  const writableLabel = data.isWritable ? 'writable' : 'read-only'
  const writableVariant = data.isWritable ? 'success' : 'warning'

  return (
    <Card className="p-6">
      <div className="space-y-4">
        <div className="space-y-2">
          <h3 className="text-lg font-semibold">Config Path</h3>
          <p className="text-sm text-muted-foreground">
            Current configuration path used by mcpd.
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="config-path">Path</Label>
          <Input
            id="config-path"
            value={pathLabel}
            readOnly
            className="font-mono"
          />
        </div>

        <div className="flex flex-wrap gap-2">
          <Badge variant="secondary">{modeLabel}</Badge>
          <Badge variant={writableVariant}>{writableLabel}</Badge>
        </div>
      </div>
    </Card>
  )
}
