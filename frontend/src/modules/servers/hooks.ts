// Input: Config hooks, tools hooks, SWR
// Output: Combined data hooks for servers module
// Position: Data layer for unified servers module

export { useActiveClients } from '@/hooks/use-active-clients'
export { useConfigMode, useOpenConfigInEditor, useRuntimeStatus, useServer, useServerInitStatus, useServers } from '@/modules/config/hooks'
export { type ServerGroup, useToolsByServer } from '@/modules/tools/hooks'
