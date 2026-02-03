// Input: TanStack Router, analytics toggle, debug utilities
// Output: Advanced settings page with telemetry toggle
// Position: /settings/advanced route

import { createFileRoute } from '@tanstack/react-router'

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
import {
  toggleAnalytics,
  useAnalyticsEnabledValue,
} from '@/lib/analytics'

export const Route = createFileRoute('/settings/advanced')({
  component: AdvancedSettingsPage,
})

function AdvancedSettingsPage() {
  const analyticsEnabled = useAnalyticsEnabledValue()

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
