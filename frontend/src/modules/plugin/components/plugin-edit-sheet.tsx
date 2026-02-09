// Input: open state, plugin data (for edit mode), callbacks, analytics
// Output: Sheet component for adding/editing plugin configurations
// Position: Overlay sheet triggered from plugin list

import type { PluginListEntry } from '@bindings/mcpv/internal/ui/types'
import { PlusIcon, SaveIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useCallback, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectItem,
  SelectPopup,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetPanel,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { toastManager } from '@/components/ui/toast'
import { AnalyticsEvents, track } from '@/lib/analytics'

import { PluginCategoryBadge } from './plugin-category-badge'

const PLUGIN_CATEGORIES = [
  { value: 'observability', label: 'Observability', description: 'Logging, metrics, tracing' },
  { value: 'authentication', label: 'Authentication', description: 'Token validation, identity' },
  { value: 'authorization', label: 'Authorization', description: 'Role-based access control' },
  { value: 'rate_limiting', label: 'Rate Limiting', description: 'Request throttling' },
  { value: 'validation', label: 'Validation', description: 'Schema and content validation' },
  { value: 'content', label: 'Content', description: 'Request/response rewriting' },
  { value: 'audit', label: 'Audit', description: 'Event tracking and logging' },
] as const

const PLUGIN_FLOWS = [
  { value: 'request', label: 'Request', description: 'Process incoming requests' },
  { value: 'response', label: 'Response', description: 'Process outgoing responses' },
] as const

interface PluginEditSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  plugin?: PluginListEntry | null
  editTargetName?: string | null
  onSaved?: () => void
}

interface FormData {
  name: string
  category: string
  cmd: string
  cwd: string
  env: string
  timeoutMs: number
  required: boolean
  flows: string[]
  configJson: string
  commitHash: string
}

const INITIAL_FORM_DATA: FormData = {
  name: '',
  category: 'observability',
  cmd: '',
  cwd: '',
  env: '',
  timeoutMs: 5000,
  required: false,
  flows: ['request'],
  configJson: '{}',
  commitHash: '',
}

function FormField({
  label,
  description,
  required,
  children,
}: {
  label: string
  description?: string
  required?: boolean
  children: React.ReactNode
}) {
  return (
    <div className="space-y-2">
      <label className="text-sm font-medium">
        {label}
        {required && <span className="ml-1 text-destructive">*</span>}
      </label>
      {description && (
        <p className="text-xs text-muted-foreground">{description}</p>
      )}
      {children}
    </div>
  )
}

export function PluginEditSheet({
  open,
  onOpenChange,
  plugin,
  editTargetName,
  onSaved,
}: PluginEditSheetProps) {
  const isEdit = Boolean(editTargetName)
  const isEditLoading = isEdit && !plugin
  const isMissingPlugin = isEdit && !plugin
  const isFormDisabled = isEditLoading || isMissingPlugin
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<FormData>({
    defaultValues: INITIAL_FORM_DATA,
  })

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
  } = form

  const category = watch('category')
  const flows = watch('flows')

  useEffect(() => {
    if (!open) return

    if (plugin) {
      // Parse env map to KEY=value format
      const envString = plugin.env
        ? Object.entries(plugin.env)
            .map(([key, value]) => `${key}=${value}`)
            .join('\n')
        : ''

      // Format configJson for display (pretty print if valid JSON)
      let configJsonString = plugin.configJson || '{}'
      try {
        const parsed = JSON.parse(configJsonString)
        configJsonString = JSON.stringify(parsed, null, 2)
      }
      catch {
        // Keep as-is if not valid JSON
      }

      reset({
        name: plugin.name,
        category: plugin.category,
        cmd: plugin.cmd?.join(' ') || '', // Join array to string for display
        cwd: plugin.cwd || '',
        env: envString,
        timeoutMs: plugin.timeoutMs,
        required: plugin.required,
        flows: plugin.flows,
        configJson: configJsonString,
        commitHash: plugin.commitHash ?? '',
      })
    }
    else {
      reset(INITIAL_FORM_DATA)
    }
  }, [plugin, open, reset])

  const toggleFlow = useCallback((flow: string) => {
    const currentFlows = flows ?? []
    const newFlows = currentFlows.includes(flow)
      ? currentFlows.filter(f => f !== flow)
      : [...currentFlows, flow]
    setValue('flows', newFlows.length > 0 ? newFlows : ['request'])
  }, [flows, setValue])

  const onSubmit = useCallback(async (data: FormData) => {
    if (isEdit && !plugin) {
      toastManager.add({
        type: 'error',
        title: 'Plugin not ready',
        description: 'Wait for the configuration to load before saving.',
      })
      return
    }

    setIsSubmitting(true)
    try {
      // Validate configJson
      try {
        JSON.parse(data.configJson)
      }
      catch {
        toastManager.add({
          type: 'error',
          title: 'Invalid JSON',
          description: 'Config JSON must be valid JSON format',
        })
        track(AnalyticsEvents.PLUGIN_SAVE_ATTEMPTED, {
          mode: isEdit ? 'edit' : 'create',
          result: 'invalid_json',
        })
        setIsSubmitting(false)
        return
      }

      // TODO: Implement CreatePlugin/UpdatePlugin in PluginService
      // For now, show not implemented message
      toastManager.add({
        type: 'info',
        title: isEdit ? 'Update not implemented' : 'Create not implemented',
        description: 'Plugin CRUD operations are coming soon. Edit the YAML config directly.',
      })
      track(AnalyticsEvents.PLUGIN_SAVE_ATTEMPTED, {
        mode: isEdit ? 'edit' : 'create',
        result: 'not_implemented',
      })

      onSaved?.()
      onOpenChange(false)
    }
    catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to save plugin'
      track(AnalyticsEvents.PLUGIN_SAVE_ATTEMPTED, {
        mode: isEdit ? 'edit' : 'create',
        result: 'error',
      })
      toastManager.add({
        type: 'error',
        title: 'Failed to save',
        description: message,
      })
    }
    finally {
      setIsSubmitting(false)
    }
  }, [isEdit, onSaved, onOpenChange, plugin])

  const isActionDisabled = isSubmitting || isEditLoading || isMissingPlugin

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right">
        <SheetHeader>
          <SheetTitle>{isEdit ? 'Edit Plugin' : 'Add Plugin'}</SheetTitle>
          <SheetDescription>
            {isEdit
              ? 'Modify the governance plugin configuration'
              : 'Configure a new governance plugin'}
          </SheetDescription>
        </SheetHeader>

        <SheetPanel>
          <fieldset disabled={isFormDisabled} className="min-h-full">
            <m.div
              className="space-y-6"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2 }}
            >
              {/* Basic Info */}
              <FormField label="Plugin Name" required>
                <Input
                  {...register('name')}
                  placeholder="my-plugin"
                  disabled={isEdit}
                />
                {isEdit && (
                  <p className="mt-1 text-xs text-muted-foreground">
                    Plugin name cannot be changed after creation
                  </p>
                )}
              </FormField>

              <FormField
                label="Category"
                required
                description="Governance category determines execution order"
              >
                <Select
                  value={category}
                  onValueChange={v => v && setValue('category', v)}
                >
                  <SelectTrigger>
                    <SelectValue>
                      {value => (
                        <div className="flex items-center gap-2">
                          {value && <PluginCategoryBadge category={String(value)} />}
                        </div>
                      )}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectPopup>
                    {PLUGIN_CATEGORIES.map(cat => (
                      <SelectItem key={cat.value} value={cat.value}>
                        <div className="flex flex-col gap-0.5">
                          <span>{cat.label}</span>
                          <span className="text-xs text-muted-foreground">
                            {cat.description}
                          </span>
                        </div>
                      </SelectItem>
                    ))}
                  </SelectPopup>
                </Select>
              </FormField>

              <Separator />

              {/* Execution Config */}
              <div className="flex items-center gap-2">
                <Badge variant="outline" size="sm">
                  Execution
                </Badge>
                <span className="text-xs text-muted-foreground">
                  Configure plugin execution
                </span>
              </div>

              <FormField
                label="Command"
                required
                description="Plugin executable path or command with arguments (space-separated)"
              >
                <Input
                  {...register('cmd')}
                  placeholder="./bin/my-plugin"
                  className="font-mono text-sm"
                />
              </FormField>

              <FormField
                label="Working Directory"
                description="Execution directory (optional, defaults to config directory)"
              >
                <Input
                  {...register('cwd')}
                  placeholder="/path/to/plugin"
                />
              </FormField>

              <FormField
                label="Environment Variables"
                description="One per line in KEY=value format"
              >
                <Textarea
                  {...register('env')}
                  placeholder="DEBUG=true&#10;LOG_LEVEL=info"
                  className="min-h-20 font-mono text-sm"
                />
              </FormField>

              <FormField
                label="Timeout (ms)"
                description="Maximum time for plugin to process a request"
              >
                <Input
                  type="number"
                  {...register('timeoutMs', { valueAsNumber: true })}
                  min={100}
                  max={60000}
                />
              </FormField>

              <Separator />

              {/* Flow Configuration */}
              <div className="flex items-center gap-2">
                <Badge variant="outline" size="sm">
                  Flows
                </Badge>
                <span className="text-xs text-muted-foreground">
                  When the plugin runs
                </span>
              </div>

              <div className="flex flex-col gap-3">
                {PLUGIN_FLOWS.map(flow => (
                  <label
                    key={flow.value}
                    className="flex items-start gap-3 cursor-pointer"
                  >
                    <Checkbox
                      checked={flows?.includes(flow.value)}
                      onCheckedChange={() => toggleFlow(flow.value)}
                    />
                    <div className="flex flex-col gap-0.5">
                      <span className="text-sm font-medium">{flow.label}</span>
                      <span className="text-xs text-muted-foreground">
                        {flow.description}
                      </span>
                    </div>
                  </label>
                ))}
              </div>

              <Separator />

              {/* Advanced Config */}
              <div className="flex items-center gap-2">
                <Badge variant="outline" size="sm">
                  Advanced
                </Badge>
                <span className="text-xs text-muted-foreground">
                  Additional settings
                </span>
              </div>

              <label className="flex items-start gap-3 cursor-pointer">
                <Checkbox
                  checked={watch('required')}
                  onCheckedChange={checked => setValue('required', Boolean(checked))}
                />
                <div className="flex flex-col gap-0.5">
                  <span className="text-sm font-medium">Required Plugin</span>
                  <span className="text-xs text-muted-foreground">
                    If enabled, requests will be rejected when this plugin fails
                  </span>
                </div>
              </label>

              <FormField
                label="Config JSON"
                description="Plugin-specific configuration in JSON format (automatically parsed from YAML config)"
              >
                <Textarea
                  {...register('configJson')}
                  placeholder='{"key": "value"}'
                  className="min-h-24 font-mono text-sm"
                />
              </FormField>

              <FormField
                label="Commit Hash"
                description="Optional: Pin and verify plugin binary version (validated on startup)"
              >
                <Input
                  {...register('commitHash')}
                  placeholder="abc1234"
                  className="font-mono"
                />
              </FormField>
            </m.div>
          </fieldset>
        </SheetPanel>

        <SheetFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={isSubmitting}
          >
            Cancel
          </Button>
          <Button
            onClick={handleSubmit(onSubmit)}
            disabled={isActionDisabled}
          >
            {isSubmitting
              ? (
                  <span className="flex items-center gap-2">
                    <span className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                    Saving...
                  </span>
                )
              : (
                  <span className="flex items-center gap-2">
                    {isEdit ? <SaveIcon className="size-4" /> : <PlusIcon className="size-4" />}
                    {isEdit ? 'Save Changes' : 'Add Plugin'}
                  </span>
                )}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
