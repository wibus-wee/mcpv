// Input: None
// Output: Core connection settings form types, defaults, and mappers
// Position: Settings module config helpers for core connection UI

export type CoreConnectionMode = 'local' | 'remote'
export type CoreConnectionAuthMode = 'disabled' | 'token' | 'mtls'

export type CoreConnectionFormState = {
  mode: CoreConnectionMode
  rpcAddress: string
  caller: string
  authMode: CoreConnectionAuthMode
  authToken: string
  authTokenEnv: string
  tlsEnabled: boolean
  tlsCAFile: string
  tlsCertFile: string
  tlsKeyFile: string
  maxRecvMsgSize: number
  maxSendMsgSize: number
  keepaliveTimeSeconds: number
  keepaliveTimeoutSeconds: number
}

export const CORE_CONNECTION_SECTION_KEY = 'core-connection'

export const CORE_CONNECTION_MODE_OPTIONS = [
  { value: 'local', label: 'Local' },
  { value: 'remote', label: 'Remote' },
] as const

export const CORE_CONNECTION_AUTH_OPTIONS = [
  { value: 'disabled', label: 'Disabled' },
  { value: 'token', label: 'Token' },
  { value: 'mtls', label: 'mTLS' },
] as const

const defaultRPCAddress = 'unix:///tmp/mcpv.sock'
const defaultCaller = 'mcpv-ui-internal'
const defaultMaxRecv = 16 * 1024 * 1024
const defaultMaxSend = 16 * 1024 * 1024
const defaultKeepaliveTime = 30
const defaultKeepaliveTimeout = 10

export const DEFAULT_CORE_CONNECTION_FORM: CoreConnectionFormState = {
  mode: 'local',
  rpcAddress: defaultRPCAddress,
  caller: defaultCaller,
  authMode: 'disabled',
  authToken: '',
  authTokenEnv: '',
  tlsEnabled: false,
  tlsCAFile: '',
  tlsCertFile: '',
  tlsKeyFile: '',
  maxRecvMsgSize: defaultMaxRecv,
  maxSendMsgSize: defaultMaxSend,
  keepaliveTimeSeconds: defaultKeepaliveTime,
  keepaliveTimeoutSeconds: defaultKeepaliveTimeout,
}

type CoreConnectionSectionPayload = {
  mode?: string
  rpcAddress?: string
  caller?: string
  maxRecvMsgSize?: number
  maxSendMsgSize?: number
  keepaliveTimeSeconds?: number
  keepaliveTimeoutSeconds?: number
  tls?: {
    enabled?: boolean
    caFile?: string
    certFile?: string
    keyFile?: string
  }
  auth?: {
    enabled?: boolean
    mode?: string
    token?: string
    tokenEnv?: string
  }
}

export function toCoreConnectionFormState(section: unknown): CoreConnectionFormState {
  const payload = resolveSectionObject(section)
  if (!payload) return DEFAULT_CORE_CONNECTION_FORM

  const mode = coerceMode(payload.mode)
  const rpcAddress = toStringOrDefault(payload.rpcAddress, defaultRPCAddress)
  const caller = toStringOrDefault(payload.caller, defaultCaller)

  const tls = payload.tls ?? {}
  const tlsEnabled = Boolean(tls.enabled)
    || Boolean(toStringOrDefault(tls.caFile, ''))
    || Boolean(toStringOrDefault(tls.certFile, ''))
    || Boolean(toStringOrDefault(tls.keyFile, ''))

  const auth = payload.auth ?? {}
  const authMode = coerceAuthMode(auth.enabled, auth.mode, auth.token, auth.tokenEnv)

  return {
    mode,
    rpcAddress,
    caller,
    authMode,
    authToken: toStringOrDefault(auth.token, ''),
    authTokenEnv: toStringOrDefault(auth.tokenEnv, ''),
    tlsEnabled,
    tlsCAFile: toStringOrDefault(tls.caFile, ''),
    tlsCertFile: toStringOrDefault(tls.certFile, ''),
    tlsKeyFile: toStringOrDefault(tls.keyFile, ''),
    maxRecvMsgSize: toNumberOrDefault(payload.maxRecvMsgSize, defaultMaxRecv),
    maxSendMsgSize: toNumberOrDefault(payload.maxSendMsgSize, defaultMaxSend),
    keepaliveTimeSeconds: toNumberOrDefault(payload.keepaliveTimeSeconds, defaultKeepaliveTime),
    keepaliveTimeoutSeconds: toNumberOrDefault(payload.keepaliveTimeoutSeconds, defaultKeepaliveTimeout),
  }
}

export function buildCoreConnectionPayload(values: CoreConnectionFormState): CoreConnectionSectionPayload {
  const authMode = values.authMode
  const authEnabled = authMode !== 'disabled'
  const authToken = authEnabled && authMode === 'token' ? values.authToken.trim() : ''
  const authTokenEnv = authEnabled && authMode === 'token' ? values.authTokenEnv.trim() : ''
  const authModeValue = authMode === 'mtls' ? 'mtls' : 'token'

  return {
    mode: values.mode,
    rpcAddress: values.rpcAddress.trim() || defaultRPCAddress,
    caller: values.caller.trim() || defaultCaller,
    maxRecvMsgSize: values.maxRecvMsgSize,
    maxSendMsgSize: values.maxSendMsgSize,
    keepaliveTimeSeconds: values.keepaliveTimeSeconds,
    keepaliveTimeoutSeconds: values.keepaliveTimeoutSeconds,
    tls: {
      enabled: Boolean(values.tlsEnabled),
      caFile: values.tlsCAFile.trim(),
      certFile: values.tlsCertFile.trim(),
      keyFile: values.tlsKeyFile.trim(),
    },
    auth: {
      enabled: authEnabled,
      mode: authModeValue,
      token: authToken,
      tokenEnv: authTokenEnv,
    },
  }
}

function resolveSectionObject(section: unknown): CoreConnectionSectionPayload | null {
  if (!section) return null
  if (typeof section === 'string') {
    try {
      const parsed = JSON.parse(section) as unknown
      if (parsed && typeof parsed === 'object') {
        return parsed as CoreConnectionSectionPayload
      }
    }
    catch {
      return null
    }
  }
  if (typeof section === 'object') {
    return section as CoreConnectionSectionPayload
  }
  return null
}

function toStringOrDefault(value: unknown, fallback: string) {
  if (typeof value === 'string' && value.trim() !== '') return value
  return fallback
}

function toNumberOrDefault(value: unknown, fallback: number) {
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) return value
  return fallback
}

function coerceMode(value?: string): CoreConnectionMode {
  if (value === 'remote') return 'remote'
  return 'local'
}

function coerceAuthMode(enabled?: boolean, mode?: string, token?: string, tokenEnv?: string): CoreConnectionAuthMode {
  if (!enabled && !token && !tokenEnv) return 'disabled'
  if (mode === 'mtls') return 'mtls'
  return 'token'
}
