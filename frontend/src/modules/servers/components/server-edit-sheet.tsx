// Input: open state, server data (for edit mode), callbacks
// Output: Sheet component for adding/editing server configurations
// Position: Overlay sheet triggered from server list or config panel

import type { ServerDetail } from '@bindings/mcpd/internal/ui'
import { ServerService } from '@bindings/mcpd/internal/ui'
import { PlusIcon, SaveIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useCallback, useEffect, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import { reloadConfig } from '@/modules/config/lib/reload-config'

interface ServerEditSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server?: ServerDetail | null
  onSaved?: () => void
}

interface FormData {
  name: string
  transport: 'stdio' | 'streamable_http'
  cmd: string
  args: string
  cwd: string
  env: string
  endpoint: string
  tags: string
  activationMode: 'on-demand' | 'always-on'
  idleSeconds: number
  maxConcurrent: number
}

const INITIAL_FORM_DATA: FormData = {
  name: '',
  transport: 'stdio',
  cmd: '',
  args: '',
  cwd: '',
  env: '',
  endpoint: '',
  tags: '',
  activationMode: 'on-demand',
  idleSeconds: 300,
  maxConcurrent: 5,
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

export function ServerEditSheet({
  open,
  onOpenChange,
  server,
  onSaved,
}: ServerEditSheetProps) {
  const isEdit = Boolean(server)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [formData, setFormData] = useState<FormData>(INITIAL_FORM_DATA)

  useEffect(() => {
    if (!open) return

    if (server) {
      const envString = server.env
        ? Object.entries(server.env)
            .map(([k, v]) => `${k}=${v}`)
            .join('\n')
        : ''
      const argsString = server.cmd.length > 1 ? server.cmd.slice(1).join(', ') : ''

      setFormData({
        name: server.name,
        transport: server.transport as 'stdio' | 'streamable_http',
        cmd: server.cmd[0] ?? '',
        args: argsString,
        cwd: server.cwd ?? '',
        env: envString,
        endpoint: server.http?.endpoint ?? '',
        tags: (server.tags ?? []).join(', '),
        activationMode: (server.activationMode as 'on-demand' | 'always-on') ?? 'on-demand',
        idleSeconds: server.idleSeconds ?? 300,
        maxConcurrent: server.maxConcurrent ?? 5,
      })
    }
    else {
      setFormData(INITIAL_FORM_DATA)
    }
  }, [server, open])

  const handleFieldChange = useCallback(
    <K extends keyof FormData>(field: K, value: FormData[K]) => {
      setFormData(prev => ({ ...prev, [field]: value }))
    },
    [],
  )

  const handleSubmit = useCallback(async () => {
    setIsSubmitting(true)
    try {
      const parsedTags = formData.tags
        .split(',')
        .map(tag => tag.trim())
        .filter(Boolean)
      const parsedArgs = formData.args
        .split(',')
        .map(arg => arg.trim())
        .filter(Boolean)
      const cmd = formData.cmd.trim()
      const envEntries = formData.env
        .split('\n')
        .map(line => line.trim())
        .filter(Boolean)
        .map((line) => {
          const [key, ...rest] = line.split('=')
          return [key?.trim(), rest.join('=').trim()] as const
        })
        .filter(([key]) => Boolean(key))
      const env = envEntries.reduce<Record<string, string>>((acc, [key, value]) => {
        if (!key) return acc
        acc[key] = value ?? ''
        return acc
      }, {})

      const baseSpec: ServerDetail = server ?? {
        name: formData.name.trim(),
        specKey: '',
        transport: formData.transport,
        cmd: [],
        env: {},
        cwd: '',
        tags: [],
        idleSeconds: formData.idleSeconds,
        maxConcurrent: formData.maxConcurrent,
        strategy: '',
        sessionTTLSeconds: 0,
        disabled: false,
        minReady: 0,
        activationMode: formData.activationMode,
        drainTimeoutSeconds: 0,
        protocolVersion: '',
        exposeTools: [],
        http: null,
      }

      const nextSpec: ServerDetail = {
        ...baseSpec,
        name: isEdit ? baseSpec.name : formData.name.trim(),
        transport: formData.transport,
        cmd: formData.transport === 'stdio' ? [cmd, ...parsedArgs].filter(Boolean) : [],
        env: formData.transport === 'stdio' ? env : {},
        cwd: formData.transport === 'stdio' ? formData.cwd.trim() : '',
        tags: parsedTags,
        idleSeconds: formData.idleSeconds,
        maxConcurrent: formData.maxConcurrent,
        activationMode: formData.activationMode,
        http: formData.transport === 'streamable_http'
          ? {
              endpoint: formData.endpoint.trim(),
              headers: baseSpec.http?.headers ?? {},
              maxRetries: baseSpec.http?.maxRetries ?? 0,
            }
          : null,
      }

      if (isEdit) {
        await ServerService.UpdateServer({ spec: nextSpec })
      }
      else {
        await ServerService.CreateServer({ spec: nextSpec })
      }

      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        toastManager.add({
          type: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }

      toastManager.add({
        type: 'success',
        title: isEdit ? 'Server updated' : 'Server added',
        description: 'Configuration saved successfully',
      })
      onSaved?.()
      onOpenChange(false)
    }
    catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to save server'
      toastManager.add({
        type: 'error',
        title: 'Failed to save',
        description: message,
      })
    }
    finally {
      setIsSubmitting(false)
    }
  }, [formData, isEdit, onSaved, onOpenChange, server])

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right">
        <SheetHeader>
          <SheetTitle>{isEdit ? 'Edit Server' : 'Add Server'}</SheetTitle>
          <SheetDescription>
            {isEdit
              ? 'Modify the MCP server configuration'
              : 'Configure a new MCP server'}
          </SheetDescription>
        </SheetHeader>

        <SheetPanel>
          <m.div
            className="space-y-6"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.2 }}
          >
            <FormField label="Server Name" required>
              <Input
                value={formData.name}
                onChange={e => handleFieldChange('name', e.target.value)}
                placeholder="my-server"
                disabled={isEdit}
              />
              {isEdit && (
                <p className="mt-1 text-xs text-muted-foreground">
                  Server name cannot be changed after creation
                </p>
              )}
            </FormField>

            <FormField label="Transport Type" required>
              <Select
                value={formData.transport}
                onValueChange={v =>
                  handleFieldChange('transport', v as 'stdio' | 'streamable_http')}
              >
                <SelectTrigger>
                  <SelectValue>
                    {value => (value ? String(value) : 'Select transport')}
                  </SelectValue>
                </SelectTrigger>
                <SelectPopup>
                  <SelectItem value="stdio">stdio</SelectItem>
                  <SelectItem value="streamable_http">streamable_http</SelectItem>
                </SelectPopup>
              </Select>
            </FormField>

            <Separator />

            {formData.transport === 'stdio' ? (
              <>
                <div className="flex items-center gap-2">
                  <Badge variant="outline" size="sm">
                    stdio
                  </Badge>
                  <span className="text-xs text-muted-foreground">
                    Configure command execution
                  </span>
                </div>

                <FormField label="Command" required description="Executable path or command">
                  <Input
                    value={formData.cmd}
                    onChange={e => handleFieldChange('cmd', e.target.value)}
                    placeholder="/usr/bin/node"
                  />
                </FormField>

                <FormField
                  label="Arguments"
                  description="Comma-separated command arguments"
                >
                  <Input
                    value={formData.args}
                    onChange={e => handleFieldChange('args', e.target.value)}
                    placeholder="server.js, --port, 3000"
                  />
                </FormField>

                <FormField label="Working Directory" description="Execution directory">
                  <Input
                    value={formData.cwd}
                    onChange={e => handleFieldChange('cwd', e.target.value)}
                    placeholder="/path/to/project"
                  />
                </FormField>

                <FormField
                  label="Environment Variables"
                  description="One per line: KEY=value"
                >
                  <Textarea
                    value={formData.env}
                    onChange={e => handleFieldChange('env', e.target.value)}
                    placeholder="NODE_ENV=production&#10;API_KEY=secret"
                    className="min-h-24"
                  />
                </FormField>
              </>
            ) : (
              <>
                <div className="flex items-center gap-2">
                  <Badge variant="outline" size="sm">
                    streamable_http
                  </Badge>
                  <span className="text-xs text-muted-foreground">
                    Configure HTTP endpoint
                  </span>
                </div>

                <FormField
                  label="Endpoint URL"
                  required
                  description="HTTP endpoint for the MCP server"
                >
                  <Input
                    value={formData.endpoint}
                    onChange={e => handleFieldChange('endpoint', e.target.value)}
                    placeholder="http://localhost:3000/mcp"
                  />
                </FormField>
              </>
            )}

            <Separator />

            <FormField label="Tags" description="Comma-separated tags for organization">
              <Input
                value={formData.tags}
                onChange={e => handleFieldChange('tags', e.target.value)}
                placeholder="production, api, core"
              />
            </FormField>

            <FormField label="Activation Mode">
              <Select
                value={formData.activationMode}
                onValueChange={v =>
                  handleFieldChange('activationMode', v as 'on-demand' | 'always-on')}
              >
                <SelectTrigger>
                  <SelectValue>
                    {value =>
                      value === 'on-demand'
                        ? 'On Demand'
                        : value === 'always-on'
                          ? 'Always On'
                          : 'Select mode'}
                  </SelectValue>
                </SelectTrigger>
                <SelectPopup>
                  <SelectItem value="on-demand">On Demand</SelectItem>
                  <SelectItem value="always-on">Always On</SelectItem>
                </SelectPopup>
              </Select>
            </FormField>

            <div className="grid grid-cols-2 gap-4">
              <FormField
                label="Idle Timeout"
                description="Seconds before idle shutdown"
              >
                <Input
                  type="number"
                  value={formData.idleSeconds}
                  onChange={e =>
                    handleFieldChange('idleSeconds', Number.parseInt(e.target.value, 10) || 0)}
                  min={0}
                />
              </FormField>

              <FormField
                label="Max Concurrency"
                description="Maximum concurrent requests"
              >
                <Input
                  type="number"
                  value={formData.maxConcurrent}
                  onChange={e =>
                    handleFieldChange('maxConcurrent', Number.parseInt(e.target.value, 10) || 1)}
                  min={1}
                />
              </FormField>
            </div>
          </m.div>
        </SheetPanel>

        <SheetFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={isSubmitting}>
            {isSubmitting ? (
              'Saving...'
            ) : isEdit
              ? (
                  <>
                    <SaveIcon className="mr-2 size-4" />
                    Save Changes
                  </>
                )
              : (
                  <>
                    <PlusIcon className="mr-2 size-4" />
                    Add Server
                  </>
                )}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
