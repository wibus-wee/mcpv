// Input: CoreService bindings, SWR, motion, @tsparticles/confetti
// Output: AboutPage component with animated background, app icon (confetti on long press), name, description, and made by
// Position: About module page content for the /about route

import { CoreService } from '@bindings/mcpv/internal/ui/services'
import { confetti } from '@tsparticles/confetti'
import { HeartHandshakeIcon } from 'lucide-react'
import { m } from 'motion/react'
import { useCallback, useRef } from 'react'
import useSWR from 'swr'

import { Skeleton } from '@/components/ui/skeleton'
import { Spring } from '@/lib/spring'
import { swrPresets } from '@/lib/swr-config'
import { swrKeys } from '@/lib/swr-keys'

type AppInfo = Awaited<ReturnType<typeof CoreService.GetInfo>>

function useAppInfo() {
  return useSWR<AppInfo>(
    swrKeys.appInfo,
    () => CoreService.GetInfo(),
    swrPresets.longCached,
  )
}

function AnimatedBackground() {
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden">
      <div className="absolute -left-1/4 -top-1/4 size-[600px] animate-[drift_20s_ease-in-out_infinite] rounded-full bg-gradient-to-br from-amber-400/20 via-orange-300/10 to-transparent blur-3xl will-change-transform" />
      <div className="absolute -bottom-1/4 -right-1/4 size-[500px] animate-[drift_25s_ease-in-out_infinite_reverse] rounded-full bg-gradient-to-tl from-sky-400/15 via-cyan-300/10 to-transparent blur-3xl will-change-transform" />
    </div>
  )
}

export function AboutPage() {
  const { data: appInfo, isLoading } = useAppInfo()
  const iconRef = useRef<HTMLDivElement>(null)

  const triggerConfetti = useCallback(() => {
    if (!iconRef.current) return
    const rect = iconRef.current.getBoundingClientRect()
    const x = (rect.left + rect.width / 2) / window.innerWidth
    const y = (rect.top + rect.height / 2) / window.innerHeight
    confetti({
      particleCount: 100,
      spread: 170,
      origin: { x, y },
      startVelocity: 30,
    })
  }, [])

  return (
    <div className="relative flex flex-1 flex-col items-center justify-center gap-6 overflow-hidden p-6">
      <AnimatedBackground />

      <div className="relative z-10 flex flex-col items-center gap-5 text-center">
        {/* App Icon */}
        <m.div
          ref={iconRef}
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1, rotate: 0 }}
          transition={Spring.presets.bouncy}
          whileHover={{ scale: 1.1, rotate: 3 }}
          whileTap={{ scale: 0.9, rotate: -3 }}
          className="flex cursor-pointer select-none items-center justify-center"
          onMouseUp={triggerConfetti}
        >
          <img src="/appicon.png" alt="mcpv" className="size-24 drop-shadow-lg" draggable={false} />
        </m.div>

        {/* App Name + Version */}
        <m.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ type: 'spring', duration: 0.4, bounce: 0, delay: 0.08 }}
        >
          {isLoading ? (
            <Skeleton className="h-8 w-36" />
          ) : (
            <h1 className="text-2xl font-semibold tracking-tight">
              MCPV UI
              {appInfo?.version && (
                <span className="ml-2 text-base font-normal text-muted-foreground">
                  v{appInfo.version}
                </span>
              )}
            </h1>
          )}
        </m.div>

        {/* Description */}
        <m.p
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ type: 'spring', duration: 0.4, bounce: 0, delay: 0.16 }}
          className="max-w-xs text-sm leading-relaxed text-muted-foreground"
        >
          Lightweight Elastic Orchestrator for Model Context Protocol Servers (MCP Gateway)

        </m.p>

        {/* Made by */}
        <m.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ type: 'spring', duration: 0.4, bounce: 0, delay: 0.24 }}
          className="mt-2 flex items-center gap-1.5 text-sm text-muted-foreground"
        >
          <span>Made with</span>
          <HeartHandshakeIcon className="size-4 text-pink-500" />
          <span>by Wibus</span>
        </m.div>
      </div>
    </div>
  )
}
