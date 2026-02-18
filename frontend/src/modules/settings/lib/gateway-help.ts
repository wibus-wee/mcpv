// Input: FieldHelpContent type
// Output: Gateway settings field help content map
// Position: Guidance copy for gateway settings UI

import type { FieldHelpContent } from '@/components/common/field-help'

const DOCS_BASE_URL = 'https://github.com/wibus-io/mcpd/blob/main/docs'
const GATEWAY_DOCS_URL = `${DOCS_BASE_URL}/UX_REDUCTION.md`
const MCP_DOCS_URL = `${DOCS_BASE_URL}/MCP.md`

export const GATEWAY_FIELD_HELP: Record<string, FieldHelpContent> = {
  enabled: {
    id: 'gateway-enabled',
    title: 'Enable Gateway',
    summary: 'Starts the managed streamable HTTP gateway when Core is running.',
    details: 'Disable this if you want to manage mcpvmcp manually.',
    docUrl: GATEWAY_DOCS_URL,
  },
  accessMode: {
    id: 'gateway-access',
    title: 'Access scope',
    summary: 'Choose whether to bind locally or allow remote clients.',
    details: 'Network access requires a token for non-localhost bindings.',
    docUrl: MCP_DOCS_URL,
  },
  httpAddr: {
    id: 'gateway-http-addr',
    title: 'HTTP address',
    summary: 'Bind address for the streamable HTTP gateway.',
    tips: ['Local only: 127.0.0.1:8090', 'Network: 0.0.0.0:8090'],
    docUrl: MCP_DOCS_URL,
  },
  httpToken: {
    id: 'gateway-http-token',
    title: 'Access token',
    summary: 'Required for non-localhost bindings to prevent unauthorized access.',
    docUrl: MCP_DOCS_URL,
  },
  caller: {
    id: 'gateway-caller',
    title: 'Caller name',
    summary: 'Used to resolve tag visibility and logs for the gateway client.',
    docUrl: GATEWAY_DOCS_URL,
  },
  httpPath: {
    id: 'gateway-http-path',
    title: 'HTTP path',
    summary: 'Base path prefix for gateway routing.',
    tips: ['Clients append /server/{name} or /tags/{tag1,tag2}.'],
    docUrl: MCP_DOCS_URL,
  },
  rpc: {
    id: 'gateway-rpc',
    title: 'RPC address',
    summary: 'Override the Core RPC address used by the gateway.',
    details: 'Leave empty to use the default Core address.',
    docUrl: GATEWAY_DOCS_URL,
  },
  binaryPath: {
    id: 'gateway-binary-path',
    title: 'Binary path',
    summary: 'Optional path to the mcpvmcp executable.',
    docUrl: GATEWAY_DOCS_URL,
  },
  customArgs: {
    id: 'gateway-custom-args',
    title: 'Custom args',
    summary: 'Raw arguments passed to mcpvmcp (one per line).',
    details: 'When provided, these override the fields above.',
    docUrl: GATEWAY_DOCS_URL,
  },
  healthUrl: {
    id: 'gateway-health-url',
    title: 'Health check URL',
    summary: 'Optional override for the gateway health endpoint.',
    docUrl: GATEWAY_DOCS_URL,
  },
}
