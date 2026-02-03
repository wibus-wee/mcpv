// Input: runtime form state
// Output: Runtime settings card using compound component pattern
// Position: Settings page runtime section

import { AlertCircleIcon } from 'lucide-react'
import type * as React from 'react'
import type { UseFormReturn } from 'react-hook-form'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'

import type { RuntimeFormState } from '../lib/runtime-config'
import {
  ACTIVATION_MODE_LABELS,
  ACTIVATION_MODE_OPTIONS,
  BOOTSTRAP_MODE_LABELS,
  BOOTSTRAP_MODE_OPTIONS,
  NAMESPACE_STRATEGY_LABELS,
  NAMESPACE_STRATEGY_OPTIONS,
} from '../lib/runtime-config'
import { RUNTIME_FIELD_HELP } from '../lib/runtime-help'
import { SettingsCard } from './settings-card'

interface RuntimeSettingsCardProps {
  canEdit: boolean
  form: UseFormReturn<RuntimeFormState>
  statusLabel: string
  saveDisabledReason?: string
  runtimeLoading: boolean
  runtimeError: unknown
  onSubmit: (event?: React.BaseSyntheticEvent) => void
}

export const RuntimeSettingsCard = ({
  canEdit,
  form,
  statusLabel,
  saveDisabledReason,
  runtimeLoading,
  runtimeError,
  onSubmit,
}: RuntimeSettingsCardProps) => {
  return (
    <SettingsCard form={form} canEdit={canEdit} onSubmit={onSubmit}>
      <SettingsCard.Header
        title="Runtime"
        description="Adjust timeouts, retries, and global defaults for runtime behavior."
      />
      <SettingsCard.Content>
        <SettingsCard.ReadOnlyAlert />
        {Boolean(runtimeError) && (
          <Alert variant="error">
            <AlertCircleIcon />
            <AlertTitle>Failed to load runtime settings</AlertTitle>
            <AlertDescription>
              {runtimeError instanceof Error ? runtimeError.message : 'Unable to load runtime configuration.'}
            </AlertDescription>
          </Alert>
        )}
        {runtimeLoading && <RuntimeSkeleton />}

        {!runtimeLoading && (
          <>
            <SettingsCard.Section title="Core">
              <SettingsCard.SelectField<RuntimeFormState>
                name="bootstrapMode"
                label="Bootstrap Mode"
                description="How metadata is collected during startup"
                options={BOOTSTRAP_MODE_OPTIONS}
                labels={BOOTSTRAP_MODE_LABELS}
                help={RUNTIME_FIELD_HELP.bootstrapMode}
              />
              <SettingsCard.SelectField<RuntimeFormState>
                name="defaultActivationMode"
                label="Default Activation Mode"
                description="Applied when a server does not specify activationMode"
                options={ACTIVATION_MODE_OPTIONS}
                labels={ACTIVATION_MODE_LABELS}
                help={RUNTIME_FIELD_HELP.defaultActivationMode}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="routeTimeoutSeconds"
                label="Route Timeout"
                description="Maximum time to wait for routing requests"
                unit="seconds"
                help={RUNTIME_FIELD_HELP.routeTimeoutSeconds}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="pingIntervalSeconds"
                label="Ping Interval"
                description="Interval for server health checks (0 to disable)"
                unit="seconds"
                help={RUNTIME_FIELD_HELP.pingIntervalSeconds}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="toolRefreshSeconds"
                label="Tool Refresh Interval"
                description="How often to refresh tool lists from servers"
                unit="seconds"
                help={RUNTIME_FIELD_HELP.toolRefreshSeconds}
              />
            </SettingsCard.Section>

            <SettingsCard.Section title="Advanced">
              <SettingsCard.NumberField<RuntimeFormState>
                name="bootstrapConcurrency"
                label="Bootstrap Concurrency"
                description="Number of servers to initialize in parallel"
                unit="workers"
                help={RUNTIME_FIELD_HELP.bootstrapConcurrency}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="bootstrapTimeoutSeconds"
                label="Bootstrap Timeout"
                description="Maximum time for server initialization"
                unit="seconds"
                help={RUNTIME_FIELD_HELP.bootstrapTimeoutSeconds}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="toolRefreshConcurrency"
                label="Tool Refresh Concurrency"
                description="Parallel tool refresh operations limit"
                unit="workers"
                help={RUNTIME_FIELD_HELP.toolRefreshConcurrency}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="clientCheckSeconds"
                label="Client Check Interval"
                description="How often to check for inactive clients"
                unit="seconds"
                help={RUNTIME_FIELD_HELP.clientCheckSeconds}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="clientInactiveSeconds"
                label="Client Inactive Threshold"
                description="Time before marking client as inactive"
                unit="seconds"
                help={RUNTIME_FIELD_HELP.clientInactiveSeconds}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="serverInitRetryBaseSeconds"
                label="Init Retry Base Delay"
                description="Initial delay for server initialization retry"
                unit="seconds"
                help={RUNTIME_FIELD_HELP.serverInitRetryBaseSeconds}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="serverInitRetryMaxSeconds"
                label="Init Retry Max Delay"
                description="Maximum delay for server initialization retry"
                unit="seconds"
                help={RUNTIME_FIELD_HELP.serverInitRetryMaxSeconds}
              />
              <SettingsCard.NumberField<RuntimeFormState>
                name="serverInitMaxRetries"
                label="Init Max Retries"
                description="Maximum retry attempts for server initialization"
                unit="retries"
                help={RUNTIME_FIELD_HELP.serverInitMaxRetries}
              />
              <SettingsCard.SwitchField<RuntimeFormState>
                name="exposeTools"
                label="Expose Tools"
                description="Expose tools to external clients"
                help={RUNTIME_FIELD_HELP.exposeTools}
              />
              <SettingsCard.SelectField<RuntimeFormState>
                name="toolNamespaceStrategy"
                label="Tool Namespace Strategy"
                description="How to namespace tool names from different servers"
                options={NAMESPACE_STRATEGY_OPTIONS}
                labels={NAMESPACE_STRATEGY_LABELS}
                help={RUNTIME_FIELD_HELP.toolNamespaceStrategy}
              />
            </SettingsCard.Section>
          </>
        )}
      </SettingsCard.Content>
      <SettingsCard.Footer
        statusLabel={statusLabel}
        saveDisabledReason={saveDisabledReason}
        customDisabled={!canEdit || !form.formState.isDirty || form.formState.isSubmitting}
      />
    </SettingsCard>
  )
}

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
