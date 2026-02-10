'use client'

import { ArrowRight, Download, Github } from 'lucide-react'
import type { Easing } from 'motion/react'
import { AnimatePresence, motion, useMotionValue, useSpring, useTransform } from 'motion/react'
import Image from 'next/image'
import Link from 'next/link'
import { useEffect, useRef, useState } from 'react'

const ease: Easing = [0.16, 1, 0.3, 1]

const rotatingPhrases = [
  'one workspace.',
  'effortless creativity.',
  'focused productivity.',
  'seamless collaboration.',
  'intuitive control.',
]

const ROTATE_INTERVAL = 1800
const TOTAL_CYCLES = rotatingPhrases.length * 2 // loop twice then land on index 0

const fadeUp = {
  hidden: { opacity: 0, y: 32, filter: 'blur(12px)' },
  visible: (i: number) => ({
    opacity: 1,
    y: 0,
    filter: 'blur(0px)',
    transition: { delay: 0.04 + i * 0.09, duration: 0.8, ease },
  }),
}

function FloatingOrb({
  className,
  delay,
  duration,
}: {
  className: string
  delay: number
  duration: number
}) {
  return (
    <motion.div
      className={className}
      animate={{
        y: [0, -18, 0, 12, 0],
        x: [0, 10, -6, 8, 0],
        scale: [1, 1.08, 0.96, 1.04, 1],
      }}
      transition={{
        duration,
        delay,
        repeat: Number.POSITIVE_INFINITY,
        ease: 'easeInOut',
      }}
    />
  )
}

function ScreenshotCard() {
  const ref = useRef<HTMLDivElement>(null)
  const mouseX = useMotionValue(0.5)
  const mouseY = useMotionValue(0.5)

  const springX = useSpring(mouseX, { stiffness: 150, damping: 20 })
  const springY = useSpring(mouseY, { stiffness: 150, damping: 20 })

  const rotateX = useTransform(springY, [0, 1], [4, -4])
  const rotateY = useTransform(springX, [0, 1], [-4, 4])

  const [isHovered, setIsHovered] = useState(false)

  return (
    <motion.div
      ref={ref}
      onMouseMove={(e) => {
        if (!ref.current) return
        const rect = ref.current.getBoundingClientRect()
        mouseX.set((e.clientX - rect.left) / rect.width)
        mouseY.set((e.clientY - rect.top) / rect.height)
      }}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => {
        setIsHovered(false)
        mouseX.set(0.5)
        mouseY.set(0.5)
      }}
      style={{
        rotateX: isHovered ? rotateX : 0,
        rotateY: isHovered ? rotateY : 0,
        transformPerspective: 1200,
      }}
      className="relative"
    >
      <div className="relative overflow-hidden">
        <div className="overflow-hidden">
          <Image
            src="/screenshot-light.png"
            alt="MCPV UI — desktop control surface for MCP servers"
            width={1200}
            height={800}
            className="block h-auto w-full dark:hidden"
            priority
          />
          <Image
            src="/screenshot-dark.png"
            alt="MCPV UI — desktop control surface for MCP servers"
            width={1200}
            height={800}
            className="hidden h-auto w-full dark:block"
            priority
          />
        </div>
      </div>
    </motion.div>
  )
}

function RotatingText() {
  const [index, setIndex] = useState(0) // 从第一个开始
  const [tick, setTick] = useState(0)
  const stopped = tick >= TOTAL_CYCLES

  useEffect(() => {
    if (stopped) return
    const delay = ROTATE_INTERVAL
    const id = setTimeout(() => {
      setIndex(prev => (prev + 1) % rotatingPhrases.length)
      setTick(prev => prev + 1)
    }, delay)
    return () => clearTimeout(id)
  }, [tick, stopped])

  return (
    <span className="relative inline-grid h-[1.1em] items-end align-baseline overflow-hidden">
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.span
          key={rotatingPhrases[index]}
          initial={{ y: '70%', opacity: 0, filter: 'blur(10px)' }}
          animate={{ y: 0, opacity: 1, filter: 'blur(0px)' }}
          exit={{ y: '-70%', opacity: 0, filter: 'blur(10px)' }}
          transition={{ duration: 0.75, ease }}
          style={{ willChange: 'transform, filter' }}
          className="col-start-1 row-start-1 inline-block leading-[1.1] bg-linear-to-r from-neutral-400 via-neutral-500 to-neutral-700 dark:from-neutral-300 dark:via-neutral-400 dark:to-neutral-600 bg-clip-text text-transparent whitespace-pre-line"
        >
          {rotatingPhrases[index]}
        </motion.span>
      </AnimatePresence>
    </span>
  )
}

export function Hero() {
  return (
    <section className="relative overflow-hidden pb-20 pt-24 min-h-[calc(100vh-4rem)]">
      {/* ambient background */}
      <div className="pointer-events-none absolute inset-0">
        <FloatingOrb
          className="absolute -right-32 -top-32 h-130 w-130 rounded-full bg-teal-500/[0.07] blur-[120px]"
          delay={0}
          duration={14}
        />
        <FloatingOrb
          className="absolute -left-24 top-1/4 h-100 w-100 rounded-full bg-sky-500/6 blur-[100px]"
          delay={2}
          duration={18}
        />
        <FloatingOrb
          className="absolute bottom-0 right-1/4 h-90 w-90 rounded-full bg-violet-500/4 blur-[100px]"
          delay={4}
          duration={16}
        />

        {/* subtle dot matrix */}
        <div
          className="absolute inset-0 opacity-[0.25]"
          style={{
            backgroundImage:
              'radial-gradient(circle, rgba(120,120,120,0.35) 1px, transparent 1px)',
            backgroundSize: '28px 28px',
            maskImage:
              'radial-gradient(ellipse 70% 60% at 50% 40%, black 20%, transparent 80%)',
          }}
        />
      </div>

      <div className="relative mx-auto max-w-6xl px-6">
        {/* main stack */}
        <div className="flex flex-col items-center gap-12 lg:gap-16">
          {/* header block */}
          <div className="text-center">
            <motion.div
              variants={fadeUp}
              initial="hidden"
              animate="visible"
              custom={1}
              className="mb-7 inline-flex items-center gap-3"
            >
              <div className="rounded-2xl">
                <Image
                  src="/appicon.png"
                  alt="mcpv"
                  width={46}
                  height={46}
                  priority
                />
              </div>
              <div className="text-left">
                <span className="block font-[family-name:var(--font-home-mono)] text-xs font-medium text-fd-foreground/80">
                  mcpv
                </span>
                <span className="block text-[11px] text-fd-muted-foreground">
                  MCP control plane
                </span>
              </div>
            </motion.div>

            <motion.h1
              variants={fadeUp}
              initial="hidden"
              animate="visible"
              custom={2}
              className="mx-auto max-w-3xl text-pretty font-[family-name:var(--font-home-display)] text-4xl font-semibold tracking-[-0.04em] text-fd-foreground sm:text-5xl md:text-[3.5rem] md:leading-[1.1] relative"
            >
              Your entire MCP stack,
              <br />
              <RotatingText />
            </motion.h1>

            <motion.p
              variants={fadeUp}
              initial="hidden"
              animate="visible"
              custom={3}
              className="mx-auto mt-6 max-w-lg text-pretty text-[15px] leading-relaxed text-fd-muted-foreground sm:text-base"
            >
              Elastic runtime, unified gateway, and live observability for every
              MCP server — managed from a desktop app. No terminal sprawl,
              no guesswork.
            </motion.p>

            {/* CTAs */}
            <motion.div
              variants={fadeUp}
              initial="hidden"
              animate="visible"
              custom={4}
              className="mt-10 mb-5 flex flex-col items-center gap-3 sm:flex-row sm:justify-center"
            >
              <Link
                href="https://github.com/wibus-wee/mcpv/releases"
                target="_blank"
                rel="noopener noreferrer"
                className="group relative inline-flex h-12 items-center justify-center gap-2.5 overflow-hidden rounded-2xl bg-fd-foreground px-7 text-sm font-medium text-fd-background transition-[box-shadow,filter] duration-200 hover:shadow-[0_8px_30px_-8px_rgba(20,184,166,0.4)] hover:brightness-105"
              >
                <span className="relative flex items-center gap-2.5">
                  <Download className="h-4 w-4" />
                  Download Desktop App
                  <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
                </span>
              </Link>
              <Link
                href="https://github.com/wibus-wee/mcpv"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex h-12 items-center justify-center gap-2 rounded-2xl border border-fd-border/60 bg-fd-background/80 px-6 text-sm font-medium text-fd-muted-foreground backdrop-blur-sm transition-[border-color,color] duration-200 hover:border-fd-border hover:text-fd-foreground"
              >
                <Github className="h-4 w-4" />
                Source Code
              </Link>
            </motion.div>

            <motion.p
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 0.5, duration: 0.6, ease }}
              className="text-xs text-fd-muted-foreground/60"
            >
              Available for macOS · Linux and Windows coming soon
            </motion.p>
          </div>

          {/* screenshot showcase */}
          <motion.div
            initial={{ opacity: 0, y: 28, scale: 0.97, filter: 'blur(12px)' }}
            animate={{ opacity: 1, y: 0, scale: 1, filter: 'blur(0px)' }}
            transition={{ delay: 0.55, duration: 0.9, ease }}
            className="group relative mx-auto w-full max-w-5xl"
          >
            {/* glow ring behind card */}
            <div className="absolute -inset-6 rounded-[40px] bg-gradient-to-br from-teal-500/[0.08] via-transparent to-sky-500/[0.06] blur-2xl" />

            <ScreenshotCard />
          </motion.div>
        </div>
      </div>
    </section>
  )
}
