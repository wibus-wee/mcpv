// Input: gateway form state, SettingsCard compound components, UI primitives
// Output: Gateway settings card UI
// Position: Settings module gateway section

import { CheckIcon, ClipboardCopyIcon, PlugIcon } from 'lucide-react'
import type * as React from 'react'
import { useMemo, useState } from 'react'
import type { UseFormReturn } from 'react-hook-form'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { Skeleton } from '@/components/ui/skeleton'
import { toastManager } from '@/components/ui/toast'

import type { GatewayFormState } from '../lib/gateway-config'
import { GATEWAY_ACCESS_OPTIONS } from '../lib/gateway-config'
import { GATEWAY_FIELD_HELP } from '../lib/gateway-help'
import { SettingsCard } from './settings-card'

interface GatewaySettingsCardProps {
  canEdit: boolean
  form: UseFormReturn<GatewayFormState>
  statusLabel: string
  saveDisabledReason?: string
  gatewayLoading: boolean
  gatewayError: unknown
  validationError?: string
  endpointPreview: string
  accessMode: GatewayFormState['accessMode']
  enabled: boolean
  onSubmit: (event?: React.BaseSyntheticEvent) => void
}

export const GatewaySettingsCard = ({
  canEdit,
  form,
  statusLabel,
  saveDisabledReason,
  gatewayLoading,
  gatewayError,
  validationError,
  endpointPreview,
  accessMode,
  enabled,
  onSubmit,
}: GatewaySettingsCardProps) => {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(endpointPreview)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
      toastManager.add({
        type: 'success',
        title: 'Endpoint copied',
        description: endpointPreview,
      })
    }
    catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Copy failed',
        description: err instanceof Error ? err.message : 'Unable to copy endpoint',
      })
    }
  }

  const addressDescription = useMemo(() => {
    if (accessMode === 'network') {
      return 'Bind to a public or LAN address to allow remote clients.'
    }
    return 'Use a localhost address to restrict access to this machine.'
  }, [accessMode])

  const tokenDescription = useMemo(() => {
    if (accessMode === 'network') {
      return 'Required for any non-localhost binding.'
    }
    return 'Optional when bound to localhost.'
  }, [accessMode])

  const baseEndpoint = useMemo(() => endpointPreview.replace(/\/+$/, ''), [endpointPreview])
  const routingExamples = useMemo(() => ({
    server: `${baseEndpoint}/server/{name}`,
    tags: `${baseEndpoint}/tags/{tag1,tag2}`,
  }), [baseEndpoint])

  return (
    <SettingsCard form={form} canEdit={canEdit} onSubmit={onSubmit}>
      <SettingsCard.Header
        title="Gateway"
        description="Expose a streamable HTTP gateway managed by the app."
        badge="Managed"
      />
      <SettingsCard.Content>
        <SettingsCard.ReadOnlyAlert />
        <SettingsCard.ErrorAlert
          error={gatewayError}
          title="Failed to load gateway settings"
          fallbackMessage="Unable to load gateway settings."
        />
        {gatewayLoading && <GatewaySkeleton />}

        {!gatewayLoading && (
          <>
            <SettingsCard.Section title="Overview">
              <SettingsCard.Field
                label="Gateway status"
                description="Managed by the app when Core is running."
                htmlFor="gateway-status"
              >
                <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <PlugIcon className="size-4" />
                    {enabled ? 'Enabled' : 'Disabled'}
                  </div>
                  <Badge variant={enabled ? 'success' : 'warning'} size="sm">
                    {enabled ? 'Enabled' : 'Disabled'}
                  </Badge>
                </div>
              </SettingsCard.Field>
              <SettingsCard.Field
                label="Base endpoint"
                description="Use this base URL in your MCP client."
                htmlFor="gateway-endpoint"
              >
                <InputGroup>
                  <InputGroupInput
                    id="gateway-endpoint"
                    readOnly
                    value={endpointPreview}
                    aria-readonly="true"
                  />
                  <InputGroupAddon align="inline-end">
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      onClick={handleCopy}
                      aria-label="Copy endpoint"
                    >
                      {copied ? <CheckIcon /> : <ClipboardCopyIcon />}
                    </Button>
                  </InputGroupAddon>
                </InputGroup>
              </SettingsCard.Field>
              <SettingsCard.Field
                label="Routing examples"
                description="Append a selector to reach a specific server or tag group."
                htmlFor="gateway-routing"
              >
                <div id="gateway-routing" className="space-y-1 text-xs font-mono text-muted-foreground">
                  <div>{routingExamples.server}</div>
                  <div>{routingExamples.tags}</div>
                </div>
              </SettingsCard.Field>
            </SettingsCard.Section>

            <SettingsCard.Section title="Gateway behavior">
              <SettingsCard.SwitchField<GatewayFormState>
                name="enabled"
                label="Enable Gateway"
                description="Start the gateway automatically when Core is running."
                help={GATEWAY_FIELD_HELP.enabled}
              />
            </SettingsCard.Section>

            <SettingsCard.Section title="Access">
              <SettingsCard.SelectField<GatewayFormState>
                name="accessMode"
                label="Access scope"
                description="Local only is recommended for development."
                options={GATEWAY_ACCESS_OPTIONS}
                help={GATEWAY_FIELD_HELP.accessMode}
              />
              <SettingsCard.TextField<GatewayFormState>
                name="httpAddr"
                label="HTTP address"
                description={addressDescription}
                help={GATEWAY_FIELD_HELP.httpAddr}
              />
              <SettingsCard.TextField<GatewayFormState>
                name="httpToken"
                label="Access token"
                description={tokenDescription}
                help={GATEWAY_FIELD_HELP.httpToken}
              />
            </SettingsCard.Section>

            <SettingsCard.Section title="Advanced">
              <SettingsCard.TextField<GatewayFormState>
                name="caller"
                label="Caller name"
                description="Used to resolve profiles and logs."
                help={GATEWAY_FIELD_HELP.caller}
              />
              <SettingsCard.TextField<GatewayFormState>
                name="httpPath"
                label="HTTP path"
                description="Gateway endpoint path."
                help={GATEWAY_FIELD_HELP.httpPath}
              />
              <SettingsCard.TextField<GatewayFormState>
                name="rpc"
                label="RPC address"
                description="Override the Core RPC address."
                help={GATEWAY_FIELD_HELP.rpc}
              />
              <SettingsCard.TextField<GatewayFormState>
                name="binaryPath"
                label="Binary path"
                description="Optional path to the mcpvmcp executable."
                help={GATEWAY_FIELD_HELP.binaryPath}
              />
              <SettingsCard.TextareaField<GatewayFormState>
                name="customArgs"
                label="Custom args"
                description="One argument per line. Overrides the fields above."
                rows={4}
                help={GATEWAY_FIELD_HELP.customArgs}
              />
              <SettingsCard.TextField<GatewayFormState>
                name="healthUrl"
                label="Health check URL"
                description="Optional override for health checks."
                help={GATEWAY_FIELD_HELP.healthUrl}
              />
            </SettingsCard.Section>
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

const GatewaySkeleton = () => (
  <div className="space-y-4">
    <div className="space-y-2">
      <Skeleton className="h-4 w-24" />
      <div className="space-y-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <div
            key={index}
            className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,280px)]"
          >
            <div className="space-y-2">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-3 w-48" />
            </div>
            <Skeleton className="h-9 w-full sm:justify-self-end" />
          </div>
        ))}
      </div>
    </div>
  </div>
)
