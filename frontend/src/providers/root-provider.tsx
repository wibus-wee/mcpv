// Input: Wails runtime events, SWR cache, theme/motion providers
// Output: RootProvider component with buffered log stream bridge
// Position: App-level providers and core/log/status event integration

'use client'

import type {
  ActiveClient,
  CoreStateResponse,
  ServerInitStatus,
  ServerRuntimeStatus,
} from '@bindings/mcpv/internal/ui'
import { LogService } from '@bindings/mcpv/internal/ui'
import { useRouter } from '@tanstack/react-router'
import { Events } from '@wailsio/runtime'
import { Provider, useAtomValue } from 'jotai'
import { LazyMotion, MotionConfig } from 'motion/react'
import { ThemeProvider } from 'next-themes'
import { startTransition, useCallback, useEffect, useRef } from 'react'
import { useSWRConfig } from 'swr'

import { logStreamTokenAtom } from '@/atoms/logs'
import { ToastProvider } from '@/components/ui/toast'
import { activeClientsKey } from '@/hooks/use-active-clients'
import { coreStateKey, useCoreState } from '@/hooks/use-core-state'
import type { LogEntry } from '@/hooks/use-logs'
import { logsKey, maxLogEntries } from '@/hooks/use-logs'
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
  const logQueueRef = useRef<LogEntry[]>([])
  const logFlushTimerRef = useRef<number | null>(null)

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
    })
    return () => unbind()
  }, [router])

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
      try {
        await LogService.StartLogStream(level)
      }
      catch (err) {
        console.error('[WailsEvents] Failed to start log stream', err)
        return
      }
      if (cancelled) {
        await LogService.StopLogStream().catch(() => { })
        return
      }

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
        LogService.StopLogStream().catch(() => { })
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
            <Provider store={jotaiStore}>
              {children}
              <WailsEventsBridge />
            </Provider>
          </ToastProvider>
        </ThemeProvider>
      </MotionConfig>
    </LazyMotion>
  )
}
