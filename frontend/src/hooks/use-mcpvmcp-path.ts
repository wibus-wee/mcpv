import { SystemService } from '@bindings/mcpv/internal/ui'
import useSWR from 'swr'

const fallbackPath = 'mcpvmcp'

export function useMcpvmcpPath() {
  const swr = useSWR<string>(
    'mcpvmcp-path',
    () => SystemService.ResolvemcpvmcpPath(),
    {
      revalidateOnFocus: false,
    },
  )

  const resolved = swr.data || fallbackPath
  const isFallback = resolved === fallbackPath

  return {
    ...swr,
    path: resolved,
    isFallback,
  }
}
