'use client'

import type { Easing, Variants } from 'motion/react'
import { motion, useInView } from 'motion/react'
import type { ReactNode } from 'react'
import { useRef } from 'react'

export const smoothEase: Easing = [0.16, 1, 0.3, 1]

const defaultVariants: Variants = {
  hidden: { opacity: 0, y: 24, filter: 'blur(12px)' },
  visible: {
    opacity: 1,
    y: 0,
    filter: 'blur(0px)',
    transition: { duration: 0.7, ease: smoothEase },
  },
}

export const staggerItem: Variants = {
  hidden: { opacity: 0, y: 20, filter: 'blur(10px)' },
  visible: {
    opacity: 1,
    y: 0,
    filter: 'blur(0px)',
    transition: { duration: 0.6, ease: smoothEase },
  },
}

interface AnimateInViewProps {
  children: ReactNode
  delay?: number
  className?: string
  as?: keyof typeof motion
  variants?: Variants
  threshold?: number
}

export function AnimateInView({
  children,
  delay = 0,
  className,
  as = 'div',
  variants = defaultVariants,
  threshold = 0.18,
}: AnimateInViewProps) {
  const ref = useRef<HTMLDivElement>(null)
  const inView = useInView(ref, { once: true, amount: threshold })

  const Component = motion[as] as typeof motion.div

  return (
    <Component
      ref={ref}
      initial="hidden"
      animate={inView ? 'visible' : 'hidden'}
      variants={variants}
      transition={delay ? { delay } : undefined}
      className={className}
    >
      {children}
    </Component>
  )
}

interface StaggerChildrenProps {
  children: ReactNode
  className?: string
  staggerDelay?: number
  delay?: number
  threshold?: number
}

const staggerContainer = (
  staggerDelay: number,
  delay: number,
): Variants => ({
  hidden: {},
  visible: {
    transition: {
      delayChildren: delay,
      staggerChildren: staggerDelay,
    },
  },
})

export function StaggerChildren({
  children,
  className,
  staggerDelay = 0.08,
  delay = 0,
  threshold = 0.1,
}: StaggerChildrenProps) {
  const ref = useRef<HTMLDivElement>(null)
  const inView = useInView(ref, { once: true, amount: threshold })

  return (
    <motion.div
      ref={ref}
      initial="hidden"
      animate={inView ? 'visible' : 'hidden'}
      variants={staggerContainer(staggerDelay, delay)}
      className={className}
    >
      {children}
    </motion.div>
  )
}
