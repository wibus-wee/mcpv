// Input: Wails bindings, UI components
// Output: ProfileSubAgentToggle component for per-profile SubAgent enable/disable
// Position: Settings module component for profile-level SubAgent control

import { useCallback, useState } from 'react'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Card } from '@/components/ui/card'
import { Alert } from '@/components/ui/alert'
import { SetProfileSubAgentEnabled } from '@bindings/mcpd/internal/ui/wailsservice'

interface ProfileSubAgentToggleProps {
  profileName: string
  initialEnabled: boolean
  onToggle?: (enabled: boolean) => void
}

export function ProfileSubAgentToggle({
  profileName,
  initialEnabled,
  onToggle
}: ProfileSubAgentToggleProps) {
  const [enabled, setEnabled] = useState(initialEnabled)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleToggle = useCallback(async (checked: boolean) => {
    setIsLoading(true)
    setError(null)

    try {
      await SetProfileSubAgentEnabled({
        profile: profileName,
        enabled: checked,
      })
      setEnabled(checked)
      onToggle?.(checked)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update SubAgent state')
      console.error('Failed to toggle SubAgent:', err)
    } finally {
      setIsLoading(false)
    }
  }, [profileName, onToggle])

  return (
    <Card className="p-4">
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <Label htmlFor={`subagent-${profileName}`} className="text-base font-medium">
              Enable SubAgent for {profileName}
            </Label>
            <p className="text-sm text-muted-foreground">
              Use LLM-based tool filtering for this profile
            </p>
          </div>
          <Switch
            id={`subagent-${profileName}`}
            checked={enabled}
            onCheckedChange={handleToggle}
            disabled={isLoading}
          />
        </div>

        {error && (
          <Alert variant="error">
            <p className="text-sm">{error}</p>
          </Alert>
        )}

        {enabled && (
          <div className="text-xs text-muted-foreground border-l-2 border-primary pl-3">
            <p>When enabled, this profile will expose only:</p>
            <ul className="list-disc list-inside mt-1 space-y-1">
              <li><code className="text-xs">mcpd.automatic_mcp</code> - Get filtered tool list</li>
              <li><code className="text-xs">mcpd.automatic_eval</code> - Execute tools</li>
            </ul>
          </div>
        )}
      </div>
    </Card>
  )
}
