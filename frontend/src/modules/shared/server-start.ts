import type { StartCause } from '@bindings/mcpv/internal/ui/types'

export function formatStartReason(
  cause?: StartCause | null,
  activationMode?: string,
  minReady?: number,
): string {
  if (!cause?.reason) {
    return 'Unknown reason (no info)'
  }
  switch (cause.reason) {
    case 'bootstrap': {
      const policyLabel = resolvePolicyLabel(cause, activationMode, minReady)
      if (policyLabel !== '—') {
        return `Refresh tool metadata · ${policyLabel} keep-alive`
      }
      return 'Refresh tool metadata'
    }
    case 'tool_call':
      return 'Triggered by tool call'
    case 'client_activate':
      return 'Triggered by client activation'
    case 'policy_always_on':
      return 'always-on running'
    case 'policy_min_ready':
      return `minReady=${cause.policy?.minReady ?? 0} minimum ready`
    default:
      return `Unknown reason (${cause.reason})`
  }
}

export function formatStartTriggerLines(cause?: StartCause | null): string[] {
  if (!cause) {
    return []
  }
  const lines: string[] = []
  if (cause.client) {
    lines.push(`client: ${cause.client}`)
  }
  if (cause.toolName) {
    lines.push(`tool: ${cause.toolName}`)
  }
  return lines
}

export function formatPolicyLabel(cause?: StartCause | null): string {
  if (!cause?.policy) {
    return '—'
  }
  if (cause.policy.activationMode === 'always-on') {
    return 'always-on'
  }
  if (cause.policy.minReady > 0) {
    return `minReady=${cause.policy.minReady}`
  }
  return '—'
}

export function resolvePolicyLabel(
  cause: StartCause | null | undefined,
  activationMode?: string,
  minReady?: number,
): string {
  if (cause?.policy) {
    return formatPolicyLabel(cause)
  }
  if (activationMode === 'always-on') {
    return 'always-on'
  }
  if (minReady && minReady > 0) {
    return `minReady=${minReady}`
  }
  return '—'
}

export function resolveStartCause(
  cause: StartCause | null | undefined,
  activationMode?: string,
  minReady?: number,
): StartCause | null {
  if (cause?.reason) {
    return cause
  }
  if (!activationMode && !minReady) {
    return null
  }
  if (activationMode === 'always-on') {
    return {
      reason: 'policy_always_on',
      timestamp: '',
      policy: {
        activationMode,
        minReady: minReady ?? 0,
      },
    }
  }
  if (minReady && minReady > 0) {
    return {
      reason: 'policy_min_ready',
      timestamp: '',
      policy: {
        activationMode: activationMode ?? 'on-demand',
        minReady,
      },
    }
  }
  return null
}
