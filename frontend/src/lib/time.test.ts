// Input: time.ts utility functions
// Output: Unit tests for formatDuration, formatLatency, getElapsedMs
// Position: Test file for time utilities

import { describe, expect, it, vi } from 'vitest'

import { formatDuration, formatLatency, getElapsedMs } from './time'

describe('formatDuration', () => {
  it('returns "0s" for zero or negative values', () => {
    expect(formatDuration(0)).toBe('0s')
    expect(formatDuration(-100)).toBe('0s')
    expect(formatDuration(-1000)).toBe('0s')
  })

  it('formats seconds only (< 60s)', () => {
    expect(formatDuration(1000)).toBe('1s')
    expect(formatDuration(30000)).toBe('30s')
    expect(formatDuration(59000)).toBe('59s')
    expect(formatDuration(59999)).toBe('59s') // floors to 59s
  })

  it('formats minutes and seconds (< 1 hour)', () => {
    expect(formatDuration(60000)).toBe('1m 0s')
    expect(formatDuration(90000)).toBe('1m 30s')
    expect(formatDuration(3599000)).toBe('59m 59s')
  })

  it('formats hours and minutes (< 1 day)', () => {
    expect(formatDuration(3600000)).toBe('1h 0m')
    expect(formatDuration(5400000)).toBe('1h 30m')
    expect(formatDuration(86399000)).toBe('23h 59m')
  })

  it('formats days and hours (>= 1 day)', () => {
    expect(formatDuration(86400000)).toBe('1d 0h')
    expect(formatDuration(90000000)).toBe('1d 1h')
    expect(formatDuration(172800000)).toBe('2d 0h')
    expect(formatDuration(259200000 + 3600000 * 5)).toBe('3d 5h')
  })
})

describe('formatLatency', () => {
  it('formats milliseconds (< 1s)', () => {
    expect(formatLatency(0)).toBe('0ms')
    expect(formatLatency(1)).toBe('1ms')
    expect(formatLatency(100)).toBe('100ms')
    expect(formatLatency(999)).toBe('999ms')
    expect(formatLatency(500.6)).toBe('501ms') // rounds
  })

  it('formats seconds with one decimal (< 60s)', () => {
    expect(formatLatency(1000)).toBe('1.0s')
    expect(formatLatency(1500)).toBe('1.5s')
    expect(formatLatency(59999)).toBe('60.0s') // edge case
  })

  it('formats minutes and seconds (>= 60s)', () => {
    expect(formatLatency(60000)).toBe('1m 0s')
    expect(formatLatency(90000)).toBe('1m 30s')
    expect(formatLatency(125000)).toBe('2m 5s')
  })
})

describe('getElapsedMs', () => {
  it('returns null for undefined or null input', () => {
    expect(getElapsedMs()).toBe(null)
    expect(getElapsedMs(null)).toBe(null)
  })

  it('returns null for invalid timestamp strings', () => {
    expect(getElapsedMs('')).toBe(null)
    expect(getElapsedMs('invalid')).toBe(null)
    expect(getElapsedMs('not-a-date')).toBe(null)
  })

  it('returns elapsed milliseconds for valid timestamps', () => {
    const now = Date.now()
    vi.setSystemTime(now)

    // 10 seconds ago
    const tenSecondsAgo = new Date(now - 10000).toISOString()
    expect(getElapsedMs(tenSecondsAgo)).toBe(10000)

    // 1 minute ago
    const oneMinuteAgo = new Date(now - 60000).toISOString()
    expect(getElapsedMs(oneMinuteAgo)).toBe(60000)

    vi.useRealTimers()
  })

  it('returns 0 for future timestamps (clamped)', () => {
    const now = Date.now()
    vi.setSystemTime(now)

    const future = new Date(now + 10000).toISOString()
    expect(getElapsedMs(future)).toBe(0)

    vi.useRealTimers()
  })
})
