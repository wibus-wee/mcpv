// Input: open state, server data (for edit mode), callbacks, help content
// Output: Sheet component for adding/editing server configurations with guidance
// Position: Overlay sheet triggered from server list or config panel

import { ServerService } from '@bindings/mcpv/internal/ui/services'
import type { ServerDetail } from '@bindings/mcpv/internal/ui/types'
import { AlertTriangleIcon, ChevronDownIcon, InfoIcon, PlusIcon, SaveIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
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
import { formatCommaSeparated, formatEnvironmentVariables, parseCommaSeparated, parseEnvironmentVariables } from '@/lib/parsers'
import { reloadConfig } from '@/modules/servers/lib/reload-config'
import type { ServerFormValues } from '@/modules/servers/lib/server-form-content'
import {
  SERVER_ADVICE_RULES,
  SERVER_FIELD_HELP,
  SERVER_FIELD_IDS,
  SERVER_FORM_TEXT,
  SERVER_FORM_VALIDATION,
  SERVER_SELECT_OPTIONS,
} from '@/modules/servers/lib/server-form-content'

import { ServerFormField } from './server-form-field'

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

const resolveSelectLabel = (
  options: Array<{ value: string, label: string }>,
  value: unknown,
  placeholder: string,
) => {
  if (!value) return placeholder
  const match = options.find(option => option.value === value)
  return match ? match.label : placeholder
}

const getErrorMessage = (message: unknown) => (typeof message === 'string' ? message : undefined)

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
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [httpAdvancedOpen, setHttpAdvancedOpen] = useState(false)

  const form = useForm<FormData>({
    defaultValues: INITIAL_FORM_DATA,
  })

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    control,
    formState: { errors },
  } = form

  const [
    transportRaw,
    activationModeRaw,
    strategyRaw,
    idleSecondsRaw,
    maxConcurrentRaw,
    minReadyRaw,
    sessionTTLSecondsRaw,
    drainTimeoutSecondsRaw,
  ] = useWatch({
    control,
    name: [
      'transport',
      'activationMode',
      'strategy',
      'idleSeconds',
      'maxConcurrent',
      'minReady',
      'sessionTTLSeconds',
      'drainTimeoutSeconds',
    ],
  })
  const transport = (transportRaw ?? 'stdio') as FormData['transport']
  const activationMode = (activationModeRaw ?? 'on-demand') as FormData['activationMode']
  const strategy = (strategyRaw ?? 'stateless') as string
  const idleSeconds = Number(idleSecondsRaw ?? 0)
  const maxConcurrent = Number(maxConcurrentRaw ?? 0)
  const minReady = Number(minReadyRaw ?? 0)
  const sessionTTLSeconds = Number(sessionTTLSecondsRaw ?? 0)
  const drainTimeoutSeconds = Number(drainTimeoutSecondsRaw ?? 0)

  const adviceItems = useMemo(() => {
    const values: ServerFormValues = {
      transport,
      activationMode,
      idleSeconds,
      maxConcurrent,
      minReady,
      strategy,
      sessionTTLSeconds,
      drainTimeoutSeconds,
    }
    return SERVER_ADVICE_RULES.filter(rule => rule.when(values))
  }, [
    transport,
    activationMode,
    idleSeconds,
    maxConcurrent,
    minReady,
    strategy,
    sessionTTLSeconds,
    drainTimeoutSeconds,
  ])

  const warningAdvice = adviceItems.filter(item => item.severity === 'warning')
  const infoAdvice = adviceItems.filter(item => item.severity === 'info')

  useEffect(() => {
    if (!open) return

    if (server) {
      // Filter out undefined values from env and headers
      const env = Object.fromEntries(
        Object.entries(server.env ?? {}).filter(([_, value]) => value !== undefined),
      ) as Record<string, string>
      const headers = Object.fromEntries(
        Object.entries(server.http?.headers ?? {}).filter(([_, value]) => value !== undefined),
      ) as Record<string, string>

      const envString = formatEnvironmentVariables(env)
      const argsString = server.cmd.length > 1 ? formatCommaSeparated(server.cmd.slice(1)) : ''
      const exposeToolsString = formatCommaSeparated(server.exposeTools ?? [])
      const httpHeadersString = formatEnvironmentVariables(headers)

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
        track(AnalyticsEvents.SERVER_SAVE, {
          mode: isEdit ? 'edit' : 'create',
          result: 'reload_failed',
          transport: data.transport,
          activation_mode: data.activationMode,
          strategy: data.strategy,
          tags_count: filteredTags.length,
          expose_tools_count: exposeTools.length,
        })
        toastManager.add({
          type: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }

      track(AnalyticsEvents.SERVER_SAVE, {
        mode: isEdit ? 'edit' : 'create',
        result: 'success',
        transport: data.transport,
        activation_mode: data.activationMode,
        strategy: data.strategy,
        tags_count: filteredTags.length,
        expose_tools_count: exposeTools.length,
      })
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
      track(AnalyticsEvents.SERVER_SAVE, {
        mode: isEdit ? 'edit' : 'create',
        result: 'error',
        transport: data.transport,
        activation_mode: data.activationMode,
        strategy: data.strategy,
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
              <ServerFormField
                id={SERVER_FIELD_IDS.name}
                label="Server Name"
                required
                help={SERVER_FIELD_HELP.name}
                error={getErrorMessage(errors.name?.message)}
              >
                <Input
                  {...register('name', { required: SERVER_FORM_VALIDATION.nameRequired })}
                  id={SERVER_FIELD_IDS.name}
                  placeholder={SERVER_FORM_TEXT.placeholders.name}
                  disabled={isEdit}
                />
                {isEdit ? (
                  <p className="mt-1 text-xs text-muted-foreground">
                    Server name cannot be changed after creation
                  </p>
                ) : null}
              </ServerFormField>

              <ServerFormField
                id={SERVER_FIELD_IDS.transport}
                label="Transport Type"
                required
                help={SERVER_FIELD_HELP.transport}
                error={getErrorMessage(errors.transport?.message)}
              >
                <Select
                  value={transport}
                  onValueChange={v => setValue('transport', v as 'stdio' | 'streamable_http')}
                >
                  <SelectTrigger id={SERVER_FIELD_IDS.transport}>
                    <SelectValue>
                      {value => resolveSelectLabel(
                        SERVER_SELECT_OPTIONS.transport,
                        value,
                        SERVER_FORM_TEXT.selectPlaceholders.transport,
                      )}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectPopup>
                    {SERVER_SELECT_OPTIONS.transport.map(option => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectPopup>
                </Select>
              </ServerFormField>

              <Separator />

              {transport === 'stdio' ? (
                <>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" size="sm">
                      {SERVER_FORM_TEXT.badges.stdio}
                    </Badge>
                    <span className="text-xs text-muted-foreground">
                      {SERVER_FORM_TEXT.transportSummaries.stdio}
                    </span>
                  </div>

                  <ServerFormField
                    id={SERVER_FIELD_IDS.cmd}
                    label="Command"
                    required
                    description={SERVER_FORM_TEXT.descriptions.cmd}
                    help={SERVER_FIELD_HELP.cmd}
                    error={getErrorMessage(errors.cmd?.message)}
                  >
                    <Input
                      {...register('cmd', {
                        validate: value => (
                          transport === 'stdio'
                            ? (value?.trim()
                                ? true
                                : SERVER_FORM_VALIDATION.cmdRequired)
                            : true
                        ),
                      })}
                      id={SERVER_FIELD_IDS.cmd}
                      placeholder={SERVER_FORM_TEXT.placeholders.cmd}
                    />
                  </ServerFormField>

                  <ServerFormField
                    id={SERVER_FIELD_IDS.args}
                    label="Arguments"
                    description={SERVER_FORM_TEXT.descriptions.args}
                    help={SERVER_FIELD_HELP.args}
                    error={getErrorMessage(errors.args?.message)}
                  >
                    <Input
                      {...register('args')}
                      id={SERVER_FIELD_IDS.args}
                      placeholder={SERVER_FORM_TEXT.placeholders.args}
                    />
                  </ServerFormField>

                  <ServerFormField
                    id={SERVER_FIELD_IDS.cwd}
                    label="Working Directory"
                    description={SERVER_FORM_TEXT.descriptions.cwd}
                    help={SERVER_FIELD_HELP.cwd}
                    error={getErrorMessage(errors.cwd?.message)}
                  >
                    <Input
                      {...register('cwd')}
                      id={SERVER_FIELD_IDS.cwd}
                      placeholder={SERVER_FORM_TEXT.placeholders.cwd}
                    />
                  </ServerFormField>

                  <ServerFormField
                    id={SERVER_FIELD_IDS.env}
                    label="Environment Variables"
                    description={SERVER_FORM_TEXT.descriptions.env}
                    help={SERVER_FIELD_HELP.env}
                    error={getErrorMessage(errors.env?.message)}
                  >
                    <Textarea
                      {...register('env')}
                      id={SERVER_FIELD_IDS.env}
                      placeholder={SERVER_FORM_TEXT.placeholders.env}
                      className="min-h-24"
                    />
                  </ServerFormField>
                </>
              ) : (
                <>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" size="sm">
                      {SERVER_FORM_TEXT.badges.streamableHttp}
                    </Badge>
                    <span className="text-xs text-muted-foreground">
                      {SERVER_FORM_TEXT.transportSummaries.streamableHttp}
                    </span>
                  </div>

                  <ServerFormField
                    id={SERVER_FIELD_IDS.endpoint}
                    label="Endpoint URL"
                    required
                    description={SERVER_FORM_TEXT.descriptions.endpoint}
                    help={SERVER_FIELD_HELP.endpoint}
                    error={getErrorMessage(errors.endpoint?.message)}
                  >
                    <Input
                      {...register('endpoint', {
                        validate: value => (
                          transport === 'streamable_http'
                            ? (value?.trim()
                                ? true
                                : SERVER_FORM_VALIDATION.endpointRequired)
                            : true
                        ),
                      })}
                      id={SERVER_FIELD_IDS.endpoint}
                      placeholder={SERVER_FORM_TEXT.placeholders.endpoint}
                    />
                  </ServerFormField>

                  <Collapsible open={httpAdvancedOpen} onOpenChange={setHttpAdvancedOpen}>
                    <CollapsibleTrigger className="w-full">
                      <Button
                        variant="ghost"
                        className="w-full justify-between h-auto px-3 py-2"
                      >
                        <span className="text-sm font-medium">
                          {SERVER_FORM_TEXT.advanced.toggleLabel}
                        </span>
                        <m.div
                          animate={{ rotate: httpAdvancedOpen ? 180 : 0 }}
                          transition={{ duration: 0.2 }}
                        >
                          <ChevronDownIcon className="size-4" />
                        </m.div>
                      </Button>
                    </CollapsibleTrigger>
                    <CollapsibleContent>
                      <m.div
                        className="mt-3 space-y-4"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        transition={{ duration: 0.2 }}
                      >
                        <p className="text-xs text-muted-foreground">
                          {SERVER_FORM_TEXT.advanced.description}
                        </p>
                        <ServerFormField
                          id={SERVER_FIELD_IDS.httpMaxRetries}
                          label="Max Retries"
                          description={SERVER_FORM_TEXT.descriptions.httpMaxRetries}
                          help={SERVER_FIELD_HELP.httpMaxRetries}
                          error={getErrorMessage(errors.httpMaxRetries?.message)}
                        >
                          <Input
                            type="number"
                            {...register('httpMaxRetries', {
                              valueAsNumber: true,
                              min: { value: 0, message: SERVER_FORM_VALIDATION.minZero },
                            })}
                            id={SERVER_FIELD_IDS.httpMaxRetries}
                            min={0}
                          />
                        </ServerFormField>

                        <ServerFormField
                          id={SERVER_FIELD_IDS.httpHeaders}
                          label="HTTP Headers"
                          description={SERVER_FORM_TEXT.descriptions.httpHeaders}
                          help={SERVER_FIELD_HELP.httpHeaders}
                          error={getErrorMessage(errors.httpHeaders?.message)}
                        >
                          <Textarea
                            {...register('httpHeaders')}
                            id={SERVER_FIELD_IDS.httpHeaders}
                            placeholder={SERVER_FORM_TEXT.placeholders.httpHeaders}
                            className="min-h-24"
                          />
                        </ServerFormField>
                      </m.div>
                    </CollapsibleContent>
                  </Collapsible>
                </>
              )}

              <Separator />

              <ServerFormField
                id={SERVER_FIELD_IDS.tags}
                label="Tags"
                description={SERVER_FORM_TEXT.descriptions.tags}
                help={SERVER_FIELD_HELP.tags}
                error={getErrorMessage(errors.tags?.message)}
              >
                <Input
                  {...register('tags')}
                  id={SERVER_FIELD_IDS.tags}
                  placeholder={SERVER_FORM_TEXT.placeholders.tags}
                />
              </ServerFormField>

              <ServerFormField
                id={SERVER_FIELD_IDS.activationMode}
                label="Activation Mode"
                help={SERVER_FIELD_HELP.activationMode}
                error={getErrorMessage(errors.activationMode?.message)}
              >
                <Select
                  value={activationMode}
                  onValueChange={v => setValue('activationMode', v as 'on-demand' | 'always-on')}
                >
                  <SelectTrigger id={SERVER_FIELD_IDS.activationMode}>
                    <SelectValue>
                      {value => resolveSelectLabel(
                        SERVER_SELECT_OPTIONS.activationMode,
                        value,
                        SERVER_FORM_TEXT.selectPlaceholders.activationMode,
                      )}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectPopup>
                    {SERVER_SELECT_OPTIONS.activationMode.map(option => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectPopup>
                </Select>
              </ServerFormField>

              <ServerFormField
                id={SERVER_FIELD_IDS.strategy}
                label="Strategy"
                help={SERVER_FIELD_HELP.strategy}
                error={getErrorMessage(errors.strategy?.message)}
              >
                <Select
                  value={strategy}
                  onValueChange={v => setValue('strategy', v as string)}
                >
                  <SelectTrigger id={SERVER_FIELD_IDS.strategy}>
                    <SelectValue>
                      {value => resolveSelectLabel(
                        SERVER_SELECT_OPTIONS.strategy,
                        value,
                        SERVER_FORM_TEXT.selectPlaceholders.strategy,
                      )}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectPopup>
                    {SERVER_SELECT_OPTIONS.strategy.map(option => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectPopup>
                </Select>
              </ServerFormField>

              <div className="grid grid-cols-2 gap-4">
                <ServerFormField
                  id={SERVER_FIELD_IDS.idleSeconds}
                  label="Idle Timeout"
                  description={SERVER_FORM_TEXT.descriptions.idleSeconds}
                  help={SERVER_FIELD_HELP.idleSeconds}
                  error={getErrorMessage(errors.idleSeconds?.message)}
                >
                  <Input
                    type="number"
                    {...register('idleSeconds', {
                      valueAsNumber: true,
                      min: { value: 0, message: SERVER_FORM_VALIDATION.minZero },
                    })}
                    id={SERVER_FIELD_IDS.idleSeconds}
                    min={0}
                  />
                </ServerFormField>

                <ServerFormField
                  id={SERVER_FIELD_IDS.maxConcurrent}
                  label="Max Concurrency"
                  description={SERVER_FORM_TEXT.descriptions.maxConcurrent}
                  help={SERVER_FIELD_HELP.maxConcurrent}
                  error={getErrorMessage(errors.maxConcurrent?.message)}
                >
                  <Input
                    type="number"
                    {...register('maxConcurrent', {
                      valueAsNumber: true,
                      min: { value: 1, message: SERVER_FORM_VALIDATION.minOne },
                    })}
                    id={SERVER_FIELD_IDS.maxConcurrent}
                    min={1}
                  />
                </ServerFormField>
              </div>

              {warningAdvice.length > 0 ? (
                <Alert variant="warning">
                  <AlertTriangleIcon />
                  <AlertTitle>{SERVER_FORM_TEXT.advice.title}</AlertTitle>
                  <AlertDescription>
                    <ul className="list-disc space-y-1 pl-4">
                      {warningAdvice.map(item => (
                        <li key={item.id}>{item.message}</li>
                      ))}
                    </ul>
                  </AlertDescription>
                </Alert>
              ) : null}

              {infoAdvice.length > 0 ? (
                <Alert variant="info">
                  <InfoIcon />
                  <AlertTitle>{SERVER_FORM_TEXT.advice.title}</AlertTitle>
                  <AlertDescription>
                    <ul className="list-disc space-y-1 pl-4">
                      {infoAdvice.map(item => (
                        <li key={item.id}>{item.message}</li>
                      ))}
                    </ul>
                  </AlertDescription>
                </Alert>
              ) : null}

              <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
                <CollapsibleTrigger className="w-full">
                  <Button
                    variant="ghost"
                    className="w-full justify-between h-auto px-3 py-2"
                  >
                    <span className="text-sm font-medium">
                      {SERVER_FORM_TEXT.advanced.toggleLabel}
                    </span>
                    <m.div
                      animate={{ rotate: advancedOpen ? 180 : 0 }}
                      transition={{ duration: 0.2 }}
                    >
                      <ChevronDownIcon className="size-4" />
                    </m.div>
                  </Button>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <m.div
                    className="mt-3 space-y-4"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.2 }}
                  >
                    <p className="text-xs text-muted-foreground">
                      {SERVER_FORM_TEXT.advanced.description}
                    </p>
                    <div className="grid grid-cols-2 gap-4">
                      <ServerFormField
                        id={SERVER_FIELD_IDS.minReady}
                        label="Min Ready"
                        description={SERVER_FORM_TEXT.descriptions.minReady}
                        help={SERVER_FIELD_HELP.minReady}
                        error={getErrorMessage(errors.minReady?.message)}
                      >
                        <Input
                          type="number"
                          {...register('minReady', {
                            valueAsNumber: true,
                            min: { value: 0, message: SERVER_FORM_VALIDATION.minZero },
                          })}
                          id={SERVER_FIELD_IDS.minReady}
                          min={0}
                        />
                      </ServerFormField>

                      <ServerFormField
                        id={SERVER_FIELD_IDS.sessionTTLSeconds}
                        label="Session TTL"
                        description={SERVER_FORM_TEXT.descriptions.sessionTTLSeconds}
                        help={SERVER_FIELD_HELP.sessionTTLSeconds}
                        error={getErrorMessage(errors.sessionTTLSeconds?.message)}
                      >
                        <Input
                          type="number"
                          {...register('sessionTTLSeconds', {
                            valueAsNumber: true,
                            min: { value: 0, message: SERVER_FORM_VALIDATION.minZero },
                          })}
                          id={SERVER_FIELD_IDS.sessionTTLSeconds}
                          min={0}
                        />
                      </ServerFormField>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <ServerFormField
                        id={SERVER_FIELD_IDS.drainTimeoutSeconds}
                        label="Drain Timeout"
                        description={SERVER_FORM_TEXT.descriptions.drainTimeoutSeconds}
                        help={SERVER_FIELD_HELP.drainTimeoutSeconds}
                        error={getErrorMessage(errors.drainTimeoutSeconds?.message)}
                      >
                        <Input
                          type="number"
                          {...register('drainTimeoutSeconds', {
                            valueAsNumber: true,
                            min: { value: 0, message: SERVER_FORM_VALIDATION.minZero },
                          })}
                          id={SERVER_FIELD_IDS.drainTimeoutSeconds}
                          min={0}
                        />
                      </ServerFormField>
                      <div />
                    </div>
                  </m.div>
                </CollapsibleContent>
              </Collapsible>
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
