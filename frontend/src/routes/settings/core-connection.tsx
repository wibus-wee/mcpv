// Input: TanStack Router, core connection settings components
// Output: Core connection settings page
// Position: /settings/core-connection route

import { createFileRoute } from '@tanstack/react-router'

import { CoreConnectionSettingsCard } from '@/modules/settings/components/core-connection-settings-card'
import { useCoreConnectionSettings } from '@/modules/settings/hooks/use-core-connection-settings'

export const Route = createFileRoute('/settings/core-connection')({
  component: CoreConnectionSettingsPage,
})

function CoreConnectionSettingsPage() {
  const coreConnection = useCoreConnectionSettings({ canEdit: true })

  return (
    <div className="p-3">
      <CoreConnectionSettingsCard
        canEdit
        form={coreConnection.form}
        statusLabel={coreConnection.statusLabel}
        saveDisabledReason={coreConnection.saveDisabledReason}
        coreConnectionLoading={coreConnection.coreConnectionLoading}
        coreConnectionError={coreConnection.coreConnectionError}
        validationError={coreConnection.validationError}
        mode={coreConnection.mode}
        authMode={coreConnection.authMode}
        tlsEnabled={coreConnection.tlsEnabled}
        onSubmit={coreConnection.handleSave}
      />
    </div>
  )
}
