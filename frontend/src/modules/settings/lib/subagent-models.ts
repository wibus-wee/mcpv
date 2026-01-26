export type ModelCatalogEntry = {
  id: string
  label: string
  provider: string
}

export type ModelsDevIndex = Map<string, Map<string, ModelCatalogEntry>>

export type ModelFetchState = 'idle' | 'loading' | 'ready' | 'error'

export const MODELS_DEV_API_URL = 'https://models.dev/api.json'
export const MODEL_FETCH_TIMEOUT_MS = 15_000

const DEFAULT_PROVIDER_BASE_URLS: Record<string, string> = {
  openai: 'https://api.openai.com/v1',
}

const normalizeProvider = (value: string) =>
  value.toLowerCase().replaceAll(/[^a-z0-9]/g, '')

export const resolveProviderBaseURL = (provider: string, baseURL: string) => {
  const trimmedBaseURL = baseURL.trim().replace(/\/+$/, '')
  if (trimmedBaseURL) {
    return trimmedBaseURL
  }
  const normalized = normalizeProvider(provider)
  return DEFAULT_PROVIDER_BASE_URLS[normalized] ?? ''
}

export const buildModelsURL = (baseURL: string) =>
  `${baseURL.replace(/\/+$/, '')}/models`

export const parseModelsDevIndex = (payload: unknown): ModelsDevIndex => {
  const index: ModelsDevIndex = new Map()
  if (!payload || typeof payload !== 'object') {
    return index
  }

  const providers = payload as Record<string, unknown>
  for (const [providerKey, providerValue] of Object.entries(providers)) {
    if (!providerValue || typeof providerValue !== 'object') {
      continue
    }
    const providerRecord = providerValue as Record<string, unknown>
    const providerID = normalizeProvider(String(providerRecord.id ?? providerKey ?? ''))
    if (!providerID) {
      continue
    }

    const modelEntries = new Map<string, ModelCatalogEntry>()
    const addModel = (id: string, name?: string) => {
      const trimmedID = id.trim()
      if (!trimmedID) {
        return
      }
      const label = name?.trim() || trimmedID
      modelEntries.set(trimmedID, {
        id: trimmedID,
        label,
        provider: providerID,
      })
    }

    const modelsValue = providerRecord.models
    if (Array.isArray(modelsValue)) {
      for (const modelItem of modelsValue) {
        if (!modelItem || typeof modelItem !== 'object') {
          continue
        }
        const record = modelItem as Record<string, unknown>
        const idValue = record.id ?? record.modelId ?? record.model_id ?? record.model ?? record.name
        if (typeof idValue !== 'string') {
          continue
        }
        const nameValue = record.name ?? record.label ?? record.displayName ?? idValue
        addModel(idValue, typeof nameValue === 'string' ? nameValue : undefined)
      }
    }
    else if (modelsValue && typeof modelsValue === 'object') {
      for (const [modelKey, modelValue] of Object.entries(modelsValue as Record<string, unknown>)) {
        if (modelValue && typeof modelValue === 'object') {
          const record = modelValue as Record<string, unknown>
          const idValue = record.id ?? modelKey
          const nameValue = record.name ?? record.label ?? record.displayName ?? idValue
          addModel(String(idValue), typeof nameValue === 'string' ? nameValue : undefined)
        }
        else {
          addModel(modelKey)
        }
      }
    }

    if (modelEntries.size > 0) {
      index.set(providerID, modelEntries)
    }
  }

  return index
}

export const parseProviderModelIDs = (payload: unknown): string[] => {
  const collectList = (input: unknown): unknown[] => {
    if (Array.isArray(input)) {
      return input
    }
    if (input && typeof input === 'object') {
      const record = input as Record<string, unknown>
      if (Array.isArray(record.data)) {
        return record.data
      }
      if (Array.isArray(record.models)) {
        return record.models
      }
      if (Array.isArray(record.items)) {
        return record.items
      }
    }
    return []
  }

  const list = collectList(payload)
  const ids = new Map<string, string>()
  for (const item of list) {
    if (typeof item === 'string') {
      const trimmed = item.trim()
      if (trimmed) {
        ids.set(trimmed, trimmed)
      }
      continue
    }
    if (!item || typeof item !== 'object') {
      continue
    }
    const record = item as Record<string, unknown>
    const idValue = record.id ?? record.model ?? record.name ?? record.slug ?? record.key
    if (typeof idValue === 'string') {
      const trimmed = idValue.trim()
      if (trimmed) {
        ids.set(trimmed, trimmed)
      }
    }
  }
  return Array.from(ids.values())
}

export const mergeProviderModels = (
  provider: string,
  modelIDs: string[],
  modelsDevIndex: ModelsDevIndex,
): ModelCatalogEntry[] => {
  const providerID = normalizeProvider(provider)
  const modelsDevProvider = modelsDevIndex.get(providerID)
  return modelIDs.map((modelID) => {
    const matched = modelsDevProvider?.get(modelID)
    return {
      id: modelID,
      label: matched?.label ?? modelID,
      provider: providerID || provider,
    }
  })
}
