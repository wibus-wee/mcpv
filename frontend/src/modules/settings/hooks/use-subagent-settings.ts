// Input: subagent bindings, proxy service, react-hook-form
// Output: subagent settings state + model fetch helpers
// Position: Settings SubAgent hook

import type {
  ActiveClient,
  ServerSummary,
  UpdateSubAgentConfigRequest,
} from '@bindings/mcpd/internal/ui'
import { ProxyService, SubAgentService } from '@bindings/mcpd/internal/ui'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import useSWR from 'swr'

import { toastManager } from '@/components/ui/toast'
import { useClients, useServers } from '@/modules/config/hooks'
import { reloadConfig } from '@/modules/config/lib/reload-config'
import {
  DEFAULT_SUBAGENT_FORM,
  type SubAgentFormState,
  toSubAgentFormState,
} from '../lib/subagent-config'
import {
  MODELS_DEV_API_URL,
  MODEL_FETCH_TIMEOUT_MS,
  type ModelCatalogEntry,
  type ModelFetchState,
  buildModelsURL,
  mergeProviderModels,
  parseModelsDevIndex,
  parseProviderModelIDs,
  resolveProviderBaseURL,
} from '../lib/subagent-models'

type UseSubAgentSettingsOptions = {
  canEdit: boolean
}

const buildAvailableTags = (
  servers?: ServerSummary[],
  clients?: ActiveClient[],
) => {
  const tagSet = new Set<string>()
  servers?.forEach((server) => {
    server.tags?.forEach((tag) => tagSet.add(tag))
  })
  clients?.forEach((client) => {
    client.tags?.forEach((tag) => tagSet.add(tag))
  })
  return Array.from(tagSet).sort((a, b) => a.localeCompare(b))
}

export const useSubAgentSettings = ({ canEdit }: UseSubAgentSettingsOptions) => {
  const form = useForm<SubAgentFormState>({
    defaultValues: DEFAULT_SUBAGENT_FORM,
  })
  const { reset, formState, getValues, setValue, watch } = form
  const isDirty = formState.isDirty

  const {
    data: subAgentConfig,
    error: subAgentError,
    isLoading: subAgentLoading,
    mutate: mutateSubAgentConfig,
  } = useSWR(
    'subagent-config',
    () => SubAgentService.GetSubAgentConfig(),
    {
      revalidateOnFocus: false,
    },
  )

  const { data: servers } = useServers()
  const { data: clients } = useClients()

  const [apiKeyInput, setApiKeyInput] = useState('')
  const [modelInputValue, setModelInputValue] = useState('')
  const [modelOptions, setModelOptions] = useState<ModelCatalogEntry[]>([])
  const [modelFetchState, setModelFetchState] = useState<ModelFetchState>('idle')
  const [modelFetchError, setModelFetchError] = useState<string | null>(null)

  const subAgentSnapshotRef = useRef<string | null>(null)
  const selectedProvider = watch('provider', DEFAULT_SUBAGENT_FORM.provider)

  const hasApiKeyInput = apiKeyInput.trim().length > 0
  const hasSubAgentChanges = isDirty || hasApiKeyInput

  useEffect(() => {
    if (!subAgentConfig) {
      return
    }
    if (isDirty || hasApiKeyInput) {
      return
    }
    const nextState = toSubAgentFormState(subAgentConfig)
    const snapshot = JSON.stringify(nextState)
    if (snapshot !== subAgentSnapshotRef.current) {
      subAgentSnapshotRef.current = snapshot
      reset(nextState, { keepDirty: false })
      setModelInputValue(nextState.model)
    }
  }, [subAgentConfig, reset, isDirty, hasApiKeyInput])

  useEffect(() => {
    setModelOptions([])
    setModelFetchError(null)
    setModelFetchState('idle')
  }, [selectedProvider])

  const modelOptionIDs = useMemo(
    () => modelOptions.map(option => option.id),
    [modelOptions],
  )
  const modelLabelMap = useMemo(
    () => new Map(modelOptions.map(option => [option.id, option.label])),
    [modelOptions],
  )

  const availableTags = useMemo(
    () => buildAvailableTags(servers, clients),
    [servers, clients],
  )

  const modelFetchLabel = useMemo(() => {
    if (modelFetchState === 'loading') {
      return 'Fetching models...'
    }
    if (modelFetchState === 'error') {
      return 'Fetch failed'
    }
    if (modelOptions.length > 0) {
      return `${modelOptions.length} models`
    }
    return 'No models loaded'
  }, [modelFetchState, modelOptions.length])

  const setModelValue = useCallback((value: string) => {
    setModelInputValue(value)
    setValue('model', value, { shouldDirty: true })
  }, [setValue])

  const fetchModels = useCallback(async () => {
    const apiKey = apiKeyInput.trim()
    if (!apiKey) {
      setModelFetchError('API key is required to fetch models.')
      setModelFetchState('error')
      return
    }

    const { provider, baseURL } = getValues()
    const resolvedBaseURL = resolveProviderBaseURL(provider, baseURL)
    if (!resolvedBaseURL) {
      setModelFetchError('Base URL is required to fetch models.')
      setModelFetchState('error')
      return
    }

    setModelFetchState('loading')
    setModelFetchError(null)

    try {
      const modelsURL = buildModelsURL(resolvedBaseURL)
      const [modelsDevResponse, providerResponse] = await Promise.all([
        ProxyService.Fetch({
          url: MODELS_DEV_API_URL,
          method: 'GET',
          headers: {},
          timeoutMs: MODEL_FETCH_TIMEOUT_MS,
        }),
        ProxyService.Fetch({
          url: modelsURL,
          method: 'GET',
          headers: {
            Authorization: `Bearer ${apiKey}`,
          },
          timeoutMs: MODEL_FETCH_TIMEOUT_MS,
        }),
      ])

      let modelsDevIndex = new Map()
      if (modelsDevResponse.status >= 200 && modelsDevResponse.status < 300) {
        const modelsDevPayload = JSON.parse(modelsDevResponse.body) as unknown
        modelsDevIndex = parseModelsDevIndex(modelsDevPayload)
      } else {
        setModelFetchError(`models.dev returned ${modelsDevResponse.status}`)
      }

      if (providerResponse.status < 200 || providerResponse.status >= 300) {
        throw new Error(`Provider returned ${providerResponse.status}`)
      }
      const providerPayload = JSON.parse(providerResponse.body) as unknown
      const providerModelIDs = parseProviderModelIDs(providerPayload)
      const mergedModels = mergeProviderModels(provider, providerModelIDs, modelsDevIndex)

      setModelOptions(mergedModels)
      setModelFetchState('ready')
    } catch (err) {
      setModelFetchError(err instanceof Error ? err.message : 'Failed to fetch models.')
      setModelFetchState('error')
    }
  }, [apiKeyInput, getValues])

  const handleSave = form.handleSubmit(async (values) => {
    if (!canEdit) {
      return
    }
    try {
      const req: UpdateSubAgentConfigRequest = {
        enabledTags: values.enabledTags,
        model: values.model,
        provider: values.provider,
        apiKeyEnvVar: values.apiKeyEnvVar,
        baseURL: values.baseURL,
        maxToolsPerRequest: values.maxToolsPerRequest,
        filterPrompt: values.filterPrompt,
      }
      const trimmedKey = apiKeyInput.trim()
      if (trimmedKey) {
        req.apiKey = trimmedKey
      }

      await SubAgentService.UpdateSubAgentConfig(req)

      const reloadResult = await reloadConfig()
      if (!reloadResult.ok) {
        toastManager.add({
          type: 'error',
          title: 'Reload failed',
          description: reloadResult.message,
        })
        return
      }

      await mutateSubAgentConfig()
      reset(values, { keepDirty: false })
      setApiKeyInput('')
      setModelInputValue(values.model)

      toastManager.add({
        type: 'success',
        title: 'SubAgent updated',
        description: 'SubAgent configuration updated successfully.',
      })
    } catch (err) {
      toastManager.add({
        type: 'error',
        title: 'Update failed',
        description: err instanceof Error ? err.message : 'Update failed',
      })
    }
  })

  const statusLabel = useMemo(() => {
    if (subAgentLoading) {
      return 'Loading SubAgent settings'
    }
    if (hasSubAgentChanges) {
      return 'Unsaved changes'
    }
    return 'All changes saved'
  }, [hasSubAgentChanges, subAgentLoading])

  const saveDisabledReason = useMemo(() => {
    if (subAgentLoading) {
      return 'SubAgent settings are still loading'
    }
    if (!canEdit) {
      return 'Configuration is read-only'
    }
    if (!hasSubAgentChanges) {
      return 'No changes to save'
    }
    return undefined
  }, [canEdit, hasSubAgentChanges, subAgentLoading])

  return {
    form,
    apiKeyInput,
    setApiKeyInput,
    modelInputValue,
    setModelValue,
    modelOptionIDs,
    modelLabelMap,
    modelFetchState,
    modelFetchError,
    modelFetchLabel,
    hasSubAgentChanges,
    statusLabel,
    saveDisabledReason,
    subAgentError,
    handleSave,
    fetchModels,
    availableTags,
  }
}
