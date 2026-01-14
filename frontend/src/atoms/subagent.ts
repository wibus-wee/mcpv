// Input: jotai for state management
// Output: SubAgent configuration atoms for runtime and per-profile settings
// Position: Global state atoms for SubAgent feature

import { SubAgentService } from '@bindings/mcpd/internal/ui'
import { atom } from 'jotai'
import { atomWithRefresh } from 'jotai/utils'

// Runtime-level SubAgent LLM provider configuration (shared across all profiles)
export interface SubAgentConfig {
  model: string
  provider: string
  apiKeyEnvVar: string
  maxToolsPerRequest: number
  filterPrompt: string
}

// Per-profile SubAgent enabled state
export interface ProfileSubAgentConfig {
  enabled: boolean
}

// Atom to fetch runtime-level SubAgent config
export const subAgentConfigAtom = atomWithRefresh(async () => {
  try {
    const config = await SubAgentService.GetSubAgentConfig()
    return config as SubAgentConfig
  } catch (error) {
    console.error('Failed to fetch SubAgent config:', error)
    return null
  }
})

// Atom to check if SubAgent infrastructure is available
export const isSubAgentAvailableAtom = atom(async () => {
  try {
    return await SubAgentService.IsSubAgentAvailable()
  } catch (error) {
    console.error('Failed to check SubAgent availability:', error)
    return false
  }
})

// Atom to fetch per-profile SubAgent config
export const profileSubAgentConfigAtom = atom(
  async (_get, { profileName }: { profileName: string }) => {
    try {
      const config = await SubAgentService.GetProfileSubAgentConfig(profileName)
      return config as ProfileSubAgentConfig
    } catch (error) {
      console.error(`Failed to fetch SubAgent config for profile ${profileName}:`, error)
      return null
    }
  }
)
