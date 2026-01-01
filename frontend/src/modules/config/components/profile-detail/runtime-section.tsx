// Input: ProfileDetail type
// Output: RuntimeSection accordion component
// Position: Profile runtime configuration display

import type { ProfileDetail } from '@bindings/mcpd/internal/ui'
import { NetworkIcon, SettingsIcon } from 'lucide-react'

import {
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'

import { DetailRow } from './detail-row'

interface RuntimeSectionProps {
  profile: ProfileDetail
}

/**
 * Displays the runtime configuration section within an accordion.
 */
export function RuntimeSection({ profile }: RuntimeSectionProps) {
  const { runtime } = profile

  return (
    <AccordionItem value="runtime" className="border-none">
      <AccordionTrigger className="py-2 hover:no-underline">
        <div className="flex items-center gap-2">
          <SettingsIcon className="size-3.5 text-muted-foreground" />
          <span className="text-sm font-medium">Runtime Configuration</span>
        </div>
      </AccordionTrigger>
      <AccordionContent className="pb-0">
        <div className="divide-y divide-border/50 pb-3">
          <DetailRow label="Route Timeout" value={`${runtime.routeTimeoutSeconds}s`} mono />
          <DetailRow label="Ping Interval" value={`${runtime.pingIntervalSeconds}s`} mono />
          <DetailRow label="Tool Refresh" value={`${runtime.toolRefreshSeconds}s`} mono />
          <DetailRow label="Caller Check" value={`${runtime.callerCheckSeconds}s`} mono />
          <DetailRow label="Init Retry Base" value={`${runtime.serverInitRetryBaseSeconds}s`} mono />
          <DetailRow label="Init Retry Max" value={`${runtime.serverInitRetryMaxSeconds}s`} mono />
          <DetailRow label="Init Max Retries" value={`${runtime.serverInitMaxRetries}`} mono />
          <DetailRow
            label="Expose Tools"
            value={
              <Badge variant={runtime.exposeTools ? 'success' : 'secondary'} size="sm">
                {runtime.exposeTools ? 'Yes' : 'No'}
              </Badge>
            }
          />
          <DetailRow
            label="Namespace Strategy"
            value={
              <Badge variant="outline" size="sm">
                {runtime.toolNamespaceStrategy || 'prefix'}
              </Badge>
            }
          />
        </div>

        <div className="border-t pt-3 pb-2">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground mb-2">
            <NetworkIcon className="size-3" />
            RPC Configuration
          </div>
          <div className="divide-y divide-border/50">
            <DetailRow
              label="Listen Address"
              value={
                <span className="font-mono text-xs truncate max-w-40 block text-right">
                  {runtime.rpc.listenAddress}
                </span>
              }
            />
            <DetailRow label="Socket Mode" value={runtime.rpc.socketMode || '0660'} mono />
          </div>
        </div>
      </AccordionContent>
    </AccordionItem>
  )
}
