// Input: ConfigService/ProfileService bindings, react-hook-form, config mode hook, UI components, SWR
// Output: SettingsPage component with runtime configuration form
// Position: Settings module page for global runtime settings

import type {
  ProfileDetail,
  ProfileSummary,
  RuntimeConfigDetail,
} from '@bindings/mcpd/internal/ui'
import { ConfigService, ProfileService } from '@bindings/mcpd/internal/ui'
import {
  AlertCircleIcon,
  SaveIcon,
  SettingsIcon,
  ShieldAlertIcon,
} from 'lucide-react'
import { m } from 'motion/react'
import { useEffect } from 'react'
import {
  Controller,
  useForm,
  type UseFormRegisterReturn,
} from 'react-hook-form'
import useSWR, { useSWRConfig } from 'swr'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Form } from '@/components/ui/form'
import {
  InputGroup,
  InputGroupInput,
  InputGroupText,
} from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { toastManager } from '@/components/ui/toast'
import { Spring } from '@/lib/spring'
import { cn } from '@/lib/utils'
import { useConfigMode } from '@/modules/config/hooks'
import { reloadConfig } from '@/modules/config/lib/reload-config'

const BOOTSTRAP_MODE_OPTIONS = [
  { value: 'metadata', label: 'Metadata' },
  { value: 'disabled', label: 'Disabled' },
] as const

const ACTIVATION_MODE_OPTIONS = [
  { value: 'on-demand', label: 'On-demand' },
  { value: 'always-on', label: 'Always-on' },
] as const

const NAMESPACE_STRATEGY_OPTIONS = [
  { value: 'prefix', label: 'Prefix' },
  { value: 'flat', label: 'Flat' },
] as const

const BOOTSTRAP_MODE_LABELS: Record<string, string> = {
  metadata: 'Metadata',
  disabled: 'Disabled',
}

const ACTIVATION_MODE_LABELS: Record<string, string> = {
  'on-demand': 'On-demand',
  'always-on': 'Always-on',
}

const NAMESPACE_STRATEGY_LABELS: Record<string, string> = {
  prefix: 'Prefix',
  flat: 'Flat',
}

type RuntimeFormState = {
  routeTimeoutSeconds: number
  pingIntervalSeconds: number
  toolRefreshSeconds: number
  toolRefreshConcurrency: number
  callerCheckSeconds: number
  callerInactiveSeconds: number
  serverInitRetryBaseSeconds: number
  serverInitRetryMaxSeconds: number
  serverInitMaxRetries: number
  bootstrapMode: string
  bootstrapConcurrency: number
  bootstrapTimeoutSeconds: number
  defaultActivationMode: string
  exposeTools: boolean
  toolNamespaceStrategy: string
}

const DEFAULT_RUNTIME_FORM: RuntimeFormState = {
  routeTimeoutSeconds: 0,
  pingIntervalSeconds: 0,
  toolRefreshSeconds: 0,
  toolRefreshConcurrency: 0,
  callerCheckSeconds: 0,
  callerInactiveSeconds: 0,
  serverInitRetryBaseSeconds: 0,
  serverInitRetryMaxSeconds: 0,
  serverInitMaxRetries: 0,
  bootstrapMode: 'metadata',
  bootstrapConcurrency: 0,
  bootstrapTimeoutSeconds: 0,
  defaultActivationMode: 'on-demand',
  exposeTools: false,
  toolNamespaceStrategy: 'prefix',
}

const toRuntimeFormState = (runtime: RuntimeConfigDetail): RuntimeFormState => ({
  routeTimeoutSeconds: runtime.routeTimeoutSeconds,
  pingIntervalSeconds: runtime.pingIntervalSeconds,
  toolRefreshSeconds: runtime.toolRefreshSeconds,
  toolRefreshConcurrency: runtime.toolRefreshConcurrency,
  callerCheckSeconds: runtime.callerCheckSeconds,
  callerInactiveSeconds: runtime.callerInactiveSeconds,
  serverInitRetryBaseSeconds: runtime.serverInitRetryBaseSeconds,
  serverInitRetryMaxSeconds: runtime.serverInitRetryMaxSeconds,
  serverInitMaxRetries: runtime.serverInitMaxRetries,
  bootstrapMode: runtime.bootstrapMode || 'metadata',
  bootstrapConcurrency: runtime.bootstrapConcurrency,
  bootstrapTimeoutSeconds: runtime.bootstrapTimeoutSeconds,
  defaultActivationMode: runtime.defaultActivationMode || 'on-demand',
  exposeTools: runtime.exposeTools,
  toolNamespaceStrategy: runtime.toolNamespaceStrategy || 'prefix',
})

const normalizeNumber = (value: string) => {
  const nextValue = Number(value)
  return Number.isNaN(nextValue) ? 0 : nextValue
}

const SettingsHeader = () => (
  <m.div
    className="p-6 pb-0"
    initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
    animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
    transition={Spring.smooth(0.4)}
  >
    <div className="flex items-center gap-2">
      <SettingsIcon className="size-4 text-muted-foreground" />
      <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
    </div>
    <p className="text-muted-foreground text-sm">
      Runtime defaults shared across all profiles
    </p>
  </m.div>
)

interface RuntimeFieldRowProps {
  label: string
  description?: string
  htmlFor: string
  children: React.ReactNode
  className?: string
}

const RuntimeFieldRow = ({
  label,
  description,
  htmlFor,
  children,
  className,
}: RuntimeFieldRowProps) => (
  <div
    className={cn(
      'grid gap-3 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,280px)] sm:items-center',
      className,
    )}
  >
    <div className="space-y-1">
      <Label htmlFor={htmlFor}>{label}</Label>
      {description && (
        <p className="text-xs text-muted-foreground">{description}</p>
      )}
    </div>
    <div className="w-full sm:max-w-64 sm:justify-self-end">
      {children}
    </div>
  </div>
)

interface RuntimeNumberRowProps {
  id: string
  label: string
  description: string
  unit?: string
  disabled?: boolean
  inputProps: UseFormRegisterReturn
}

const RuntimeNumberRow = ({
  id,
  label,
  description,
  unit,
  disabled,
  inputProps,
}: RuntimeNumberRowProps) => (
  <RuntimeFieldRow label={label} description={description} htmlFor={id}>
    <InputGroup className="w-full">
      <InputGroupInput
        id={id}
        type="number"
        min={0}
        step={1}
        disabled={disabled}
        inputMode="numeric"
        {...inputProps}
      />
      {unit && (
        <InputGroupText className="text-xs text-muted-foreground pr-4">
          {unit}
        </InputGroupText>
      )}
    </InputGroup>
  </RuntimeFieldRow>
)

const RuntimeSkeleton = () => (
  <div className="space-y-4">
    <div className="space-y-2">
      <Skeleton className="h-4 w-24" />
      <div className="space-y-3">
        {Array.from({ length: 4 }).map((_, index) => (
          <div
            key={index}
            className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,280px)]"
          >
            <div className="space-y-2">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-3 w-48" />
            </div>
            <Skeleton className="h-9 w-full sm:max-w-64 sm:justify-self-end" />
          </div>
        ))}
      </div>
    </div>
  </div>
)

export const SettingsPage = () => {
  const { data: configMode } = useConfigMode()
  const { mutate } = useSWRConfig()
  const {
    control,
    register,
    handleSubmit,
    reset,
    watch,
    formState: { isDirty, isSubmitting },
  } = useForm<RuntimeFormState>({
    defaultValues: DEFAULT_RUNTIME_FORM,
  })

  const {
    data: profiles,
    error: profilesError,
    isLoading: profilesLoading,
  } = useSWR<ProfileSummary[]>(
    'profiles',
    () => ProfileService.ListProfiles(),
  )

  const runtimeProfileName = profiles?.find(profile => profile.isDefault)?.name
    ?? profiles?.[0]?.name
    ?? null

  const {
    data: runtimeProfile,
    error: runtimeError,
    isLoading: runtimeLoading,
    mutate: mutateRuntimeProfile,
  } = useSWR<ProfileDetail | null>(
    runtimeProfileName ? ['profile', runtimeProfileName] : null,
    () => (runtimeProfileName ? ProfileService.GetProfile(runtimeProfileName) : null),
  )

  useEffect(() => {
    if (runtimeProfile?.runtime) {
      reset(toRuntimeFormState(runtimeProfile.runtime), { keepDirty: false })
      return
    }
    if (runtimeProfile === null) {
      reset(DEFAULT_RUNTIME_FORM, { keepDirty: false })
    }
  }, [runtimeProfile, reset])

  const canEdit = Boolean(configMode?.isWritable)
  const hasRuntimeProfile = Boolean(runtimeProfile?.runtime)
  const isSaving = isSubmitting
  const showRuntimeSkeleton = profilesLoading || runtimeLoading
  const statusLabel = showRuntimeSkeleton
    ? 'Loading runtime settings'
    : !hasRuntimeProfile
      ? 'Runtime data unavailable'
      : isDirty
        ? 'Unsaved changes'
        : 'All changes saved'
  const saveDisabledReason = showRuntimeSkeleton
    ? 'Runtime settings are still loading'
    : !hasRuntimeProfile
      ? 'Runtime settings are unavailable'
      : !canEdit
        ? 'Configuration is read-only'
        : !isDirty
          ? 'No changes to save'
          : undefined
  const exposeTools = watch('exposeTools', DEFAULT_RUNTIME_FORM.exposeTools)

  const handleSave = handleSubmit(async (values) => {
    if (!runtimeProfileName || !canEdit) {
      return
    }
    try {
      await ConfigService.UpdateRuntimeConfig(values)

      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        toastManager.add({
          type: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }

      await Promise.all([
        mutateRuntimeProfile(),
        mutate('profiles'),
      ])
      reset(values, { keepDirty: false })

      toastManager.add({
        type: 'success',
        title: 'Runtime updated',
        description: 'Changes applied successfully.',
      })
    } catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Update failed',
        description: err instanceof Error ? err.message : 'Update failed',
      })
    }
  })

  const hasProfiles = (profiles?.length ?? 0) > 0

  return (
    <div className="flex flex-1 flex-col overflow-auto">
      <SettingsHeader />
      <Separator className="my-6" />
      <m.div
        className="flex-1 space-y-6 px-6 pb-8"
        initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
        animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
        transition={Spring.smooth(0.4)}
      >
        {profilesError && (
          <Alert variant="error">
            <AlertCircleIcon />
            <AlertTitle>Failed to load profiles</AlertTitle>
            <AlertDescription>
              {profilesError instanceof Error
                ? profilesError.message
                : 'Unable to load configuration profiles.'}
            </AlertDescription>
          </Alert>
        )}

        {!hasProfiles && !profilesLoading && !profilesError && (
          <Empty className="min-h-[280px]">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <SettingsIcon className="size-4" />
              </EmptyMedia>
              <EmptyTitle>No profiles available</EmptyTitle>
              <EmptyDescription>
                Create a profile before editing runtime settings.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}

        {hasProfiles && (
          <Form
            className="gap-0"
            onSubmit={handleSave}
          >
            <Card className='p-1'>
              <CardHeader className='pt-3'>
                <CardTitle className="flex items-center gap-2">
                  Runtime
                  {runtimeProfileName && (
                    <Badge variant="secondary" size="sm">
                      {runtimeProfileName}
                    </Badge>
                  )}
                  {!canEdit && (
                    <Badge variant="warning" size="sm">
                      Read-only
                    </Badge>
                  )}
                </CardTitle>
                <CardDescription>
                  Adjust timeouts, retries, and global defaults for runtime behavior.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                {!canEdit && (
                  <Alert variant="warning">
                    <ShieldAlertIcon />
                    <AlertTitle>Configuration is read-only</AlertTitle>
                    <AlertDescription>
                      Update permissions to enable runtime edits.
                    </AlertDescription>
                  </Alert>
                )}

                {runtimeError && (
                  <Alert variant="error">
                    <AlertCircleIcon />
                    <AlertTitle>Failed to load runtime settings</AlertTitle>
                    <AlertDescription>
                      {runtimeError instanceof Error
                        ? runtimeError.message
                        : 'Unable to load runtime configuration.'}
                    </AlertDescription>
                  </Alert>
                )}

                {!runtimeLoading && runtimeProfile === null && (
                  <Alert variant="warning">
                    <AlertCircleIcon />
                    <AlertTitle>Runtime settings unavailable</AlertTitle>
                    <AlertDescription>
                      The default profile could not be loaded.
                    </AlertDescription>
                  </Alert>
                )}

                {showRuntimeSkeleton && <RuntimeSkeleton />}

                {hasRuntimeProfile && !showRuntimeSkeleton && (
                  <div className="space-y-6">
                    <div className="space-y-3">
                      <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        Core
                      </div>
                      <div className="divide-y divide-border">
                        <RuntimeFieldRow
                          label="Bootstrap Mode"
                          description="How metadata is collected during startup"
                          htmlFor="runtime-bootstrap-mode"
                        >
                          <Controller
                            control={control}
                            name="bootstrapMode"
                            render={({ field }) => (
                              <Select
                                value={field.value}
                                onValueChange={field.onChange}
                                disabled={!canEdit || isSaving}
                              >
                                <SelectTrigger id="runtime-bootstrap-mode">
                                  <SelectValue>
                                    {value =>
                                      value
                                        ? BOOTSTRAP_MODE_LABELS[String(value)]
                                        ?? String(value)
                                        : 'Select mode'}
                                  </SelectValue>
                                </SelectTrigger>
                                <SelectContent>
                                  {BOOTSTRAP_MODE_OPTIONS.map(option => (
                                    <SelectItem key={option.value} value={option.value}>
                                      {option.label}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            )}
                          />
                        </RuntimeFieldRow>
                        <RuntimeFieldRow
                          label="Default Activation Mode"
                          description="Applied when a server does not specify activationMode"
                          htmlFor="runtime-default-activation-mode"
                        >
                          <Controller
                            control={control}
                            name="defaultActivationMode"
                            render={({ field }) => (
                              <Select
                                value={field.value}
                                onValueChange={field.onChange}
                                disabled={!canEdit || isSaving}
                              >
                                <SelectTrigger id="runtime-default-activation-mode">
                                  <SelectValue>
                                    {value =>
                                      value
                                        ? ACTIVATION_MODE_LABELS[String(value)]
                                        ?? String(value)
                                        : 'Select mode'}
                                  </SelectValue>
                                </SelectTrigger>
                                <SelectContent>
                                  {ACTIVATION_MODE_OPTIONS.map(option => (
                                    <SelectItem key={option.value} value={option.value}>
                                      {option.label}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            )}
                          />
                        </RuntimeFieldRow>
                        <RuntimeNumberRow
                          id="runtime-route-timeout"
                          label="Route Timeout"
                          description="Maximum time to wait for routing requests"
                          unit="seconds"
                          disabled={!canEdit || isSaving}
                          inputProps={register('routeTimeoutSeconds', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeNumberRow
                          id="runtime-ping-interval"
                          label="Ping Interval"
                          description="Interval for server health checks (0 to disable)"
                          unit="seconds"
                          disabled={!canEdit || isSaving}
                          inputProps={register('pingIntervalSeconds', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeNumberRow
                          id="runtime-tool-refresh"
                          label="Tool Refresh Interval"
                          description="How often to refresh tool lists from servers"
                          unit="seconds"
                          disabled={!canEdit || isSaving}
                          inputProps={register('toolRefreshSeconds', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                      </div>
                    </div>

                    <div className="space-y-3">
                      <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        Advanced
                      </div>
                      <div className="divide-y divide-border">
                        <RuntimeNumberRow
                          id="runtime-bootstrap-concurrency"
                          label="Bootstrap Concurrency"
                          description="Number of servers to initialize in parallel"
                          unit="workers"
                          disabled={!canEdit || isSaving}
                          inputProps={register('bootstrapConcurrency', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeNumberRow
                          id="runtime-bootstrap-timeout"
                          label="Bootstrap Timeout"
                          description="Maximum time for server initialization"
                          unit="seconds"
                          disabled={!canEdit || isSaving}
                          inputProps={register('bootstrapTimeoutSeconds', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeNumberRow
                          id="runtime-tool-refresh-concurrency"
                          label="Tool Refresh Concurrency"
                          description="Parallel tool refresh operations limit"
                          unit="workers"
                          disabled={!canEdit || isSaving}
                          inputProps={register('toolRefreshConcurrency', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeNumberRow
                          id="runtime-caller-check"
                          label="Caller Check Interval"
                          description="How often to check for inactive callers"
                          unit="seconds"
                          disabled={!canEdit || isSaving}
                          inputProps={register('callerCheckSeconds', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeNumberRow
                          id="runtime-caller-inactive"
                          label="Caller Inactive Threshold"
                          description="Time before marking caller as inactive"
                          unit="seconds"
                          disabled={!canEdit || isSaving}
                          inputProps={register('callerInactiveSeconds', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeNumberRow
                          id="runtime-init-retry-base"
                          label="Init Retry Base Delay"
                          description="Initial delay for server initialization retry"
                          unit="seconds"
                          disabled={!canEdit || isSaving}
                          inputProps={register('serverInitRetryBaseSeconds', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeNumberRow
                          id="runtime-init-retry-max"
                          label="Init Retry Max Delay"
                          description="Maximum delay for server initialization retry"
                          unit="seconds"
                          disabled={!canEdit || isSaving}
                          inputProps={register('serverInitRetryMaxSeconds', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeNumberRow
                          id="runtime-init-max-retries"
                          label="Init Max Retries"
                          description="Maximum retry attempts for server initialization"
                          unit="retries"
                          disabled={!canEdit || isSaving}
                          inputProps={register('serverInitMaxRetries', {
                            valueAsNumber: true,
                            setValueAs: normalizeNumber,
                          })}
                        />
                        <RuntimeFieldRow
                          label="Expose Tools"
                          description="Expose tools to external callers"
                          htmlFor="runtime-expose-tools"
                        >
                          <div className="flex items-center justify-between gap-3 rounded-lg border bg-muted/20 px-3 py-2">
                            <Controller
                              control={control}
                              name="exposeTools"
                              render={({ field }) => (
                                <Switch
                                  id="runtime-expose-tools"
                                  checked={field.value}
                                  onCheckedChange={checked => field.onChange(checked === true)}
                                  disabled={!canEdit || isSaving}
                                />
                              )}
                            />
                            <Badge
                              variant={exposeTools ? 'success' : 'secondary'}
                              size="sm"
                            >
                              {exposeTools ? 'Enabled' : 'Disabled'}
                            </Badge>
                          </div>
                        </RuntimeFieldRow>
                        <RuntimeFieldRow
                          label="Tool Namespace Strategy"
                          description="How to namespace tool names from different servers"
                          htmlFor="runtime-tool-namespace"
                        >
                          <Controller
                            control={control}
                            name="toolNamespaceStrategy"
                            render={({ field }) => (
                              <Select
                                value={field.value}
                                onValueChange={field.onChange}
                                disabled={!canEdit || isSaving}
                              >
                                <SelectTrigger id="runtime-tool-namespace">
                                  <SelectValue>
                                    {value =>
                                      value
                                        ? NAMESPACE_STRATEGY_LABELS[String(value)]
                                        ?? String(value)
                                        : 'Select strategy'}
                                  </SelectValue>
                                </SelectTrigger>
                                <SelectContent>
                                  {NAMESPACE_STRATEGY_OPTIONS.map(option => (
                                    <SelectItem key={option.value} value={option.value}>
                                      {option.label}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            )}
                          />
                        </RuntimeFieldRow>
                      </div>
                    </div>
                  </div>
                )}
              </CardContent>
              <CardFooter className="border-t">
                <div className="flex w-full flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="text-xs text-muted-foreground">
                    {statusLabel}
                  </div>
                  <Button
                    type="submit"
                    size="sm"
                    disabled={!hasRuntimeProfile || !canEdit || !isDirty || isSaving}
                    title={saveDisabledReason}
                  >
                    {isSaving ? (
                      <Spinner className="size-4" />
                    ) : (
                      <SaveIcon className="size-4" />
                    )}
                    {isSaving ? 'Saving...' : 'Save changes'}
                  </Button>
                </div>
              </CardFooter>
            </Card>
          </Form>
        )}
      </m.div>
    </div>
  )
}
