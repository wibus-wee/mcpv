// Input: ProfileDetail, SubAgentService for SubAgent toggle
// Output: ProfileSubAgentSection - SubAgent configuration section
// Position: Section component in profile detail view

import type { ProfileDetail, UpdateProfileSubAgentRequest } from '@bindings/mcpd/internal/ui'
import { SubAgentService } from '@bindings/mcpd/internal/ui'
import { useState } from 'react'

import { Switch } from '@/components/ui/switch'
import { toastManager } from '@/components/ui/toast'

interface ProfileSubAgentSectionProps {
  profile: ProfileDetail
  canEdit: boolean
  onToggle: () => void
}

/**
 * SubAgent configuration section with enable/disable toggle.
 */
export function ProfileSubAgentSection({
  profile,
  canEdit,
  onToggle,
}: ProfileSubAgentSectionProps) {
  const [isToggling, setIsToggling] = useState(false)

  const handleToggle = async (checked: boolean) => {
    if (!canEdit || isToggling) return

    setIsToggling(true)
    try {
      const req: UpdateProfileSubAgentRequest = {
        profile: profile.name,
        enabled: checked,
      }
      await SubAgentService.SetProfileSubAgentEnabled(req)
      onToggle()
      toastManager.add({
        type: 'success',
        title: 'SubAgent updated',
        description: `SubAgent ${checked ? 'enabled' : 'disabled'} for ${profile.name}`,
      })
    } catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Update failed',
        description: err instanceof Error ? err.message : 'Unknown error',
      })
    } finally {
      setIsToggling(false)
    }
  }

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-sm font-medium">SubAgent</h2>
        <p className="text-xs text-muted-foreground mt-0.5">
          LLM-based automatic tool filtering and selection
        </p>
      </div>

      <div className="flex items-center justify-between py-2.5 px-3 rounded-md hover:bg-muted/50 transition-colors">
        <div className="flex-1">
          <div className="text-sm font-medium">Enable SubAgent</div>
          <div className="text-xs text-muted-foreground">
            Use LLM to automatically filter and select relevant tools
          </div>
        </div>
        <div className="shrink-0">
          <Switch
            checked={profile.subAgent.enabled}
            onCheckedChange={handleToggle}
            disabled={!canEdit || isToggling}
          />
        </div>
      </div>
    </section>
  )
}
