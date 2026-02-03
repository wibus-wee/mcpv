// Input: TanStack Router, subagent settings components
// Output: SubAgent settings page
// Position: /settings/subagent route

import { createFileRoute } from '@tanstack/react-router'

import { ExperimentalBanner } from '@/components/common/experimental-banner'
import { useConfigMode } from '@/modules/servers/hooks'
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
    <div className="flex flex-col gap-3 p-3">
      <ExperimentalBanner
        feature="Feature"
        description="SubAgent is currently under active development and the implementation may change."
        inspirationName="Alma by yetone"
        inspirationUrl="https://alma.now/"
      />
      <SubAgentSettingsCard
        canEdit={canEdit}
        form={subAgent.form}
        apiKeyInput={subAgent.apiKeyInput}
        onApiKeyChange={subAgent.setApiKeyInput}
        modelInputValue={subAgent.modelInputValue}
        onModelInputChange={subAgent.setModelInput}
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
