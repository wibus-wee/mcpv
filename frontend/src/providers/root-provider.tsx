// Input: Wails runtime events, SWR cache, theme/motion providers
// Output: RootProvider component with Wails event bridge
// Position: App-level providers and core/log event integration

'use client'

import { WailsService } from '@bindings/mcpd/internal/ui'
import { Events } from '@wailsio/runtime'
import { Provider } from 'jotai'
import { LazyMotion, MotionConfig } from 'motion/react'
import { ThemeProvider } from 'next-themes'
import { useEffect, useRef } from 'react'
import { useSWRConfig } from 'swr'

import type { CoreStateResponse } from '@bindings/mcpd/internal/ui'
import type { LogEntry } from '@/hooks/use-logs'
import { coreStateKey, useCoreState } from '@/hooks/use-core-state'
import { logsKey, maxLogEntries } from '@/hooks/use-logs'
import { jotaiStore } from '@/lib/jotai'
import { Spring } from '@/lib/spring'

function WailsEventsBridge() {
  const { mutate } = useSWRConfig()
  const { coreStatus } = useCoreState()
  const stopRef = useRef<(() => void) | null>(null)

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
      console.log('[WailsEvents] core:state received:', { state, data })
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

  const level = coreStatus === 'running' ? 'debug' : null

  useEffect(() => {
    console.log('[WailsEvents] Log stream effect triggered:', { level, coreStatus })
    
    if (level === null) {
      // Stop existing stream when core is not running
      console.log('[WailsEvents] Stopping log stream (core not running)')
      stopRef.current?.()
      stopRef.current = null
      return
    }

    let cancelled = false
    let unbind: (() => void) | undefined

    const start = async () => {
      console.log('[WailsEvents] Starting log stream with level:', level)
      try {
        await WailsService.StartLogStream(level)
        console.log('[WailsEvents] Log stream started successfully')
      }
      catch (err) {
        console.error('[WailsEvents] Failed to start log stream', err)
        return
      }
      if (cancelled) {
        await WailsService.StopLogStream().catch(() => {})
        return
      }

      unbind = Events.On('logs:entry', (event) => {
        const logEntry = event?.data as {
          logger?: string
          level?: string
          timestamp?: string
          data?: { message?: string } | string
        } | undefined

        console.log('[WailsEvents] logs:entry received:', logEntry)

        const rawLevel = String(logEntry?.level ?? 'info').toLowerCase()
        const parsedLevel
          = rawLevel === 'warning' || rawLevel === 'warn'
            ? 'warn'
            : rawLevel === 'error'
              || rawLevel === 'critical'
              || rawLevel === 'alert'
              || rawLevel === 'emergency'
              ? 'error'
              : rawLevel === 'debug'
                ? 'debug'
                : 'info'
        const timestamp = logEntry?.timestamp
          ? new Date(logEntry.timestamp)
          : new Date()
        const logData = logEntry?.data
        const message
          = typeof logData === 'string'
            ? logData
            : typeof logData?.message === 'string'
              ? logData.message
              : typeof logData === 'object'
                ? JSON.stringify(logData)
                : ''

        mutate(
          logsKey,
          (current?: LogEntry[]) => {
            const next = [
              {
                id: crypto.randomUUID(),
                timestamp,
                level: parsedLevel,
                message,
                source: logEntry?.logger,
              },
              ...(current ?? []),
            ]
            return next.slice(0, maxLogEntries)
          },
          { revalidate: false },
        )
      })

      stopRef.current = () => {
        unbind?.()
        unbind = undefined
        stopRef.current = null
        WailsService.StopLogStream().catch(() => {})
      }
    }

    start()

    return () => {
      cancelled = true
      stopRef.current?.()
    }
  }, [level, mutate])

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
          <Provider store={jotaiStore}>
            {children}
            <WailsEventsBridge />
          </Provider>
        </ThemeProvider>
      </MotionConfig>
    </LazyMotion>
  )
}
