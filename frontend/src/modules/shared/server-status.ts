import type { VariantProps } from 'class-variance-authority'

import type { badgeVariants } from '@/components/ui/badge'

export type ServerRuntimeState
  = | 'ready'
    | 'busy'
    | 'starting'
    | 'initializing'
    | 'handshaking'
    | 'draining'
    | 'stopped'
    | 'failed'

type BadgeVariant = VariantProps<typeof badgeVariants>['variant']

export const serverStateVariants: Record<ServerRuntimeState, BadgeVariant> = {
  ready: 'success',
  busy: 'warning',
  starting: 'info',
  initializing: 'info',
  handshaking: 'info',
  draining: 'secondary',
  stopped: 'secondary',
  failed: 'error',
}

export const serverStateLabels: Record<ServerRuntimeState, string> = {
  ready: 'Ready',
  busy: 'Busy',
  starting: 'Starting',
  initializing: 'Initializing',
  handshaking: 'Handshaking',
  draining: 'Draining',
  stopped: 'Stopped',
  failed: 'Failed',
}

export const ACTIVE_INSTANCE_STATES = new Set<ServerRuntimeState>([
  'ready',
  'busy',
  'starting',
  'initializing',
  'handshaking',
  'draining',
])

export function hasActiveInstance(instances?: { state?: string }[]) {
  return (instances ?? []).some(instance => ACTIVE_INSTANCE_STATES.has(instance.state as ServerRuntimeState))
}
