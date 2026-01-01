// Input: SubAgent atoms, UI components, Wails bindings
// Output: SubAgentConfigForm component for runtime-level LLM configuration
// Position: Settings module component for SubAgent configuration

import { useAtomValue } from 'jotai'
import { useCallback, useState } from 'react'
import { subAgentConfigAtom, isSubAgentAvailableAtom } from '@/atoms/subagent'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card } from '@/components/ui/card'
import { Alert } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'

export function SubAgentConfigForm() {
  const config = useAtomValue(subAgentConfigAtom)
  const isAvailable = useAtomValue(isSubAgentAvailableAtom)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [formData, setFormData] = useState({
    model: config?.model || '',
    provider: config?.provider || '',
    apiKeyEnvVar: config?.apiKeyEnvVar || '',
    maxToolsPerRequest: config?.maxToolsPerRequest || 20,
    filterPrompt: config?.filterPrompt || '',
  })

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError(null)

    try {
      // Note: Backend doesn't support updating runtime config yet
      // This is a placeholder for future implementation
      console.log('SubAgent config update:', formData)
      alert('Runtime config updates are not yet supported. Please edit runtime.yaml manually.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update config')
    } finally {
      setIsLoading(false)
    }
  }, [formData])

  if (config === null) {
    return (
      <Card className="p-6">
        <div className="space-y-4">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </div>
      </Card>
    )
  }

  if (!isAvailable) {
    return (
      <Alert>
        <p className="text-sm">
          SubAgent infrastructure is not configured. Please add SubAgent configuration to your runtime.yaml file.
        </p>
      </Alert>
    )
  }

  return (
    <Card className="p-6">
      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="space-y-2">
          <h3 className="text-lg font-semibold">Runtime Configuration</h3>
          <p className="text-sm text-muted-foreground">
            LLM provider settings shared across all profiles. Edit runtime.yaml to change these values.
          </p>
        </div>

        {error && (
          <Alert variant="error">
            <p className="text-sm">{error}</p>
          </Alert>
        )}

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="model">Model</Label>
            <Input
              id="model"
              value={formData.model}
              onChange={(e) => setFormData({ ...formData, model: e.target.value })}
              placeholder="gpt-4"
              disabled
            />
            <p className="text-xs text-muted-foreground">
              LLM model to use for tool filtering
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="provider">Provider</Label>
            <Input
              id="provider"
              value={formData.provider}
              onChange={(e) => setFormData({ ...formData, provider: e.target.value })}
              placeholder="openai"
              disabled
            />
            <p className="text-xs text-muted-foreground">
              LLM provider (openai, anthropic, etc.)
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="apiKeyEnvVar">API Key Environment Variable</Label>
            <Input
              id="apiKeyEnvVar"
              value={formData.apiKeyEnvVar}
              onChange={(e) => setFormData({ ...formData, apiKeyEnvVar: e.target.value })}
              placeholder="OPENAI_API_KEY"
              disabled
            />
            <p className="text-xs text-muted-foreground">
              Environment variable name containing the API key
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="maxToolsPerRequest">Max Tools Per Request</Label>
            <Input
              id="maxToolsPerRequest"
              type="number"
              value={formData.maxToolsPerRequest}
              onChange={(e) => setFormData({ ...formData, maxToolsPerRequest: Number.parseInt(e.target.value) })}
              min={1}
              max={100}
              disabled
            />
            <p className="text-xs text-muted-foreground">
              Maximum number of tools to return per request
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="filterPrompt">Custom Filter Prompt (Optional)</Label>
            <Textarea
              id="filterPrompt"
              value={formData.filterPrompt}
              onChange={(e) => setFormData({ ...formData, filterPrompt: e.target.value })}
              placeholder="Custom prompt for tool filtering..."
              rows={4}
              disabled
            />
            <p className="text-xs text-muted-foreground">
              Custom system prompt for LLM-based tool filtering
            </p>
          </div>
        </div>

        <div className="flex justify-end">
          <Button type="submit" disabled={isLoading}>
            {isLoading ? 'Saving...' : 'Save Changes'}
          </Button>
        </div>
      </form>
    </Card>
  )
}
