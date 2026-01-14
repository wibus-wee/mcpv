import type { ProfileDetail, ProfileSummary } from '@bindings/mcpd/internal/ui'
import { ProfileService } from '@bindings/mcpd/internal/ui'
import useSWR from 'swr'

import { defaultRpcAddress } from '@/lib/mcpdmcp'

const pickProfileName = (profiles: ProfileSummary[] | undefined) => {
  if (!profiles || profiles.length === 0) return null
  const preferred = profiles.find(profile => profile.isDefault)?.name
  return preferred || profiles[0]?.name || null
}

const extractRpcAddress = (detail: ProfileDetail | null) => {
  return detail?.runtime?.rpc?.listenAddress || defaultRpcAddress
}

export function useRpcAddress() {
  const swr = useSWR<string>(
    'rpc-address',
    async () => {
      const profiles = await ProfileService.ListProfiles()
      const profileName = pickProfileName(profiles)
      if (!profileName) return defaultRpcAddress
      const detail = await ProfileService.GetProfile(profileName)
      return extractRpcAddress(detail)
    },
    { revalidateOnFocus: false },
  )

  return {
    ...swr,
    rpcAddress: swr.data || defaultRpcAddress,
  }
}
