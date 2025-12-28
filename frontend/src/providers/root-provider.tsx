'use client'

import { WailsService } from '@bindings/mcpd/internal/ui'
import { Events } from '@wailsio/runtime'
import { Provider, useAtomValue, useSetAtom } from 'jotai'
import { LazyMotion, MotionConfig } from 'motion/react'
import { ThemeProvider } from 'next-themes'
import { useEffect, useMemo, useRef } from 'react'

import { coreStatusAtom } from '@/atoms/core'
import { logsAtom } from '@/atoms/dashboard'
import { jotaiStore } from '@/lib/jotai'
import { Spring } from '@/lib/spring'

function WailsEventsBridge() {
  const setLogs = useSetAtom(logsAtom)
  const setCoreStatus = useSetAtom(coreStatusAtom)
  const coreStatus = useAtomValue(coreStatusAtom)
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
      if (state) {
        setCoreStatus(state)
      }
    })
    return () => unbind()
  }, [setCoreStatus])

  const level = useMemo(() => (coreStatus === 'running' ? 'info' : null), [coreStatus])

  useEffect(() => {
    if (level === null) {
      // Stop existing stream when core is not running
      stopRef.current?.()
      stopRef.current = null
      return
    }

    let cancelled = false
    let unbind: (() => void) | undefined

    const start = async () => {
      try {
        await WailsService.StartLogStream(level)
      }
      catch (err) {
        console.error('Failed to start log stream', err)
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

        setLogs(prev => [
          {
            id: crypto.randomUUID(),
            timestamp,
            level: parsedLevel,
            message,
            source: logEntry?.logger,
          },
          ...prev,
        ])
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
  }, [level, setLogs])

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
