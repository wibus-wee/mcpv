// Input: open state, server data (for edit mode), callbacks
// Output: Sheet component for adding/editing server configurations
// Position: Overlay sheet triggered from server list or config panel

import type { ServerDetail } from '@bindings/mcpv/internal/ui'
import { ServerService } from '@bindings/mcpv/internal/ui'
import { PlusIcon, SaveIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useCallback, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'

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
import { formatCommaSeparated, formatEnvironmentVariables, parseCommaSeparated, parseEnvironmentVariables } from '@/lib/parsers'
import { reloadConfig } from '@/modules/servers/lib/reload-config'

interface ServerEditSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server?: ServerDetail | null
  editTargetName?: string | null
  isLoading?: boolean
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
  minReady: number
  strategy: string
  sessionTTLSeconds: number
  drainTimeoutSeconds: number
  exposeTools: string
  httpMaxRetries: number
  httpHeaders: string
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
  minReady: 0,
  strategy: 'stateless',
  sessionTTLSeconds: 3600,
  drainTimeoutSeconds: 30,
  exposeTools: '',
  httpMaxRetries: 3,
  httpHeaders: '',
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
  editTargetName,
  isLoading,
  onSaved,
}: ServerEditSheetProps) {
  const isEdit = Boolean(server ?? editTargetName)
  const isEditLoading = Boolean(isLoading) && isEdit && !server
  const isMissingServer = isEdit && !server
  const isFormDisabled = isEditLoading || isMissingServer
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

  const transport = watch('transport')

  useEffect(() => {
    if (!open) return

    if (server) {
      const envString = formatEnvironmentVariables(server.env ?? {})
      const argsString = server.cmd.length > 1 ? formatCommaSeparated(server.cmd.slice(1)) : ''
      const exposeToolsString = formatCommaSeparated(server.exposeTools ?? [])
      const httpHeadersString = formatEnvironmentVariables(server.http?.headers ?? {})

      reset({
        name: server.name,
        transport: server.transport as 'stdio' | 'streamable_http',
        cmd: server.cmd[0] ?? '',
        args: argsString,
        cwd: server.cwd ?? '',
        env: envString,
        endpoint: server.http?.endpoint ?? '',
        tags: formatCommaSeparated(server.tags ?? []),
        activationMode: (server.activationMode as 'on-demand' | 'always-on') ?? 'on-demand',
        idleSeconds: server.idleSeconds ?? 300,
        maxConcurrent: server.maxConcurrent ?? 5,
        minReady: server.minReady ?? 0,
        strategy: server.strategy ?? 'stateless',
        sessionTTLSeconds: server.sessionTTLSeconds ?? 3600,
        drainTimeoutSeconds: server.drainTimeoutSeconds ?? 30,
        exposeTools: exposeToolsString,
        httpMaxRetries: server.http?.maxRetries ?? 3,
        httpHeaders: httpHeadersString,
      })
    }
    else {
      reset({
        ...INITIAL_FORM_DATA,
        name: editTargetName ?? '',
      })
    }
  }, [server, open, reset, editTargetName])

  const onSubmit = useCallback(async (data: FormData) => {
    if (isEdit && !server) {
      toastManager.add({
        type: 'error',
        title: 'Server not ready',
        description: 'Wait for the configuration to load before saving.',
      })
      return
    }

    setIsSubmitting(true)
    try {
      const filteredTags = parseCommaSeparated(data.tags)
      const filteredArgs = parseCommaSeparated(data.args)
      const env = parseEnvironmentVariables(data.env)
      const cmd = data.cmd.trim()
      const exposeTools = parseCommaSeparated(data.exposeTools)
      const httpHeaders = parseEnvironmentVariables(data.httpHeaders)

      const baseSpec: ServerDetail = server ?? ({
        name: data.name.trim(),
        specKey: '',
        transport: data.transport,
        cmd: [],
        env: {},
        cwd: '',
        tags: [],
        idleSeconds: data.idleSeconds,
        maxConcurrent: data.maxConcurrent,
        strategy: '',
        sessionTTLSeconds: 0,
        disabled: false,
        minReady: 0,
        activationMode: data.activationMode,
        drainTimeoutSeconds: 0,
        protocolVersion: '',
        exposeTools: [],
        http: null,
      })

      const nextSpec: ServerDetail = {
        ...baseSpec,
        name: isEdit ? baseSpec.name : data.name.trim(),
        transport: data.transport,
        cmd: data.transport === 'stdio' ? [cmd, ...filteredArgs].filter(Boolean) : [],
        env: data.transport === 'stdio' ? env : {},
        cwd: data.transport === 'stdio' ? data.cwd.trim() : '',
        tags: filteredTags,
        idleSeconds: data.idleSeconds,
        maxConcurrent: data.maxConcurrent,
        minReady: data.minReady,
        strategy: data.strategy,
        sessionTTLSeconds: data.sessionTTLSeconds,
        drainTimeoutSeconds: data.drainTimeoutSeconds,
        activationMode: data.activationMode,
        exposeTools,
        http: data.transport === 'streamable_http'
          ? {
              endpoint: data.endpoint.trim(),
              headers: httpHeaders,
              maxRetries: data.httpMaxRetries,
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
  }, [isEdit, onSaved, onOpenChange, server])

  const isActionDisabled = isSubmitting || isEditLoading || isMissingServer

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
          <fieldset disabled={isFormDisabled} className="min-h-full">
            <m.div
              className="space-y-6"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2 }}
            >
              <FormField label="Server Name" required>
                <Input
                  {...register('name')}
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
                  value={transport}
                  onValueChange={v => setValue('transport', v as 'stdio' | 'streamable_http')}
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

              {transport === 'stdio' ? (
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
                      {...register('cmd')}
                      placeholder="/usr/bin/node"
                    />
                  </FormField>

                  <FormField
                    label="Arguments"
                    description="Comma-separated command arguments"
                  >
                    <Input
                      {...register('args')}
                      placeholder="server.js, --port, 3000"
                    />
                  </FormField>

                  <FormField label="Working Directory" description="Execution directory">
                    <Input
                      {...register('cwd')}
                      placeholder="/path/to/project"
                    />
                  </FormField>

                  <FormField
                    label="Environment Variables"
                    description="One per line: KEY=value"
                  >
                    <Textarea
                      {...register('env')}
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
                      {...register('endpoint')}
                      placeholder="http://localhost:3000/mcp"
                    />
                  </FormField>

                  <FormField
                    label="Max Retries"
                    description="Maximum number of retries for HTTP requests"
                  >
                    <Input
                      type="number"
                      {...register('httpMaxRetries', { valueAsNumber: true })}
                      min={0}
                    />
                  </FormField>

                  <FormField
                    label="HTTP Headers"
                    description="One per line: Header-Name=value"
                  >
                    <Textarea
                      {...register('httpHeaders')}
                      placeholder="Authorization=Bearer token&#10;Content-Type=application/json"
                      className="min-h-24"
                    />
                  </FormField>
                </>
              )}

              <Separator />

              <FormField label="Tags" description="Comma-separated tags for organization">
                <Input
                  {...register('tags')}
                  placeholder="production, api, core"
                />
              </FormField>

              <FormField label="Activation Mode">
                <Select
                  value={watch('activationMode')}
                  onValueChange={v => setValue('activationMode', v as 'on-demand' | 'always-on')}
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

              <FormField label="Strategy">
                <Select
                  value={watch('strategy')}
                  onValueChange={v => setValue('strategy', v as string)}
                >
                  <SelectTrigger>
                    <SelectValue>
                      {value => value || 'Select strategy'}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectPopup>
                    <SelectItem value="stateless">Stateless</SelectItem>
                    <SelectItem value="stateful">Stateful</SelectItem>
                  </SelectPopup>
                </Select>
              </FormField>

              <div className="grid grid-cols-2 gap-4">
                <FormField
                  label="Drain Timeout"
                  description="Drain timeout in seconds"
                >
                  <Input
                    type="number"
                    {...register('drainTimeoutSeconds', { valueAsNumber: true })}
                    min={0}
                  />
                </FormField>

                <div />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <FormField
                  label="Idle Timeout"
                  description="Seconds before idle shutdown"
                >
                  <Input
                    type="number"
                    {...register('idleSeconds', { valueAsNumber: true })}
                    min={0}
                  />
                </FormField>

                <FormField
                  label="Max Concurrency"
                  description="Maximum concurrent requests"
                >
                  <Input
                    type="number"
                    {...register('maxConcurrent', { valueAsNumber: true })}
                    min={1}
                  />
                </FormField>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <FormField
                  label="Min Ready"
                  description="Minimum ready instances"
                >
                  <Input
                    type="number"
                    {...register('minReady', { valueAsNumber: true })}
                    min={0}
                  />
                </FormField>

                <FormField
                  label="Session TTL"
                  description="Session time-to-live in seconds"
                >
                  <Input
                    type="number"
                    {...register('sessionTTLSeconds', { valueAsNumber: true })}
                    min={0}
                  />
                </FormField>
              </div>
            </m.div>
          </fieldset>
        </SheetPanel>

        <SheetFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit(onSubmit)} disabled={isActionDisabled}>
            {isActionDisabled && isEditLoading
              ? 'Loading...'
              : isSubmitting
                ? 'Saving...'
                : isEdit
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
