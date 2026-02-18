// Input: FieldHelpContent type
// Output: Core connection settings field help content map
// Position: Guidance copy for core connection settings UI

import type { FieldHelpContent } from '@/components/common/field-help'

const DOCS_BASE_URL = 'https://github.com/wibus-io/mcpd/blob/main/docs'
const CONNECTION_DOCS_URL = `${DOCS_BASE_URL}/UX_REDUCTION.md`
const SECURITY_DOCS_URL = `${DOCS_BASE_URL}/MCP.md`

export const CORE_CONNECTION_FIELD_HELP: Record<string, FieldHelpContent> = {
  mode: {
    id: 'core-connection-mode',
    title: 'Connection mode',
    summary: 'Switch between local embedded Core and a remote RPC endpoint.',
    details: 'Remote mode disables local Core controls and uses the settings below.',
    docUrl: CONNECTION_DOCS_URL,
  },
  rpcAddress: {
    id: 'core-connection-rpc-address',
    title: 'RPC address',
    summary: 'The gRPC address of the remote Core.',
    tips: ['unix:///tmp/mcpv.sock', '127.0.0.1:7233'],
    docUrl: SECURITY_DOCS_URL,
  },
  authMode: {
    id: 'core-connection-auth-mode',
    title: 'Authentication',
    summary: 'Configure the auth mode expected by the remote Core.',
    details: 'Token auth is common for remote deployments; mTLS requires TLS.',
    docUrl: SECURITY_DOCS_URL,
  },
  authToken: {
    id: 'core-connection-auth-token',
    title: 'Auth token',
    summary: 'Bearer token sent on every RPC request.',
    details: 'Matches rpc.auth.token in the remote Core config.',
    docUrl: SECURITY_DOCS_URL,
  },
  authTokenEnv: {
    id: 'core-connection-auth-token-env',
    title: 'Auth token env',
    summary: 'Environment variable that contains the RPC token.',
    docUrl: SECURITY_DOCS_URL,
  },
  tlsEnabled: {
    id: 'core-connection-tls-enabled',
    title: 'Enable TLS',
    summary: 'Use TLS for gRPC connections to the remote Core.',
    details: 'Required for mTLS authentication.',
    docUrl: SECURITY_DOCS_URL,
  },
  tlsCAFile: {
    id: 'core-connection-tls-ca',
    title: 'CA file',
    summary: 'Path to the CA certificate used to verify the remote Core.',
    docUrl: SECURITY_DOCS_URL,
  },
  tlsCertFile: {
    id: 'core-connection-tls-cert',
    title: 'Client cert file',
    summary: 'Client certificate for mTLS authentication.',
    docUrl: SECURITY_DOCS_URL,
  },
  tlsKeyFile: {
    id: 'core-connection-tls-key',
    title: 'Client key file',
    summary: 'Private key matching the client certificate.',
    docUrl: SECURITY_DOCS_URL,
  },
  caller: {
    id: 'core-connection-caller',
    title: 'Caller name',
    summary: 'Client identifier registered with the Core.',
    details: 'Used for visibility and governance.',
    docUrl: SECURITY_DOCS_URL,
  },
  maxRecvMsgSize: {
    id: 'core-connection-max-recv',
    title: 'Max recv size',
    summary: 'Maximum message size the UI accepts from the Core.',
    docUrl: SECURITY_DOCS_URL,
  },
  maxSendMsgSize: {
    id: 'core-connection-max-send',
    title: 'Max send size',
    summary: 'Maximum message size the UI sends to the Core.',
    docUrl: SECURITY_DOCS_URL,
  },
  keepaliveTimeSeconds: {
    id: 'core-connection-keepalive-time',
    title: 'Keepalive time',
    summary: 'Interval for gRPC keepalive pings.',
    docUrl: SECURITY_DOCS_URL,
  },
  keepaliveTimeoutSeconds: {
    id: 'core-connection-keepalive-timeout',
    title: 'Keepalive timeout',
    summary: 'Timeout before keepalive is considered failed.',
    docUrl: SECURITY_DOCS_URL,
  },
}
