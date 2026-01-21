// Input: runtime form state, runtime profile metadata
// Output: Runtime settings card
// Position: Settings page runtime section

import type { ProfileDetail } from '@bindings/mcpd/internal/ui'
import { AlertCircleIcon, SaveIcon, ShieldAlertIcon } from 'lucide-react'
import type * as React from 'react'
import { Controller, type UseFormReturn } from 'react-hook-form'

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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import {
  ACTIVATION_MODE_LABELS,
  ACTIVATION_MODE_OPTIONS,
  BOOTSTRAP_MODE_LABELS,
  BOOTSTRAP_MODE_OPTIONS,
  DEFAULT_RUNTIME_FORM,
  NAMESPACE_STRATEGY_LABELS,
  NAMESPACE_STRATEGY_OPTIONS,
  type RuntimeFormState,
} from '../lib/runtime-config'
import { normalizeNumber } from '../lib/form-utils'
import { RuntimeFieldRow, RuntimeNumberRow, RuntimeSkeleton } from './runtime-field-rows'

interface RuntimeSettingsCardProps {
  runtimeProfileName: string | null
  runtimeProfile: ProfileDetail | null | undefined
  runtimeError: unknown
  canEdit: boolean
  form: UseFormReturn<RuntimeFormState>
  statusLabel: string
  saveDisabledReason?: string
  showRuntimeSkeleton: boolean
  hasRuntimeProfile: boolean
  onSubmit: (event?: React.BaseSyntheticEvent) => void
}

export const RuntimeSettingsCard = ({
  runtimeProfileName,
  runtimeProfile,
  runtimeError,
  canEdit,
  form,
  statusLabel,
  saveDisabledReason,
  showRuntimeSkeleton,
  hasRuntimeProfile,
  onSubmit,
}: RuntimeSettingsCardProps) => {
  const { control, register, watch, formState } = form
  const isSaving = formState.isSubmitting
  const isDirty = formState.isDirty
  const exposeTools = watch('exposeTools', DEFAULT_RUNTIME_FORM.exposeTools)

  return (
    <form
      className="flex w-full flex-col gap-0"
      onSubmit={onSubmit}
    >
      <Card className="p-1">
        <CardHeader className="pt-3">
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

          {!showRuntimeSkeleton && runtimeProfile === null && (
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
    </form>
  )
}
