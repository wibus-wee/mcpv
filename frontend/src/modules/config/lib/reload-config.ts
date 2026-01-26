import { ConfigService } from '@bindings/mcpd/internal/ui'

export type ReloadOutcome
  = | { ok: true }
    | { ok: false, message: string }

const normalizeReloadError = (message: string) => {
  if (message.includes('CORE_NOT_RUNNING')) {
    return 'Core is not running. Start Core to apply changes.'
  }
  if (message.includes('restart required') || message.includes('runtime config changed')) {
    return 'Runtime config changed. Restart required to apply.'
  }
  if (message.includes('INVALID_CONFIG')) {
    return 'Reload rejected. Fix configuration errors and retry.'
  }
  return message
}

const formatReloadError = (err: unknown) => {
  if (err instanceof Error) {
    return normalizeReloadError(err.message)
  }
  if (err && typeof err === 'object') {
    const { message } = err as { message?: unknown }
    if (typeof message === 'string') {
      return normalizeReloadError(message)
    }
  }
  return normalizeReloadError(String(err ?? 'Unknown error'))
}

export const reloadConfig = async (): Promise<ReloadOutcome> => {
  try {
    await ConfigService.ReloadConfig()
    return { ok: true }
  }
  catch (err) {
    return { ok: false, message: formatReloadError(err) }
  }
}
