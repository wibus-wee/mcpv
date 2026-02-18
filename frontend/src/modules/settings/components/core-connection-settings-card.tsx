// Input: core connection form state, SettingsCard components, UI primitives
// Output: Core connection settings card UI
// Position: Settings module core connection section

import { ChevronDownIcon, SettingsIcon, ShieldIcon } from 'lucide-react'
import { m } from 'motion/react'
import type * as React from 'react'
import { useState } from 'react'
import type { UseFormReturn } from 'react-hook-form'

import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import type { CoreConnectionFormState } from '../lib/core-connection-config'
import {
  CORE_CONNECTION_AUTH_OPTIONS,
  CORE_CONNECTION_MODE_OPTIONS,
} from '../lib/core-connection-config'
import { CORE_CONNECTION_FIELD_HELP } from '../lib/core-connection-help'
import { SettingsCard } from './settings-card'

interface CoreConnectionSettingsCardProps {
  canEdit: boolean
  form: UseFormReturn<CoreConnectionFormState>
  statusLabel: string
  saveDisabledReason?: string
  coreConnectionLoading: boolean
  coreConnectionError: unknown
  validationError?: string
  mode: CoreConnectionFormState['mode']
  authMode: CoreConnectionFormState['authMode']
  tlsEnabled: boolean
  onSubmit: (event?: React.BaseSyntheticEvent) => void
}

export const CoreConnectionSettingsCard = ({
  canEdit,
  form,
  statusLabel,
  saveDisabledReason,
  coreConnectionLoading,
  coreConnectionError,
  validationError,
  mode,
  authMode,
  tlsEnabled,
  onSubmit,
}: CoreConnectionSettingsCardProps) => {
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const badge = mode === 'remote' ? 'Remote' : 'Local'

  return (
    <SettingsCard form={form} canEdit={canEdit} onSubmit={onSubmit}>
      <SettingsCard.Header
        title="Core Connection"
        description="Switch between the embedded Core and a remote RPC endpoint."
        badge={badge}
      />
      <SettingsCard.Content>
        <SettingsCard.ReadOnlyAlert />
        <SettingsCard.ErrorAlert
          error={coreConnectionError}
          title="Failed to load connection settings"
          fallbackMessage="Unable to load connection settings."
        />

        {coreConnectionLoading ? null : (
          <>
            <SettingsCard.Section title="Mode">
              <SettingsCard.SelectField<CoreConnectionFormState>
                name="mode"
                label="Connection mode"
                description="Remote mode disables local Core controls."
                options={CORE_CONNECTION_MODE_OPTIONS}
                help={CORE_CONNECTION_FIELD_HELP.mode}
              />
            </SettingsCard.Section>

            <SettingsCard.Section title="Remote Endpoint">
              <SettingsCard.TextField<CoreConnectionFormState>
                name="rpcAddress"
                label="RPC address"
                description="Target address for remote Core RPC."
                help={CORE_CONNECTION_FIELD_HELP.rpcAddress}
              />
            </SettingsCard.Section>

            <SettingsCard.Section title="Authentication">
              <SettingsCard.SelectField<CoreConnectionFormState>
                name="authMode"
                label="Auth mode"
                description="Select the auth scheme expected by the remote Core."
                options={CORE_CONNECTION_AUTH_OPTIONS}
                help={CORE_CONNECTION_FIELD_HELP.authMode}
              />
              {authMode === 'token' && (
                <>
                  <SettingsCard.TextField<CoreConnectionFormState>
                    name="authToken"
                    label="Auth token"
                    description="Bearer token sent to the remote Core."
                    type="password"
                    help={CORE_CONNECTION_FIELD_HELP.authToken}
                  />
                  <SettingsCard.TextField<CoreConnectionFormState>
                    name="authTokenEnv"
                    label="Auth token env"
                    description="Environment variable for the token."
                    help={CORE_CONNECTION_FIELD_HELP.authTokenEnv}
                  />
                </>
              )}
            </SettingsCard.Section>

            <SettingsCard.Section title="TLS">
              <SettingsCard.SwitchField<CoreConnectionFormState>
                name="tlsEnabled"
                label="Enable TLS"
                description="Use TLS for gRPC connections."
                help={CORE_CONNECTION_FIELD_HELP.tlsEnabled}
              />
              {tlsEnabled && (
                <>
                  <SettingsCard.TextField<CoreConnectionFormState>
                    name="tlsCAFile"
                    label="CA file"
                    description="CA certificate used to verify the Core."
                    help={CORE_CONNECTION_FIELD_HELP.tlsCAFile}
                  />
                  <SettingsCard.TextField<CoreConnectionFormState>
                    name="tlsCertFile"
                    label="Client cert file"
                    description="Client certificate for mTLS."
                    help={CORE_CONNECTION_FIELD_HELP.tlsCertFile}
                  />
                  <SettingsCard.TextField<CoreConnectionFormState>
                    name="tlsKeyFile"
                    label="Client key file"
                    description="Private key for the client certificate."
                    help={CORE_CONNECTION_FIELD_HELP.tlsKeyFile}
                  />
                </>
              )}
            </SettingsCard.Section>

            <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <CollapsibleTrigger
                type="button"
                className={cn(
                  buttonVariants({ variant: 'ghost', size: 'sm' }),
                  'w-full justify-between px-0 hover:bg-transparent',
                )}
              >
                <div className="flex items-center gap-2">
                  <SettingsIcon className="size-4" />
                  <span className="text-sm font-medium">Advanced Options</span>
                </div>
                <m.div
                  animate={{ rotate: advancedOpen ? 180 : 0 }}
                  transition={{ duration: 0.2 }}
                >
                  <ChevronDownIcon className="size-4 text-muted-foreground" />
                </m.div>
              </CollapsibleTrigger>
              <CollapsibleContent className="space-y-4 pt-4">
                <SettingsCard.Section title="Client Identity">
                  <SettingsCard.TextField<CoreConnectionFormState>
                    name="caller"
                    label="Caller name"
                    description="Client identity registered with the Core."
                    help={CORE_CONNECTION_FIELD_HELP.caller}
                  />
                </SettingsCard.Section>
                <SettingsCard.Section title="RPC Limits">
                  <SettingsCard.NumberField<CoreConnectionFormState>
                    name="maxRecvMsgSize"
                    label="Max receive size"
                    description="Maximum message size accepted (bytes)."
                    unit="bytes"
                    help={CORE_CONNECTION_FIELD_HELP.maxRecvMsgSize}
                  />
                  <SettingsCard.NumberField<CoreConnectionFormState>
                    name="maxSendMsgSize"
                    label="Max send size"
                    description="Maximum message size sent (bytes)."
                    unit="bytes"
                    help={CORE_CONNECTION_FIELD_HELP.maxSendMsgSize}
                  />
                </SettingsCard.Section>
                <SettingsCard.Section title="Keepalive">
                  <SettingsCard.NumberField<CoreConnectionFormState>
                    name="keepaliveTimeSeconds"
                    label="Keepalive time"
                    description="Seconds between keepalive pings."
                    unit="s"
                    help={CORE_CONNECTION_FIELD_HELP.keepaliveTimeSeconds}
                  />
                  <SettingsCard.NumberField<CoreConnectionFormState>
                    name="keepaliveTimeoutSeconds"
                    label="Keepalive timeout"
                    description="Timeout before keepalive fails."
                    unit="s"
                    help={CORE_CONNECTION_FIELD_HELP.keepaliveTimeoutSeconds}
                  />
                </SettingsCard.Section>
                <div className="flex items-center gap-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
                  <ShieldIcon className="size-3" />
                  Advanced settings apply to the RPC client used by the UI.
                </div>
              </CollapsibleContent>
            </Collapsible>
          </>
        )}
      </SettingsCard.Content>
      <SettingsCard.Footer
        statusLabel={statusLabel}
        saveDisabledReason={saveDisabledReason}
        customDisabled={!canEdit || !form.formState.isDirty || form.formState.isSubmitting || Boolean(validationError)}
      />
    </SettingsCard>
  )
}
