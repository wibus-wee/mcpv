import { SystemService } from '@bindings/mcpd/internal/ui'
import useSWR from 'swr'

const fallbackPath = 'mcpdmcp'

export function useMcpdmcpPath() {
  const swr = useSWR<string>(
    'mcpdmcp-path',
    () => SystemService.ResolveMcpdmcpPath(),
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
