// Input: SVG primitives, motion animation
// Output: Sparkline, MiniGauge visualization components
// Position: Reusable dashboard visualization primitives

import { m, useSpring, useTransform } from 'motion/react'
import { useEffect, useMemo, useRef } from 'react'

import { cn } from '@/lib/utils'

interface SparklineProps {
  data: number[]
  width?: number
  height?: number
  className?: string
  strokeColor?: string
  fillColor?: string
  showDots?: boolean
}

export function Sparkline({
  data,
  width = 80,
  height = 24,
  className,
  strokeColor = 'currentColor',
  fillColor,
  showDots = false,
}: SparklineProps) {
  const pathData = useMemo(() => {
    if (data.length < 2) return ''

    const min = Math.min(...data)
    const max = Math.max(...data)
    const range = max - min || 1
    const padding = 2
    const effectiveHeight = height - padding * 2
    const effectiveWidth = width - padding * 2

    const points = data.map((value, i) => {
      const x = padding + (i / (data.length - 1)) * effectiveWidth
      const y = padding + effectiveHeight - ((value - min) / range) * effectiveHeight
      return { x, y }
    })

    const linePath = points.map((p, i) =>
      i === 0 ? `M ${p.x} ${p.y}` : `L ${p.x} ${p.y}`,
    ).join(' ')

    return linePath
  }, [data, width, height])

  const areaPath = useMemo(() => {
    if (data.length < 2 || !fillColor) return ''

    const min = Math.min(...data)
    const max = Math.max(...data)
    const range = max - min || 1
    const padding = 2
    const effectiveHeight = height - padding * 2
    const effectiveWidth = width - padding * 2

    const points = data.map((value, i) => {
      const x = padding + (i / (data.length - 1)) * effectiveWidth
      const y = padding + effectiveHeight - ((value - min) / range) * effectiveHeight
      return { x, y }
    })

    const linePath = points.map((p, i) =>
      i === 0 ? `M ${p.x} ${p.y}` : `L ${p.x} ${p.y}`,
    ).join(' ')

    return `${linePath} L ${points.at(-1)?.x} ${height - padding} L ${padding} ${height - padding} Z`
  }, [data, width, height, fillColor])

  if (data.length < 2) {
    return (
      <svg width={width} height={height} className={cn('text-muted-foreground/30', className)}>
        <line x1={2} y1={height / 2} x2={width - 2} y2={height / 2} stroke="currentColor" strokeWidth={1} strokeDasharray="2 2" />
      </svg>
    )
  }

  return (
    <svg width={width} height={height} className={className}>
      {fillColor && (
        <m.path
          d={areaPath}
          fill={fillColor}
          initial={{ opacity: 0 }}
          animate={{ opacity: 0.15 }}
          transition={{ duration: 0.3 }}
        />
      )}
      <m.path
        d={pathData}
        fill="none"
        stroke={strokeColor}
        strokeWidth={1.5}
        strokeLinecap="round"
        strokeLinejoin="round"
        initial={{ pathLength: 0, opacity: 0 }}
        animate={{ pathLength: 1, opacity: 1 }}
        transition={{ duration: 0.5, ease: 'easeOut' }}
      />
      {showDots && data.length > 0 && (
        <m.circle
          cx={width - 2}
          cy={(() => {
            const min = Math.min(...data)
            const max = Math.max(...data)
            const range = max - min || 1
            const padding = 2
            const effectiveHeight = height - padding * 2
            return padding + effectiveHeight - ((data.at(-1) ?? 0 - min) / range) * effectiveHeight
          })()}
          r={2}
          fill={strokeColor}
          initial={{ scale: 0 }}
          animate={{ scale: 1 }}
          transition={{ delay: 0.4 }}
        />
      )}
    </svg>
  )
}

interface MiniGaugeProps {
  value: number
  max?: number
  size?: number
  strokeWidth?: number
  className?: string
  showValue?: boolean
  thresholds?: { warning: number, critical: number }
}

export function MiniGauge({
  value,
  max = 100,
  size = 40,
  strokeWidth = 4,
  className,
  showValue = true,
  thresholds = { warning: 60, critical: 85 },
}: MiniGaugeProps) {
  const percentage = Math.min(100, Math.max(0, (value / max) * 100))
  const radius = (size - strokeWidth) / 2
  const circumference = 2 * Math.PI * radius

  const animatedProgress = useSpring(0, { duration: 800 })
  const strokeDashoffset = useTransform(
    animatedProgress,
    [0, 100],
    [circumference, 0],
  )

  useEffect(() => {
    animatedProgress.set(percentage)
  }, [percentage, animatedProgress])

  const getColor = () => {
    if (percentage >= thresholds.critical) return 'text-red-500'
    if (percentage >= thresholds.warning) return 'text-amber-500'
    return 'text-emerald-500'
  }

  return (
    <div className={cn('relative inline-flex items-center justify-center', className)}>
      <svg width={size} height={size} className="-rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          className="text-muted/20"
        />
        <m.circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          style={{ strokeDashoffset }}
          className={getColor()}
        />
      </svg>
      {showValue && (
        <span className="absolute text-[10px] font-medium tabular-nums">
          {Math.round(percentage)}%
        </span>
      )}
    </div>
  )
}

interface StackedBarProps {
  segments: Array<{
    value: number
    color: string
    label: string
  }>
  height?: number
  className?: string
}

export function StackedBar({ segments, height = 8, className }: StackedBarProps) {
  const total = segments.reduce((sum, s) => sum + s.value, 0)

  if (total === 0) {
    return (
      <div
        className={cn('w-full rounded-full bg-muted/30', className)}
        style={{ height }}
      />
    )
  }

  return (
    <div
      className={cn('flex w-full overflow-hidden rounded-full', className)}
      style={{ height }}
    >
      {segments.map((segment) => {
        const width = (segment.value / total) * 100
        if (width === 0) return null
        return (
          <div
            key={segment.label}
            className={segment.color}
            style={{ width: `${width}%`, height }}
            title={`${segment.label}: ${segment.value}`}
          />
        )
      })}
    </div>
  )
}

interface AnimatedNumberProps {
  value: number
  className?: string
  format?: (n: number) => string
}

export function AnimatedNumber({
  value,
  className,
  format = n => n.toLocaleString(),
}: AnimatedNumberProps) {
  const spring = useSpring(value, { duration: 500 })
  const display = useTransform(spring, v => format(Math.round(v)))
  const ref = useRef<HTMLSpanElement>(null)

  useEffect(() => {
    spring.set(value)
  }, [value, spring])

  useEffect(() => {
    const unsubscribe = display.on('change', (v) => {
      if (ref.current) ref.current.textContent = v
    })
    return unsubscribe
  }, [display])

  return <span ref={ref} className={cn('tabular-nums', className)}>{format(value)}</span>
}

interface TrendIndicatorProps {
  current: number
  previous: number
  className?: string
  showPercentage?: boolean
}

export function TrendIndicator({
  current,
  previous,
  className,
  showPercentage = true,
}: TrendIndicatorProps) {
  if (previous === 0 && current === 0) return null

  const diff = current - previous
  const percentage = previous !== 0 ? ((diff / previous) * 100) : (current > 0 ? 100 : 0)

  if (Math.abs(diff) < 0.01) return null

  const isPositive = diff > 0
  const isNegative = diff < 0

  return (
    <span className={cn(
      'inline-flex items-center gap-0.5 text-xs font-medium',
      {
        'text-emerald-500': isPositive,
        'text-red-500': isNegative,
        'text-muted-foreground': !isPositive && !isNegative,
      },
      className,
    )}
    >
      {isPositive && '↑'}
      {isNegative && '↓'}
      {showPercentage && (
        <span>{Math.abs(percentage).toFixed(0)}%</span>
      )}
    </span>
  )
}
