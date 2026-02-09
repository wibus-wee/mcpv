// Input: subagent form state, model list, fetch handlers, analytics
// Output: SubAgent settings card using compound component pattern
// Position: Settings page SubAgent section

import type { SubAgentConfigDetail } from '@bindings/mcpv/internal/ui/types'
import { AlertCircleIcon, AlertTriangleIcon, TagIcon, XIcon } from 'lucide-react'
import type * as React from 'react'
import type { UseFormReturn } from 'react-hook-form'
import { Controller } from 'react-hook-form'

import type { FieldHelpContent } from '@/components/common/field-help'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Combobox,
  ComboboxCollection,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxPopup,
} from '@/components/ui/combobox'
import { InputGroup, InputGroupInput } from '@/components/ui/input-group'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { AnalyticsEvents, track } from '@/lib/analytics'

import { SUBAGENT_PROVIDER_OPTIONS } from '../lib/subagent-config'
import { SUBAGENT_FIELD_HELP } from '../lib/subagent-help'
import type { ModelFetchState } from '../lib/subagent-models'
import { SettingsCard, useSettingsCardContext } from './settings-card'

interface SubAgentSettingsCardProps {
  canEdit: boolean
  form: UseFormReturn<SubAgentConfigDetail>
  apiKeyInput: string
  onApiKeyChange: (value: string) => void
  modelInputValue: string
  onModelInputChange: (value: string) => void
  onModelValueChange: (value: string) => void
  modelLabelMap: Map<string, string>
  modelOptionIDs: string[]
  modelFetchState: ModelFetchState
  modelFetchError: string | null
  modelFetchLabel: string
  onFetchModels: () => void
  availableTags: string[]
  statusLabel: string
  saveDisabledReason?: string
  hasSubAgentChanges: boolean
  subAgentError: unknown
  onSubmit: (event?: React.BaseSyntheticEvent) => void
}

export const SubAgentSettingsCard = ({
  canEdit,
  form,
  apiKeyInput,
  onApiKeyChange,
  modelInputValue,
  onModelInputChange,
  onModelValueChange,
  modelLabelMap,
  modelOptionIDs,
  modelFetchState,
  modelFetchError,
  modelFetchLabel,
  onFetchModels,
  availableTags,
  statusLabel,
  saveDisabledReason,
  hasSubAgentChanges,
  subAgentError,
  onSubmit,
}: SubAgentSettingsCardProps) => {
  return (
    <SettingsCard form={form} canEdit={canEdit} onSubmit={onSubmit}>
      <SettingsCard.Header
        title="SubAgent"
        description="Configure the LLM provider for automatic tool filtering."
      />
      <SettingsCard.Content>
        <SettingsCard.ReadOnlyAlert />
        <SettingsCard.ErrorAlert
          error={subAgentError}
          title="Failed to load SubAgent settings"
          fallbackMessage="Unable to load SubAgent configuration."
        />

        <SettingsCard.Section title="Provider">
          <SettingsCard.SelectField<SubAgentConfigDetail>
            name="provider"
            label="Provider"
            description="Currently only OpenAI is supported"
            options={SUBAGENT_PROVIDER_OPTIONS}
            help={SUBAGENT_FIELD_HELP.provider}
          />
          <ModelField
            modelInputValue={modelInputValue}
            onModelInputChange={onModelInputChange}
            onModelValueChange={onModelValueChange}
            modelLabelMap={modelLabelMap}
            modelOptionIDs={modelOptionIDs}
            modelFetchState={modelFetchState}
            modelFetchLabel={modelFetchLabel}
            onFetchModels={onFetchModels}
            help={SUBAGENT_FIELD_HELP.model}
          />
        </SettingsCard.Section>

        {modelFetchError && (
          <Alert variant="error">
            <AlertCircleIcon />
            <AlertTitle>Failed to fetch models</AlertTitle>
            <AlertDescription>{modelFetchError}</AlertDescription>
          </Alert>
        )}

        <SettingsCard.Section title="Credentials">
          <ApiKeyField
            apiKeyInput={apiKeyInput}
            onApiKeyChange={onApiKeyChange}
            help={SUBAGENT_FIELD_HELP.apiKey}
          />
          <SettingsCard.TextField<SubAgentConfigDetail>
            name="apiKeyEnvVar"
            label="API Key Env Var"
            description="Environment variable name used when no inline key is set"
            placeholder="OPENAI_API_KEY"
            help={SUBAGENT_FIELD_HELP.apiKeyEnvVar}
          />
          <SettingsCard.TextField<SubAgentConfigDetail>
            name="baseURL"
            label="Base URL"
            description="Optional override for the provider endpoint"
            placeholder="https://api.openai.com/v1"
            help={SUBAGENT_FIELD_HELP.baseURL}
          />
        </SettingsCard.Section>

        <SettingsCard.Section title="Behavior">
          <EnabledTagsField
            availableTags={availableTags}
            help={SUBAGENT_FIELD_HELP.enabledTags}
          />
          <SettingsCard.NumberField<SubAgentConfigDetail>
            name="maxToolsPerRequest"
            label="Max Tools Per Request"
            description="Upper bound on tool candidates per query"
            unit="tools"
            help={SUBAGENT_FIELD_HELP.maxToolsPerRequest}
          />
          <SettingsCard.TextareaField<SubAgentConfigDetail>
            name="filterPrompt"
            label="Filter Prompt"
            description="Optional prompt override for tool filtering"
            placeholder="Describe how tools should be filtered..."
            help={SUBAGENT_FIELD_HELP.filterPrompt}
          />
        </SettingsCard.Section>
      </SettingsCard.Content>
      <SettingsCard.Footer
        statusLabel={statusLabel}
        saveDisabledReason={saveDisabledReason}
        customDisabled={!canEdit || !hasSubAgentChanges || form.formState.isSubmitting}
      />
    </SettingsCard>
  )
}

interface ModelFieldProps {
  modelInputValue: string
  onModelInputChange: (value: string) => void
  onModelValueChange: (value: string) => void
  modelLabelMap: Map<string, string>
  modelOptionIDs: string[]
  modelFetchState: ModelFetchState
  modelFetchLabel: string
  onFetchModels: () => void
  help?: FieldHelpContent
}

const ModelField = ({
  modelInputValue,
  onModelInputChange,
  onModelValueChange,
  modelLabelMap,
  modelOptionIDs,
  modelFetchState,
  modelFetchLabel,
  onFetchModels,
  help,
}: ModelFieldProps) => {
  const { canEdit, isSaving } = useSettingsCardContext()
  const selectedModelValue = modelOptionIDs.includes(modelInputValue)
    ? modelInputValue
    : null

  return (
    <SettingsCard.Field
      label="Model"
      description="Fetch available models from the provider and match with models.dev"
      htmlFor="subagent-model"
      help={help}
    >
      <Combobox
        items={modelOptionIDs}
        value={selectedModelValue}
        inputValue={modelInputValue}
        onInputValueChange={onModelInputChange}
        onValueChange={(value) => {
          if (typeof value === 'string') {
            onModelValueChange(value)
          }
        }}
      >
        <ComboboxInput
          id="subagent-model"
          placeholder="gpt-4o"
          disabled={!canEdit || isSaving}
        />
        <ComboboxPopup>
          <ComboboxList>
            <ComboboxEmpty>
              {modelFetchState === 'loading'
                ? 'Loading models...'
                : modelOptionIDs.length === 0
                  ? 'No models loaded'
                  : 'No matches'}
            </ComboboxEmpty>
            <ComboboxCollection>
              {(modelID) => {
                const label = modelLabelMap.get(modelID) ?? modelID
                return (
                  <ComboboxItem key={modelID} value={modelID}>
                    <div className="flex flex-col">
                      <span className="text-sm">{label}</span>
                      {label !== modelID && (
                        <span className="text-xs text-muted-foreground">
                          {modelID}
                        </span>
                      )}
                    </div>
                  </ComboboxItem>
                )
              }}
            </ComboboxCollection>
          </ComboboxList>
        </ComboboxPopup>
      </Combobox>
      <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
        <span>{modelFetchLabel}</span>
        <Button
          type="button"
          size="xs"
          variant="secondary"
          onClick={onFetchModels}
          disabled={!canEdit || isSaving || modelFetchState === 'loading'}
        >
          {modelFetchState === 'loading' ? 'Fetching...' : 'Fetch models'}
        </Button>
      </div>
    </SettingsCard.Field>
  )
}

interface EnabledTagsFieldProps {
  availableTags: string[]
  help?: FieldHelpContent
}

const EnabledTagsField = ({ availableTags, help }: EnabledTagsFieldProps) => {
  const { form, canEdit, isSaving } = useSettingsCardContext<SubAgentConfigDetail>()
  const canInteract = canEdit && !isSaving

  return (
    <SettingsCard.Field
      label="Enabled Tags"
      description="Leave empty to enable SubAgent for all client tags."
      htmlFor="subagent-enabled-tags"
      help={help}
    >
      <Controller
        control={form.control}
        name="enabledTags"
        render={({ field }) => {
          const selectedTags = Array.isArray(field.value) ? field.value : []
          const availableSet = new Set(availableTags)
          const unavailableTags = selectedTags.filter(tag => !availableSet.has(tag))

          const handleTagChange = (values: string[]) => {
            const next = [...new Set([...values, ...unavailableTags])]
            const nextUnavailableTags = next.filter(tag => !availableSet.has(tag))
            field.onChange(next)
            track(AnalyticsEvents.SETTINGS_SUBAGENT_TAGS_CHANGE, {
              selected_count: next.length,
              unavailable_count: nextUnavailableTags.length,
            })
          }

          return (
            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <div className="flex items-center gap-1">
                  <TagIcon className="size-3.5" />
                  {availableTags.length > 0
                    ? `${availableTags.length} tags available`
                    : 'No tags discovered'}
                </div>
                {selectedTags.length > 0 && (
                  <Button
                    type="button"
                    size="xs"
                    variant="ghost"
                    onClick={() => {
                      field.onChange([])
                      track(AnalyticsEvents.SETTINGS_SUBAGENT_TAGS_CHANGE, {
                        selected_count: 0,
                        unavailable_count: 0,
                      })
                    }}
                    disabled={!canInteract}
                  >
                    Clear selection
                  </Button>
                )}
              </div>

              {availableTags.length === 0 ? (
                <div
                  id="subagent-enabled-tags"
                  className="rounded-md border border-dashed bg-muted/20 p-3 text-xs text-muted-foreground"
                >
                  No tags detected yet. Add tags to servers or active clients to make them selectable.
                </div>
              ) : (
                <ScrollArea className="max-h-28 pr-2">
                  <ToggleGroup
                    id="subagent-enabled-tags"
                    multiple
                    value={selectedTags}
                    onValueChange={values => handleTagChange(values as string[])}
                    className="flex flex-wrap gap-1.5"
                  >
                    {availableTags.map(tag => (
                      <ToggleGroupItem
                        key={tag}
                        value={tag}
                        size="sm"
                        variant="outline"
                        disabled={!canInteract}
                      >
                        <TagIcon className="size-3" />
                        {tag}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </ScrollArea>
              )}

              <div className="flex flex-wrap items-center gap-1.5">
                {selectedTags.length === 0 && (
                  <Badge size="sm" variant="secondary">
                    All tags enabled
                  </Badge>
                )}
                {unavailableTags.map(tag => (
                  <Button
                    key={`unavailable-${tag}`}
                    type="button"
                    size="xs"
                    variant="outline"
                    onClick={() => {
                      const next = selectedTags.filter(value => value !== tag)
                      const nextUnavailableTags = next.filter(value => !availableSet.has(value))
                      field.onChange(next)
                      track(AnalyticsEvents.SETTINGS_SUBAGENT_TAGS_CHANGE, {
                        selected_count: next.length,
                        unavailable_count: nextUnavailableTags.length,
                      })
                    }}
                    disabled={!canInteract}
                  >
                    <AlertTriangleIcon className="size-3.5" />
                    {tag}
                    <XIcon className="size-3.5" />
                  </Button>
                ))}
              </div>
            </div>
          )
        }}
      />
    </SettingsCard.Field>
  )
}

interface ApiKeyFieldProps {
  apiKeyInput: string
  onApiKeyChange: (value: string) => void
  help?: FieldHelpContent
}

const ApiKeyField = ({ apiKeyInput, onApiKeyChange, help }: ApiKeyFieldProps) => {
  const { canEdit, isSaving } = useSettingsCardContext()

  return (
    <SettingsCard.Field
      label="API Key"
      description="Leave blank to keep the current key"
      htmlFor="subagent-api-key"
      help={help}
    >
      <InputGroup className="w-full">
        <InputGroupInput
          id="subagent-api-key"
          type="password"
          autoComplete="off"
          value={apiKeyInput}
          onChange={event => onApiKeyChange(event.target.value)}
          placeholder="Paste API key"
          disabled={!canEdit || isSaving}
        />
      </InputGroup>
    </SettingsCard.Field>
  )
}
