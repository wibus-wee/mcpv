// Input: ProfileDetail type, SubAgentService
// Output: SubAgentSection accordion component
// Position: Profile SubAgent configuration toggle

import type { ProfileDetail } from '@bindings/mcpd/internal/ui'
import { SubAgentService } from '@bindings/mcpd/internal/ui'
import { AlertCircleIcon, CpuIcon } from 'lucide-react'
import { useState } from 'react'

import {
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'

import { reloadConfig } from '../../lib/reload-config'

interface SubAgentSectionProps {
  profile: ProfileDetail
  canEdit: boolean
  onToggle: (enabled: boolean) => void
}

/**
 * Displays the SubAgent configuration section with toggle functionality.
 */
export function SubAgentSection({
  profile,
  canEdit,
  onToggle,
}: SubAgentSectionProps) {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const isEnabled = profile.subAgent?.enabled ?? false

  const handleToggle = async (checked: boolean) => {
    setIsLoading(true)
    setError(null)

    try {
      await SubAgentService.SetProfileSubAgentEnabled({
        profile: profile.name,
        enabled: checked,
      })
      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        setError(reloadResult.message)
        return
      }
      onToggle(checked)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update SubAgent')
      console.error('Failed to toggle SubAgent:', err)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AccordionItem value="subagent" className="border-none">
      <AccordionTrigger className="py-2 hover:no-underline">
        <div className="flex items-center gap-2">
          <CpuIcon className="size-3.5 text-muted-foreground" />
          <span className="text-sm font-medium">SubAgent</span>
          <Badge variant={isEnabled ? 'success' : 'secondary'} size="sm">
            {isEnabled ? 'Enabled' : 'Disabled'}
          </Badge>
        </div>
      </AccordionTrigger>
      <AccordionContent className="pb-3">
        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-2.5 py-2">
            <div className="flex items-center gap-2">
              <Switch
                checked={isEnabled}
                onCheckedChange={handleToggle}
                disabled={!canEdit || isLoading}
              />
              <span className="text-xs text-muted-foreground">
                LLM-based tool filtering
              </span>
            </div>
          </div>

          {error && (
            <Alert variant="error" className="py-2">
              <AlertCircleIcon className="size-3.5" />
              <AlertDescription className="text-xs">{error}</AlertDescription>
            </Alert>
          )}

          {isEnabled && (
            <div className="text-xs text-muted-foreground border-l-2 border-primary pl-2 space-y-1">
              <p>When enabled, exposes only:</p>
              <ul className="list-disc list-inside space-y-0.5 ml-1">
                <li><code className="text-xs">mcpd.automatic_mcp</code></li>
                <li><code className="text-xs">mcpd.automatic_eval</code></li>
              </ul>
            </div>
          )}
        </div>
      </AccordionContent>
    </AccordionItem>
  )
}
