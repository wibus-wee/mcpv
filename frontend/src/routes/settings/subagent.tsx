// Input: TanStack Router, subagent settings components
// Output: SubAgent settings page
// Position: /settings/subagent route

import { createFileRoute } from '@tanstack/react-router'

import { useConfigMode } from '@/modules/config/hooks'
import { SubAgentSettingsCard } from '@/modules/settings/components/subagent-settings-card'
import { useSubAgentSettings } from '@/modules/settings/hooks/use-subagent-settings'

export const Route = createFileRoute('/settings/subagent')({
  component: SubAgentSettingsPage,
})

function SubAgentSettingsPage() {
  const { data: configMode } = useConfigMode()
  const canEdit = Boolean(configMode?.isWritable)
  const subAgent = useSubAgentSettings({ canEdit })

  return (
    <div className="p-6">
      <SubAgentSettingsCard
        canEdit={canEdit}
        form={subAgent.form}
        apiKeyInput={subAgent.apiKeyInput}
        onApiKeyChange={subAgent.setApiKeyInput}
        modelInputValue={subAgent.modelInputValue}
        onModelInputChange={subAgent.setModelValue}
        onModelValueChange={subAgent.setModelValue}
        modelLabelMap={subAgent.modelLabelMap}
        modelOptionIDs={subAgent.modelOptionIDs}
        modelFetchState={subAgent.modelFetchState}
        modelFetchError={subAgent.modelFetchError}
        modelFetchLabel={subAgent.modelFetchLabel}
        onFetchModels={subAgent.fetchModels}
        availableTags={subAgent.availableTags}
        statusLabel={subAgent.statusLabel}
        saveDisabledReason={subAgent.saveDisabledReason}
        hasSubAgentChanges={subAgent.hasSubAgentChanges}
        subAgentError={subAgent.subAgentError}
        onSubmit={subAgent.handleSave}
      />
    </div>
  )
}
