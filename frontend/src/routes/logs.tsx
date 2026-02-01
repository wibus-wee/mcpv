// Input: TanStack Router, LogsViewer from logs module
// Output: Logs route component with full logs viewer
// Position: /logs route - dedicated logs page

import { createFileRoute } from '@tanstack/react-router'

import { LogsViewer } from '@/modules/logs'

export const Route = createFileRoute('/logs')({
  component: LogsPage,
})

function LogsPage() {
  return (
    <LogsViewer />
  )
}
