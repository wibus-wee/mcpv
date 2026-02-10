'use client'

import { ArrowRight, Code2, Github } from 'lucide-react'
import { motion } from 'motion/react'
import Link from 'next/link'

import { StaggerChildren, staggerItem } from '@/components/landing/animate-in-view'

export function CTA() {
  return (
    <section className="relative py-24 sm:py-28">
      <div className="mx-auto max-w-6xl px-6">
        <div className="relative overflow-hidden rounded-3xl border border-fd-border/70 bg-fd-card/65 p-10 sm:p-12 lg:p-16">
          <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(70%_70%_at_85%_10%,rgba(56,189,248,0.16),transparent_78%)]" />
          <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(55%_55%_at_15%_90%,rgba(20,184,166,0.14),transparent_75%)]" />

          <StaggerChildren
            className="relative text-center"
            staggerDelay={0.1}
            delay={0}
            threshold={0.2}
          >
            <motion.p
              variants={staggerItem}
              className="text-xs font-semibold uppercase tracking-[0.2em] text-fd-muted-foreground"
            >
              Get started
            </motion.p>
            <motion.h2
              variants={staggerItem}
              className="mx-auto mt-4 max-w-2xl font-[family-name:var(--font-home-display)] text-3xl font-semibold tracking-[-0.03em] text-fd-foreground sm:text-4xl lg:text-5xl"
            >
              Ready to streamline your MCP operations?
            </motion.h2>
            <motion.p
              variants={staggerItem}
              className="mx-auto mt-5 max-w-xl text-sm leading-relaxed text-fd-muted-foreground sm:text-base"
            >
              Download the desktop app or explore the documentation to get started with elastic runtime management.
            </motion.p>

            <motion.div
              variants={staggerItem}
              className="mt-10 flex flex-col items-center justify-center gap-3 sm:flex-row"
            >
              <Link
                href="https://github.com/wibus-wee/mcpv/releases"
                target="_blank"
                rel="noopener noreferrer"
                className="group inline-flex h-12 items-center justify-center gap-2 rounded-xl bg-fd-foreground px-7 text-sm font-medium text-fd-background transition-colors hover:bg-fd-foreground/90"
              >
                <Github className="h-4 w-4" />
                Download from GitHub
                <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
              </Link>
              <Link
                href="https://github.com/wibus-wee/mcpv"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex h-12 items-center justify-center gap-2 rounded-xl border border-fd-border bg-fd-background/80 px-7 text-sm font-medium text-fd-muted-foreground transition-colors hover:text-fd-foreground"
              >
                <Code2 className="h-4 w-4" />
                View Source Code
              </Link>
            </motion.div>

            <motion.p
              variants={staggerItem}
              className="mt-8 text-xs text-fd-muted-foreground"
            >
              Currently available for macOS · Linux and Windows support coming soon
            </motion.p>
          </StaggerChildren>
        </div>
      </div>
    </section>
  )
}
