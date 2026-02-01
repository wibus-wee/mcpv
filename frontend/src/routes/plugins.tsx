import { createFileRoute } from '@tanstack/react-router'

import { PluginPage } from '@/modules/plugin/plugin-page'

export const Route = createFileRoute('/plugins')({
  component: PluginPage,
})
