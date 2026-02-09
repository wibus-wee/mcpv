'use client'

import {
  Activity,
  Boxes,
  Brain,
  Clock3,
  Command,
  FileText,
  Globe,
  Monitor,
  ShieldCheck,
  Zap,
} from 'lucide-react'
import { motion } from 'motion/react'

import { AnimateInView, StaggerChildren, staggerItem } from '@/components/landing/animate-in-view'

const features = [
  {
    icon: Zap,
    title: 'Elastic Runtime',
    description:
      'Start server instances on the first request and hibernate them after idle windows. Keep baseline resource usage close to zero.',
  },
  {
    icon: Globe,
    title: 'Unified Gateway',
    description:
      'Expose one MCP endpoint while preserving sticky sessions, concurrency boundaries, and policy-based server visibility.',
  },
  {
    icon: Activity,
    title: 'Observable Operations',
    description:
      'Instrument every lifecycle phase with Prometheus metrics and structured logs so latency regressions are visible immediately.',
  },
  {
    icon: Brain,
    title: 'Smart SubAgent',
    description:
      'Use Eino-based intent filtering to cut irrelevant tools and lower token burn during multi-server orchestration.',
  },
  {
    icon: Monitor,
    title: 'Desktop Control Surface',
    description:
      'Inspect logs, tools, and profiles in real time through a Wails app designed for daily runtime operations.',
  },
  {
    icon: FileText,
    title: 'Config as Source of Truth',
    description:
      'Model transport, activation, and profile visibility in one YAML catalog that can be reviewed and versioned with code.',
  },
]

const workflow = [
  {
    icon: Command,
    step: '01',
    title: 'Declare runtime contracts',
    description:
      'Define transport, startup command, tags, and lifecycle strategy in a profile-aware catalog.',
  },
  {
    icon: Boxes,
    step: '02',
    title: 'Route through one gateway',
    description:
      'Clients connect once. mcpv resolves server selection and capacity while maintaining isolation.',
  },
  {
    icon: Clock3,
    step: '03',
    title: 'Scale to traffic shape',
    description:
      'Burst under demand, keep warm where needed, and hibernate when no active sessions remain.',
  },
  {
    icon: ShieldCheck,
    step: '04',
    title: 'Operate with evidence',
    description:
      'Validate latency, error rate, and utilization trends from dashboards before promoting config changes.',
  },
]

export function Features() {
  return (
    <section className="relative py-24 sm:py-28">
      <div className="mx-auto max-w-6xl px-6">
        <AnimateInView>
          <div className="mb-10 flex items-end justify-between gap-8">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.2em] text-teal-600">
                Capabilities
              </p>
              <h2 className="mt-3 max-w-2xl font-[family-name:var(--font-home-display)] text-3xl font-semibold tracking-[-0.03em] text-fd-foreground sm:text-4xl">
                Designed for developers running MCP servers as serious infrastructure.
              </h2>
            </div>
          </div>
        </AnimateInView>

        <StaggerChildren
          className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
          staggerDelay={0.07}
          delay={0.15}
        >
          {features.map(feature => (
            <motion.article
              key={feature.title}
              variants={staggerItem}
              className="group rounded-2xl border border-fd-border/70 bg-fd-card/55 p-6 transition-[border-color,background-color,translate] duration-200 hover:-translate-y-0.5 hover:border-fd-border hover:bg-fd-card/90"
            >
              <feature.icon
                className="mb-4 h-5 w-5 text-fd-muted-foreground transition-colors group-hover:text-teal-600"
                strokeWidth={1.7}
              />
              <h3 className="text-base font-semibold text-fd-foreground">
                {feature.title}
              </h3>
              <p className="mt-2 text-sm leading-relaxed text-fd-muted-foreground">
                {feature.description}
              </p>
            </motion.article>
          ))}
        </StaggerChildren>
      </div>
    </section>
  )
}
