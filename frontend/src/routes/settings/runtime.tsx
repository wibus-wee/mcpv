// Input: TanStack Router, runtime settings components
// Output: Runtime settings page
// Position: /settings/runtime route

import { createFileRoute } from '@tanstack/react-router'

import { useConfigMode } from '@/modules/config/hooks'
import { RuntimeSettingsCard } from '@/modules/settings/components/runtime-settings-card'
import { useRuntimeSettings } from '@/modules/settings/hooks/use-runtime-settings'

export const Route = createFileRoute('/settings/runtime')({
  component: RuntimeSettingsPage,
})

function RuntimeSettingsPage() {
  const { data: configMode } = useConfigMode()
  const canEdit = Boolean(configMode?.isWritable)
  const runtime = useRuntimeSettings({ canEdit })

  return (
    <div className="p-6">
      <RuntimeSettingsCard
        canEdit={canEdit}
        form={runtime.form}
        statusLabel={runtime.statusLabel}
        saveDisabledReason={runtime.saveDisabledReason}
        runtimeLoading={runtime.runtimeLoading}
        runtimeError={runtime.runtimeError}
        onSubmit={runtime.handleSave}
      />
    </div>
  )
}
