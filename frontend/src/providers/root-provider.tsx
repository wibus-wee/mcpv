'use client'

import { Provider } from 'jotai'
import { LazyMotion, MotionConfig } from 'motion/react'
import { ThemeProvider } from 'next-themes'
import { useEffect, useMemo, useRef } from 'react'
import { Events } from '@wailsio/runtime'
import { useAtomValue, useSetAtom } from 'jotai'

import { logsAtom } from '@/atoms/dashboard'
import { coreStatusAtom } from '@/atoms/core'
import { Spring } from '@/lib/spring'
import { WailsService } from '@bindings/mcpd/internal/ui'
import { jotaiStore } from '@/lib/jotai'

function WailsEventsBridge() {
  const setLogs = useSetAtom(logsAtom)
  const coreStatus = useAtomValue(coreStatusAtom)
  const stopRef = useRef<(() => void) | null>(null)

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
      } catch (err) {
        console.error('Failed to start log stream', err)
        return
      }
      if (cancelled) {
        await WailsService.StopLogStream().catch(() => {})
        return
      }

      unbind = Events.On('logs:entry', (event: any) => {
        const rawLevel = String(event?.level ?? 'info').toLowerCase()
        const parsedLevel =
          rawLevel === 'warning' || rawLevel === 'warn'
            ? 'warn'
            : rawLevel === 'error' ||
                rawLevel === 'critical' ||
                rawLevel === 'alert' ||
                rawLevel === 'emergency'
              ? 'error'
              : rawLevel === 'debug'
                ? 'debug'
                : 'info'
        const timestamp = event?.timestamp ? new Date(event.timestamp) : new Date()
        const data = event?.data ?? {}
        const message =
          typeof data.message === 'string'
            ? data.message
            : typeof data === 'object'
              ? JSON.stringify(data)
              : String(data ?? '')

        setLogs((prev) => [
          {
            id: crypto.randomUUID(),
            timestamp,
            level: parsedLevel ?? 'info',
            message,
            source: event?.logger,
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
