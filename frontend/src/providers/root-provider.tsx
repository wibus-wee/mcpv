// Input: Wails runtime events, SWR cache, router state, analytics, theme/motion providers
// Output: RootProvider component with buffered log stream bridge
// Position: App-level providers and core/log/status event integration

'use client'

import { LogService } from '@bindings/mcpv/internal/ui/services'
import type {
  ActiveClient,
  CoreStateResponse,
  ServerInitStatus,
  ServerRuntimeStatus,
} from '@bindings/mcpv/internal/ui/types'
import { useRouter, useRouterState } from '@tanstack/react-router'
import { Events } from '@wailsio/runtime'
import { Provider, useAtomValue } from 'jotai'
import { LazyMotion, MotionConfig } from 'motion/react'
import { ThemeProvider } from 'next-themes'
import { startTransition, useCallback, useEffect, useRef } from 'react'
import { SWRConfig, useSWRConfig } from 'swr'

import { logStreamTokenAtom } from '@/atoms/logs'
import { AnchoredToastProvider, toastManager, ToastProvider } from '@/components/ui/toast'
import { activeClientsKey } from '@/hooks/use-active-clients'
import { coreStateKey, useCoreState } from '@/hooks/use-core-state'
import type { LogEntry } from '@/hooks/use-logs'
import { logsKey, maxLogEntries } from '@/hooks/use-logs'
import { AnalyticsEvents, track, trackPageView } from '@/lib/analytics'
import { jotaiStore } from '@/lib/jotai'
import { Spring } from '@/lib/spring'
import { swrKeys } from '@/lib/swr-keys'

const logSourceValues = new Set<LogEntry['source']>(['core', 'downstream', 'ui'])

const normalizeLogLevel = (value: unknown): LogEntry['level'] => {
  const raw = String(value ?? 'info').toLowerCase()
  if (raw === 'warning' || raw === 'warn') return 'warn'
  if (raw === 'error' || raw === 'critical' || raw === 'alert' || raw === 'emergency') return 'error'
  if (raw === 'debug') return 'debug'
  return 'info'
}

const normalizeLogSource = (value: unknown): LogEntry['source'] => {
  if (logSourceValues.has(value as LogEntry['source'])) {
    return value as LogEntry['source']
  }
  return 'unknown'
}

const parseLogData = (input: unknown): { message: string, fields: Record<string, unknown> } => {
  if (typeof input === 'string') {
    const trimmed = input.trim()
    if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
      try {
        return parseLogData(JSON.parse(trimmed))
      }
      catch {
        return { message: input, fields: {} }
      }
    }
    return { message: input, fields: {} }
  }

  if (input && typeof input === 'object') {
    const record = input as Record<string, unknown>
    const message = typeof record.message === 'string'
      ? record.message
      : JSON.stringify(record)
    const nestedFields = record.fields
    const fieldMap = (nestedFields && typeof nestedFields === 'object')
      ? (nestedFields as Record<string, unknown>)
      : {}
    const inlineFields = Object.fromEntries(
      Object.entries(record).filter(([key]) => key !== 'message' && key !== 'fields'),
    )
    return { message, fields: { ...inlineFields, ...fieldMap } }
  }

  return { message: '', fields: {} }
}

function WailsEventsBridge() {
  const { mutate } = useSWRConfig()
  const { coreStatus } = useCoreState()
  const stopRef = useRef<(() => void) | null>(null)
  const logStreamToken = useAtomValue(logStreamTokenAtom)
  const router = useRouter()
  const pathname = useRouterState({ select: state => state.location.pathname })
  const logQueueRef = useRef<LogEntry[]>([])
  const logFlushTimerRef = useRef<number | null>(null)
  const lastTrackedPathRef = useRef<string | null>(null)
  const lastUpdateVersionRef = useRef<string | null>(null)

  const flushLogQueue = useCallback(() => {
    if (logQueueRef.current.length === 0) return
    const batch = logQueueRef.current
    logQueueRef.current = []

    startTransition(() => {
      mutate(
        logsKey,
        (current?: LogEntry[]) => {
          const next = [...batch, ...(current ?? [])]
          return next.slice(0, maxLogEntries)
        },
        { revalidate: false },
      )
    })
  }, [mutate])

  const scheduleLogFlush = useCallback(() => {
    if (logFlushTimerRef.current !== null) return
    logFlushTimerRef.current = window.setTimeout(() => {
      logFlushTimerRef.current = null
      flushLogQueue()
    }, 80)
  }, [flushLogQueue])

  // Listen for deep link events from backend
  useEffect(() => {
    const unbind = Events.On('deep-link', (event) => {
      const data = event?.data as { path?: string, params?: Record<string, string> } | undefined
      if (data?.path && data.path !== '/') {
        const search = data.params ?? {}
        // Navigate to the deep link path with query parameters
        router.navigate({ to: data.path, search } as any)
      }
      // If path is '/' or empty, just open the app without navigation
      if (data?.path) {
        const params = data.params ?? {}
        track(AnalyticsEvents.DEEP_LINK_OPENED, {
          path: data.path,
          has_params: Object.keys(params).length > 0,
          params_count: Object.keys(params).length,
        })
      }
    })
    return () => unbind()
  }, [router])

  useEffect(() => {
    const path = pathname
    if (!path || lastTrackedPathRef.current === path) {
      return
    }
    lastTrackedPathRef.current = path
    trackPageView(path, document.title)
  }, [pathname])

  // Listen for core:state events from backend
  useEffect(() => {
    const unbind = Events.On('core:state', (event) => {
      const data = event?.data as { state?: string } | undefined
      const state = data?.state as
        | 'stopped'
        | 'starting'
        | 'running'
        | 'stopping'
        | 'error'
        | undefined
      if (state) {
        mutate(
          coreStateKey,
          (current?: CoreStateResponse) => ({
            ...(current ?? { state, uptime: 0 }),
            state,
          }),
          { revalidate: false },
        )
      }
    })
    return () => unbind()
  }, [mutate])

  // Listen for runtime:status events from backend
  useEffect(() => {
    const unbind = Events.On('runtime:status', (event) => {
      const data = event?.data as { statuses?: ServerRuntimeStatus[] } | undefined
      if (data?.statuses) {
        mutate(swrKeys.runtimeStatus, data.statuses, { revalidate: false })
      }
    })
    return () => unbind()
  }, [mutate])

  // Listen for server-init:status events from backend
  useEffect(() => {
    const unbind = Events.On('server-init:status', (event) => {
      const data = event?.data as { statuses?: ServerInitStatus[] } | undefined
      if (data?.statuses) {
        mutate(swrKeys.serverInitStatus, data.statuses, { revalidate: false })
      }
    })
    return () => unbind()
  }, [mutate])

  // Listen for clients:active events from backend
  useEffect(() => {
    const unbind = Events.On('clients:active', (event) => {
      const data = event?.data as { clients?: ActiveClient[] } | undefined
      if (data?.clients) {
        mutate(activeClientsKey, data.clients, { revalidate: false })
      }
    })
    return () => unbind()
  }, [mutate])

  // Listen for update:available events from backend
  useEffect(() => {
    const unbind = Events.On('update:available', (event) => {
      const data = event?.data as {
        currentVersion?: string
        latest?: {
          version?: string
          name?: string
          url?: string
          prerelease?: boolean
          publishedAt?: string
        }
      } | undefined
      const latest = data?.latest
      if (!latest?.version || !latest?.url) {
        return
      }
      if (lastUpdateVersionRef.current === latest.version) {
        return
      }
      lastUpdateVersionRef.current = latest.version

      const versionLabel = latest.version.startsWith('v')
        ? latest.version
        : `v${latest.version}`
      const description = latest.prerelease
        ? 'Pre-release is available.'
        : `Current version ${data?.currentVersion ?? 'unknown'}`

      toastManager.add({
        type: 'info',
        title: `Update available ${versionLabel}`,
        description: latest.name || description,
        actionProps: {
          children: 'Download',
          onClick: async () => {
            const opened = window.open(latest.url, '_blank', 'noopener,noreferrer')
            if (opened) {
              return
            }
            try {
              await navigator.clipboard.writeText(latest.url || '')
              toastManager.add({
                type: 'success',
                title: 'Link copied',
                description: 'Download link copied to clipboard',
              })
            }
            catch {
              toastManager.add({
                type: 'error',
                title: 'Open failed',
                description: 'Unable to open the download link',
              })
            }
          },
        },
      })
    })
    return () => unbind()
  }, [])

  const level = (coreStatus === 'running' || coreStatus === 'starting') ? 'debug' : null

  useEffect(() => {
    if (level === null) {
      stopRef.current?.()
      stopRef.current = null
      return
    }

    let cancelled = false
    let unbind: (() => void) | undefined

    const start = async () => {
      let streamStarted = false
      const maxRetries = 3
      const retryDelay = 1000 // 1 second

      for (let attempt = 1; attempt <= maxRetries; attempt++) {
        try {
          await LogService.StartLogStream(level)
          streamStarted = true
          break
        }
        catch (err) {
          console.error(`[WailsEvents] Failed to start log stream (attempt ${attempt}/${maxRetries})`, err)
          if (attempt < maxRetries) {
            await new Promise((resolve) => {
              const timeoutId = window.setTimeout(resolve, retryDelay)
              if (cancelled) {
                window.clearTimeout(timeoutId)
              }
            })
          }
        }
      }

      if (cancelled) {
        if (streamStarted) {
          await LogService.StopLogStream().catch(() => { })
        }
        return
      }

      // Always set up the event listener, even if StartLogStream failed
      // The backend might still send logs via other mechanisms
      unbind = Events.On('logs:entry', (event) => {
        const logEntry = event?.data as {
          logger?: string
          level?: string
          timestamp?: string
          data?: Record<string, unknown> | string
        } | undefined
        const timestamp = logEntry?.timestamp
          ? new Date(logEntry.timestamp)
          : new Date()
        const { message, fields } = parseLogData(logEntry?.data)
        const source = normalizeLogSource(fields.log_source)
        const serverType = typeof fields.serverType === 'string'
          ? fields.serverType
          : undefined
        const stream = typeof fields.stream === 'string'
          ? fields.stream
          : undefined
        const logger = typeof logEntry?.logger === 'string'
          ? logEntry.logger
          : undefined

        logQueueRef.current.unshift({
          id: crypto.randomUUID(),
          timestamp,
          level: normalizeLogLevel(logEntry?.level),
          message,
          source,
          fields,
          logger,
          serverType,
          stream,
        })
        scheduleLogFlush()
      })

      stopRef.current = () => {
        unbind?.()
        unbind = undefined
        stopRef.current = null
        if (streamStarted) {
          LogService.StopLogStream().catch(() => { })
        }
      }
    }

    start()

    return () => {
      cancelled = true
      if (logFlushTimerRef.current !== null) {
        window.clearTimeout(logFlushTimerRef.current)
        logFlushTimerRef.current = null
      }
      flushLogQueue()
      stopRef.current?.()
    }
  }, [level, logStreamToken, flushLogQueue, scheduleLogFlush])

  return null
}

const loadFeatures = () =>
  import('@/lib/framer-lazy-feature').then(res => res.default)

export function RootProvider({ children }: { children: React.ReactNode }) {
  return (
    <LazyMotion features={loadFeatures} strict>
      <MotionConfig transition={Spring.presets.smooth}>
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
          <ToastProvider>
            <AnchoredToastProvider>
              <SWRConfig
                value={{
                  revalidateOnMount: true,
                  revalidateOnFocus: false,
                  revalidateOnReconnect: true,
                  dedupingInterval: 2000,
                  shouldRetryOnError: true,
                  errorRetryCount: 3,
                  errorRetryInterval: 1000,
                  keepPreviousData: true,
                }}
              >
                <Provider store={jotaiStore}>
                  {children}
                  <WailsEventsBridge />
                </Provider>
              </SWRConfig>
            </AnchoredToastProvider>
          </ToastProvider>
        </ThemeProvider>
      </MotionConfig>
    </LazyMotion>
  )
}
