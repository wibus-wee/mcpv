// Input: runtime/subagent hooks, config mode, UI layout
// Output: Settings page layout
// Position: Settings module page for global runtime settings

import { m } from 'motion/react'

import { Separator } from '@/components/ui/separator'
import { Spring } from '@/lib/spring'
import { useConfigMode } from '@/modules/config/hooks'

import { SettingsHeader } from './components/settings-header'
import { RuntimeSettingsCard } from './components/runtime-settings-card'
import { SubAgentSettingsCard } from './components/subagent-settings-card'
import { useRuntimeSettings } from './hooks/use-runtime-settings'
import { useSubAgentSettings } from './hooks/use-subagent-settings'

export const SettingsPage = () => {
  const { data: configMode } = useConfigMode()
  const canEdit = Boolean(configMode?.isWritable)

  const runtime = useRuntimeSettings({ canEdit })
  const subAgent = useSubAgentSettings({ canEdit })

  return (
    <div className="flex flex-1 flex-col overflow-auto">
      <SettingsHeader />
      <Separator className="my-6" />
      <m.div
        className="flex-1 space-y-6 px-6 pb-8"
        initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
        animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
        transition={Spring.smooth(0.4)}
      >
        <RuntimeSettingsCard
          canEdit={canEdit}
          form={runtime.form}
          statusLabel={runtime.statusLabel}
          saveDisabledReason={runtime.saveDisabledReason}
          runtimeLoading={runtime.runtimeLoading}
          runtimeError={runtime.runtimeError}
          onSubmit={runtime.handleSave}
        />
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
          statusLabel={subAgent.statusLabel}
          saveDisabledReason={subAgent.saveDisabledReason}
          hasSubAgentChanges={subAgent.hasSubAgentChanges}
          subAgentError={subAgent.subAgentError}
          onSubmit={subAgent.handleSave}
        />
      </m.div>
    </div>
  )
}
