// Input: TanStack Router, SettingsPage module
// Output: Settings route component
// Position: /settings route

import { createFileRoute } from '@tanstack/react-router'

import { SettingsPage } from '@/modules/settings/settings-page'

export const Route = createFileRoute('/settings')({
  component: SettingsPage,
})
