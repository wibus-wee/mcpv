import useSWR from 'swr'

import { defaultRpcAddress } from '@/lib/mcpdmcp'

export function useRpcAddress() {
  const swr = useSWR<string>(
    'rpc-address',
    async () => defaultRpcAddress,
    { revalidateOnFocus: false },
  )

  return {
    ...swr,
    rpcAddress: swr.data || defaultRpcAddress,
  }
}
