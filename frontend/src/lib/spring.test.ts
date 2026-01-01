// Input: spring.ts Spring class
// Output: Unit tests for animation spring presets
// Position: Test file for spring animation utilities

import { describe, expect, it } from 'vitest'

import { Spring } from './spring'

describe('Spring', () => {
  describe('presets', () => {
    it('has smooth preset with no bounce', () => {
      expect(Spring.presets.smooth).toEqual({
        type: 'spring',
        duration: 0.4,
        bounce: 0,
      })
    })

    it('has snappy preset with small bounce', () => {
      expect(Spring.presets.snappy).toEqual({
        type: 'spring',
        duration: 0.4,
        bounce: 0.15,
      })
    })

    it('has bouncy preset with higher bounce', () => {
      expect(Spring.presets.bouncy).toEqual({
        type: 'spring',
        duration: 0.4,
        bounce: 0.3,
      })
    })
  })

  describe('smooth()', () => {
    it('returns default smooth spring', () => {
      expect(Spring.smooth()).toEqual({
        type: 'spring',
        duration: 0.4,
        bounce: 0,
      })
    })

    it('accepts custom duration', () => {
      expect(Spring.smooth(0.6)).toEqual({
        type: 'spring',
        duration: 0.6,
        bounce: 0,
      })
    })

    it('accepts custom duration and extra bounce', () => {
      expect(Spring.smooth(0.5, 0.1)).toEqual({
        type: 'spring',
        duration: 0.5,
        bounce: 0.1,
      })
    })
  })

  describe('snappy()', () => {
    it('returns default snappy spring', () => {
      expect(Spring.snappy()).toEqual({
        type: 'spring',
        duration: 0.4,
        bounce: 0.15,
      })
    })

    it('accepts custom duration', () => {
      expect(Spring.snappy(0.3)).toEqual({
        type: 'spring',
        duration: 0.3,
        bounce: 0.15,
      })
    })

    it('adds extra bounce to base bounce', () => {
      expect(Spring.snappy(0.4, 0.05)).toEqual({
        type: 'spring',
        duration: 0.4,
        bounce: 0.2, // 0.15 + 0.05
      })
    })
  })

  describe('bouncy()', () => {
    it('returns default bouncy spring', () => {
      expect(Spring.bouncy()).toEqual({
        type: 'spring',
        duration: 0.4,
        bounce: 0.3,
      })
    })

    it('accepts custom duration', () => {
      expect(Spring.bouncy(0.5)).toEqual({
        type: 'spring',
        duration: 0.5,
        bounce: 0.3,
      })
    })

    it('adds extra bounce to base bounce', () => {
      expect(Spring.bouncy(0.4, 0.1)).toEqual({
        type: 'spring',
        duration: 0.4,
        bounce: 0.4, // 0.3 + 0.1
      })
    })
  })
})
