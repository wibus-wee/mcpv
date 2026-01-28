// Input: Status string, Badge component
// Output: Status badge components for core status and server runtime status
// Position: Custom component - unified status indicator patterns

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { coreStatusVariants } from '@/modules/shared/core-status'
import type { ServerRuntimeState } from '@/modules/shared/server-status'
import { serverStateLabels, serverStateVariants } from '@/modules/shared/server-status'

/**
 * Core application status types
 */
export type CoreStatus = 'stopped' | 'starting' | 'running' | 'stopping' | 'error'

/**
 * Server runtime state types
 */
export type ServerState = ServerRuntimeState

/**
 * Variants for core application status
 */

/**
 * Props for CoreStatusBadge
 */
interface CoreStatusBadgeProps {
  status: CoreStatus
  size?: 'sm' | 'default' | 'lg'
  className?: string
  /** Show capitalized status text (default: true) */
  showLabel?: boolean
}

/**
 * Badge component for displaying core application status.
 *
 * @example
 * <CoreStatusBadge status="running" />
 * <CoreStatusBadge status="starting" size="sm" />
 */
export function CoreStatusBadge({
  status,
  size = 'default',
  className,
  showLabel = true,
}: CoreStatusBadgeProps) {
  const variant = coreStatusVariants[status]

  const label = status.charAt(0).toUpperCase() + status.slice(1)

  return (
    <Badge
      variant={variant}
      size={size}
      className={cn('font-medium', className)}
    >
      {showLabel ? label : null}
    </Badge>
  )
}

/**
 * Props for ServerStateBadge
 */
interface ServerStateBadgeProps {
  state: string
  size?: 'sm' | 'default' | 'lg'
  className?: string
  /** Custom label (overrides default) */
  label?: string
}

/**
 * Badge component for displaying server runtime state.
 * Handles unknown states gracefully with 'secondary' variant.
 *
 * @example
 * <ServerStateBadge state="ready" />
 * <ServerStateBadge state="busy" size="sm" label="Processing" />
 */
export function ServerStateBadge({
  state,
  size = 'sm',
  className,
  label,
}: ServerStateBadgeProps) {
  const variant = serverStateVariants[state as ServerState] || 'secondary'
  const text = label ?? serverStateLabels[state as ServerState] ?? state

  return (
    <Badge
      variant={variant}
      size={size}
      className={cn('font-medium', className)}
    >
      {text}
    </Badge>
  )
}

/**
 * Props for ServerStateCountBadge
 */
interface ServerStateCountBadgeProps {
  state: string
  count: number
  className?: string
}

/**
 * Badge showing state with count. Returns null if count is 0.
 *
 * @example
 * <ServerStateCountBadge state="ready" count={3} />
 * // Renders: "Ready 3"
 */
export function ServerStateCountBadge({
  state,
  count,
  className,
}: ServerStateCountBadgeProps) {
  if (count <= 0) {
    return null
  }

  const variant = serverStateVariants[state as ServerState] || 'secondary'
  const stateLabel = serverStateLabels[state as ServerState] ?? state

  return (
    <Badge
      variant={variant}
      size="sm"
      className={cn('font-medium tabular-nums', className)}
    >
      {stateLabel} {count}
    </Badge>
  )
}

/**
 * Props for EnabledBadge
 */
interface EnabledBadgeProps {
  enabled: boolean
  size?: 'sm' | 'default' | 'lg'
  className?: string
  /** Labels for enabled/disabled states */
  labels?: { enabled: string, disabled: string }
}

/**
 * Badge for showing enabled/disabled state.
 *
 * @example
 * <EnabledBadge enabled={true} />
 * <EnabledBadge enabled={false} labels={{ enabled: 'Active', disabled: 'Inactive' }} />
 */
export function EnabledBadge({
  enabled,
  size = 'sm',
  className,
  labels = { enabled: 'Enabled', disabled: 'Disabled' },
}: EnabledBadgeProps) {
  return (
    <Badge
      variant={enabled ? 'success' : 'secondary'}
      size={size}
      className={className}
    >
      {enabled ? labels.enabled : labels.disabled}
    </Badge>
  )
}

/**
 * Props for ConnectionBadge
 */
interface ConnectionBadgeProps {
  connected: boolean | null
  size?: 'sm' | 'default' | 'lg'
  className?: string
}

/**
 * Badge for showing connection status (connected/disconnected/waiting).
 *
 * @example
 * <ConnectionBadge connected={true} />
 * <ConnectionBadge connected={false} />
 * <ConnectionBadge connected={null} /> // Shows "Waiting..."
 */
export function ConnectionBadge({
  connected,
  size = 'sm',
  className,
}: ConnectionBadgeProps) {
  if (connected === null) {
    return (
      <Badge variant="warning" size={size} className={className}>
        Waiting...
      </Badge>
    )
  }

  return (
    <Badge
      variant={connected ? 'success' : 'error'}
      size={size}
      className={className}
    >
      {connected ? 'Connected' : 'Disconnected'}
    </Badge>
  )
}
