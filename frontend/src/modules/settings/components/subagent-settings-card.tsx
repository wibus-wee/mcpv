// Input: subagent form state, model list, fetch handlers
// Output: SubAgent settings card
// Position: Settings page SubAgent section

import { AlertCircleIcon, SaveIcon, ShieldAlertIcon } from 'lucide-react'
import type * as React from 'react'
import { Controller, type UseFormReturn } from 'react-hook-form'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Combobox,
  ComboboxCollection,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxPopup,
} from '@/components/ui/combobox'
import {
  InputGroup,
  InputGroupInput,
  InputGroupTextarea,
} from '@/components/ui/input-group'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Spinner } from '@/components/ui/spinner'
import { SUBAGENT_PROVIDER_OPTIONS, type SubAgentFormState } from '../lib/subagent-config'
import type { ModelFetchState } from '../lib/subagent-models'
import { normalizeNumber } from '../lib/form-utils'
import { RuntimeFieldRow, RuntimeNumberRow } from './runtime-field-rows'

interface SubAgentSettingsCardProps {
  canEdit: boolean
  form: UseFormReturn<SubAgentFormState>
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
  statusLabel,
  saveDisabledReason,
  hasSubAgentChanges,
  subAgentError,
  onSubmit,
}: SubAgentSettingsCardProps) => {
  const { control, register, formState } = form
  const isSaving = formState.isSubmitting
  const selectedModelValue = modelOptionIDs.includes(modelInputValue)
    ? modelInputValue
    : null

  return (
    <form
      className="flex w-full flex-col gap-0"
      onSubmit={onSubmit}
    >
      <Card className="p-1">
        <CardHeader className="pt-3">
          <CardTitle className="flex items-center gap-2">
            SubAgent
            {!canEdit && (
              <Badge variant="warning" size="sm">
                Read-only
              </Badge>
            )}
          </CardTitle>
          <CardDescription>
            Configure the LLM provider for automatic tool filtering.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {!canEdit && (
            <Alert variant="warning">
              <ShieldAlertIcon />
              <AlertTitle>Configuration is read-only</AlertTitle>
              <AlertDescription>
                Update permissions to enable SubAgent edits.
              </AlertDescription>
            </Alert>
          )}

          {subAgentError && (
            <Alert variant="error">
              <AlertCircleIcon />
              <AlertTitle>Failed to load SubAgent settings</AlertTitle>
              <AlertDescription>
                {subAgentError instanceof Error
                  ? subAgentError.message
                  : 'Unable to load SubAgent configuration.'}
              </AlertDescription>
            </Alert>
          )}

          <div className="space-y-6">
            <div className="space-y-3">
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Provider
              </div>
              <div className="divide-y divide-border">
                <RuntimeFieldRow
                  label="Provider"
                  description="Currently only OpenAI is supported"
                  htmlFor="subagent-provider"
                >
                  <Controller
                    control={control}
                    name="provider"
                    render={({ field }) => (
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                        disabled={!canEdit || isSaving}
                      >
                        <SelectTrigger id="subagent-provider">
                          <SelectValue>
                            {value => {
                              const option = SUBAGENT_PROVIDER_OPTIONS.find(
                                item => item.value === value,
                              )
                              return option?.label ?? 'Select provider'
                            }}
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent>
                          {SUBAGENT_PROVIDER_OPTIONS.map(option => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    )}
                  />
                </RuntimeFieldRow>
                <RuntimeFieldRow
                  label="Model"
                  description="Fetch available models from the provider and match with models.dev"
                  htmlFor="subagent-model"
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
                </RuntimeFieldRow>
              </div>
              {modelFetchError && (
                <Alert variant="error">
                  <AlertCircleIcon />
                  <AlertTitle>Failed to fetch models</AlertTitle>
                  <AlertDescription>{modelFetchError}</AlertDescription>
                </Alert>
              )}
            </div>

            <div className="space-y-3">
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Credentials
              </div>
              <div className="divide-y divide-border">
                <RuntimeFieldRow
                  label="API Key"
                  description="Leave blank to keep the current key"
                  htmlFor="subagent-api-key"
                >
                  <InputGroup className="w-full">
                    <InputGroupInput
                      id="subagent-api-key"
                      type="password"
                      autoComplete="off"
                      value={apiKeyInput}
                      onChange={(event) => onApiKeyChange(event.target.value)}
                      placeholder="Paste API key"
                      disabled={!canEdit || isSaving}
                    />
                  </InputGroup>
                </RuntimeFieldRow>
                <RuntimeFieldRow
                  label="API Key Env Var"
                  description="Environment variable name used when no inline key is set"
                  htmlFor="subagent-api-key-env"
                >
                  <InputGroup className="w-full">
                    <InputGroupInput
                      id="subagent-api-key-env"
                      placeholder="OPENAI_API_KEY"
                      disabled={!canEdit || isSaving}
                      {...register('apiKeyEnvVar')}
                    />
                  </InputGroup>
                </RuntimeFieldRow>
                <RuntimeFieldRow
                  label="Base URL"
                  description="Optional override for the provider endpoint"
                  htmlFor="subagent-base-url"
                >
                  <InputGroup className="w-full">
                    <InputGroupInput
                      id="subagent-base-url"
                      placeholder="https://api.openai.com/v1"
                      disabled={!canEdit || isSaving}
                      {...register('baseURL')}
                    />
                  </InputGroup>
                </RuntimeFieldRow>
              </div>
            </div>

            <div className="space-y-3">
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Behavior
              </div>
              <div className="divide-y divide-border">
                <RuntimeNumberRow
                  id="subagent-max-tools"
                  label="Max Tools Per Request"
                  description="Upper bound on tool candidates per query"
                  unit="tools"
                  disabled={!canEdit || isSaving}
                  inputProps={register('maxToolsPerRequest', {
                    valueAsNumber: true,
                    setValueAs: normalizeNumber,
                  })}
                />
                <RuntimeFieldRow
                  label="Filter Prompt"
                  description="Optional prompt override for tool filtering"
                  htmlFor="subagent-filter-prompt"
                  className="sm:grid-cols-[minmax(0,1fr)_minmax(0,360px)]"
                >
                  <InputGroup className="w-full" data-align="block-start">
                    <InputGroupTextarea
                      id="subagent-filter-prompt"
                      placeholder="Describe how tools should be filtered..."
                      rows={4}
                      disabled={!canEdit || isSaving}
                      {...register('filterPrompt')}
                    />
                  </InputGroup>
                </RuntimeFieldRow>
              </div>
            </div>
          </div>
        </CardContent>
        <CardFooter className="border-t">
          <div className="flex w-full flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="text-xs text-muted-foreground">
              {statusLabel}
            </div>
            <Button
              type="submit"
              size="sm"
              disabled={!canEdit || !hasSubAgentChanges || isSaving}
              title={saveDisabledReason}
            >
              {isSaving ? (
                <Spinner className="size-4" />
              ) : (
                <SaveIcon className="size-4" />
              )}
              {isSaving ? 'Saving...' : 'Save changes'}
            </Button>
          </div>
        </CardFooter>
      </Card>
    </form>
  )
}
