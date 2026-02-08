// Input: @umami/node, jotai store, localStorage, window location, env channel
// Output: Analytics service with enable/disable toggle and tracking helpers
// Position: Core analytics utility for Umami integration

import umami from '@umami/node'
import { atom } from 'jotai'

import { createAtomAccessor, createAtomHooks, jotaiStore } from './jotai'

const STORAGE_KEY = 'mcpv-analytics-enabled'
const UMAMI_WEBSITE_ID = import.meta.env.VITE_UMAMI_WEBSITE_ID || '__UMAMI_WEBSITE_ID__'
const UMAMI_HOST_URL = import.meta.env.VITE_UMAMI_HOST_URL || 'https://cloud.umami.is'
const ANALYTICS_CHANNEL = import.meta.env.VITE_ANALYTICS_CHANNEL
  || (import.meta.env.DEV ? 'dev' : 'prod')
const ENV_MODE = import.meta.env.MODE || (import.meta.env.DEV ? 'dev' : 'prod')

const analyticsEnabledAtom = atom(
  typeof localStorage !== 'undefined'
    ? localStorage.getItem(STORAGE_KEY) !== 'false'
    : true,
)

export const [
  useAnalyticsEnabled,
  useSetAnalyticsEnabled,
  useAnalyticsEnabledValue,
] = createAtomHooks(analyticsEnabledAtom)

export const [getAnalyticsEnabled, setAnalyticsEnabled] = createAtomAccessor(analyticsEnabledAtom)

let initialized = false

const ensureInit = () => {
  if (initialized) return
  if (
    UMAMI_WEBSITE_ID === '__UMAMI_WEBSITE_ID__'
    || UMAMI_HOST_URL === '__UMAMI_HOST_URL__'
  ) {
    return
  }
  umami.init({
    userAgent: navigator.userAgent,
    websiteId: UMAMI_WEBSITE_ID,
    hostUrl: UMAMI_HOST_URL,
  })
  initialized = true
}

const withRoute = (data?: Record<string, unknown>) => {
  if (typeof window === 'undefined') return data
  const route = window.location?.pathname
  if (!route) return data
  if (!data) return { route }
  if (Object.prototype.hasOwnProperty.call(data, 'route')) return data
  return { route, ...data }
}

const withContext = (data?: Record<string, unknown>) => {
  const base = {
    build_channel: ANALYTICS_CHANNEL,
    env_mode: ENV_MODE,
  }
  if (!data) {
    return base
  }
  return { ...data, ...base }
}

export const track = (name: string, data?: Record<string, unknown>) => {
  if (!jotaiStore.get(analyticsEnabledAtom)) return
  ensureInit()
  if (!initialized) return
  const merged = withContext(withRoute(data))
  umami.track({ name, data: merged })
}

export const trackPageView = (url: string, title?: string) => {
  if (!jotaiStore.get(analyticsEnabledAtom)) return
  ensureInit()
  if (!initialized) return
  umami.track({ url, title, data: withContext() })
}

export const toggleAnalytics = (enabled: boolean) => {
  if (enabled) {
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(STORAGE_KEY, String(true))
    }
    setAnalyticsEnabled(true)
    track(AnalyticsEvents.SETTINGS_ANALYTICS_TOGGLE, { enabled: true })
    return
  }

  track(AnalyticsEvents.SETTINGS_ANALYTICS_TOGGLE, { enabled: false })
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(STORAGE_KEY, String(false))
  }
  setAnalyticsEnabled(false)
}

export const AnalyticsEvents = {
  APP_LAUNCH: 'app_launch',
  PAGE_VIEW: 'page_view',
  SETTINGS_ANALYTICS_TOGGLE: 'settings_analytics_toggle',
  DEEP_LINK_OPENED: 'deep_link_opened',
  CORE_START: 'core_start',
  CORE_STOP: 'core_stop',
  CORE_RESTART: 'core_restart',
  CORE_STATE_REFRESH: 'core_state_refresh',
  DEBUG_SNAPSHOT_EXPORT: 'debug_snapshot_export',
  LOGS_FILTER_CHANGED: 'logs_filter_changed',
  LOGS_SEARCH: 'logs_search',
  LOGS_CLEAR: 'logs_clear',
  LOGS_STREAM_REFRESH: 'logs_stream_refresh',
  LOG_ROW_SELECTED: 'log_row_selected',
  LOG_DETAIL_TOGGLE: 'log_detail_toggle',
  LOGS_BOTTOM_PANEL_TOGGLE: 'logs_bottom_panel_toggle',
  SERVER_SEARCH: 'server_search',
  SERVER_DETAIL_OPENED: 'server_detail_opened',
  SERVER_TAB_CHANGED: 'server_tab_changed',
  SERVER_EDIT_OPENED: 'server_edit_opened',
  SERVER_TOOLS_EXPAND: 'server_tools_expand',
  SERVER_SAVE: 'server_save',
  SERVER_TOGGLE_DISABLED: 'server_toggle_disabled',
  SERVER_DELETE: 'server_delete',
  SERVER_IMPORT_OPENED: 'server_import_opened',
  SERVER_IMPORT_PARSE: 'server_import_parse',
  SERVER_IMPORT_APPLY: 'server_import_apply',
  TOPOLOGY_VIEWED: 'topology_viewed',
  TOPOLOGY_NODE_FOCUS: 'topology_node_focus',
  SETTINGS_RUNTIME_SAVE: 'settings_runtime_save',
  SETTINGS_SUBAGENT_SAVE: 'settings_subagent_save',
  SETTINGS_SUBAGENT_FETCH_MODELS: 'settings_subagent_fetch_models',
  SETTINGS_SUBAGENT_MODEL_SELECT: 'settings_subagent_model_select',
  SETTINGS_SUBAGENT_TAGS_CHANGE: 'settings_subagent_tags_change',
  SETTINGS_THEME_CHANGE: 'settings_theme_change',
  PLUGIN_SEARCH: 'plugin_search',
  PLUGIN_EDIT_OPENED: 'plugin_edit_opened',
  PLUGIN_SAVE_ATTEMPTED: 'plugin_save_attempted',
  CONNECT_IDE_OPENED: 'connect_ide_opened',
  CONNECT_IDE_TARGET_CHANGE: 'connect_ide_target_change',
  CONNECT_IDE_TAB_CHANGE: 'connect_ide_tab_change',
  CONNECT_IDE_COPY: 'connect_ide_copy',
  CONNECT_IDE_INSTALL_CURSOR: 'connect_ide_install_cursor',
  PLUGIN_INSTALL: 'plugin_install',
  PLUGIN_REMOVE: 'plugin_remove',
} as const
