import type { InstanceStatus, ServerDetail } from '@bindings/mcpv/internal/ui/types'

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatRelativeTime } from '@/lib/time'
import { formatStartReason, formatStartTriggerLines, resolvePolicyLabel, resolveStartCause } from '@/modules/shared/server-start'

interface ServerInstancesTableProps {
  instances: InstanceStatus[]
  specDetail?: ServerDetail
}

export function ServerInstancesTable({ instances, specDetail }: ServerInstancesTableProps) {
  return (
    <div className="overflow-x-auto scrollbar-none">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Instance</TableHead>
            <TableHead>Cause</TableHead>
            <TableHead>Trigger</TableHead>
            <TableHead>Policy</TableHead>
            <TableHead>Time</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {instances.map((instance) => {
            const resolvedCause = resolveStartCause(
              instance.lastStartCause,
              specDetail?.activationMode,
              specDetail?.minReady,
            )
            const triggerLines = formatStartTriggerLines(resolvedCause)
            const relativeTime = formatRelativeTime(
              resolvedCause?.timestamp,
            )
            const policyLabel = resolvePolicyLabel(
              resolvedCause,
              specDetail?.activationMode,
              specDetail?.minReady,
            )
            return (
              <TableRow key={instance.id}>
                <TableCell className="font-mono text-xs">
                  {instance.id}
                </TableCell>
                <TableCell className="text-xs">
                  {formatStartReason(
                    resolvedCause,
                  )}
                </TableCell>
                <TableCell className="text-xs">
                  {triggerLines.length > 0 ? (
                    <div className="space-y-1">
                      {triggerLines.map(line => (
                        <p
                          key={line}
                          className="text-xs text-muted-foreground"
                        >
                          {line}
                        </p>
                      ))}
                    </div>
                  ) : (
                    <span className="text-xs text-muted-foreground">
                      —
                    </span>
                  )}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {policyLabel}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground tabular-nums">
                  {relativeTime}
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
