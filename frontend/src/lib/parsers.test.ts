import { describe, expect, it } from 'vitest'

import {
  formatCommaSeparated,
  formatEnvironmentVariables,
  parseCommaSeparated,
  parseEnvironmentVariables,
} from './parsers'

describe('parsers', () => {
  describe('formatEnvironmentVariables', () => {
    it('converts empty object to empty string', () => {
      expect(formatEnvironmentVariables({})).toBe('')
    })

    it('converts single env var to single line', () => {
      expect(formatEnvironmentVariables({ KEY: 'value' })).toBe('KEY=value')
    })

    it('converts multiple env vars to multiline string', () => {
      const env = { KEY1: 'value1', KEY2: 'value2' }
      const result = formatEnvironmentVariables(env)
      expect(result).toBe('KEY1=value1\nKEY2=value2')
    })

    it('handles values with equals signs', () => {
      const env = { API_KEY: 'sk-123=456' }
      expect(formatEnvironmentVariables(env)).toBe('API_KEY=sk-123=456')
    })
  })

  describe('parseEnvironmentVariables', () => {
    it('parses empty string to empty object', () => {
      expect(parseEnvironmentVariables('')).toEqual({})
    })

    it('parses whitespace-only string to empty object', () => {
      expect(parseEnvironmentVariables('   \n\t  ')).toEqual({})
    })

    it('parses single line to single env var', () => {
      expect(parseEnvironmentVariables('KEY=value')).toEqual({ KEY: 'value' })
    })

    it('parses multiline string to multiple env vars', () => {
      const input = 'KEY1=value1\nKEY2=value2'
      expect(parseEnvironmentVariables(input)).toEqual({
        KEY1: 'value1',
        KEY2: 'value2',
      })
    })

    it('handles values with equals signs', () => {
      expect(parseEnvironmentVariables('API_KEY=sk-123=456')).toEqual({
        API_KEY: 'sk-123=456',
      })
    })

    it('trims whitespace from keys and values', () => {
      const input = '  KEY  =  value  \n  KEY2\t=\tvalue2\t'
      expect(parseEnvironmentVariables(input)).toEqual({
        KEY: 'value',
        KEY2: 'value2',
      })
    })

    it('ignores empty lines', () => {
      const input = 'KEY1=value1\n\nKEY2=value2\n'
      expect(parseEnvironmentVariables(input)).toEqual({
        KEY1: 'value1',
        KEY2: 'value2',
      })
    })

    it('ignores lines without equals sign', () => {
      const input = 'KEY1=value1\ninvalid line\nKEY2=value2'
      expect(parseEnvironmentVariables(input)).toEqual({
        KEY1: 'value1',
        KEY2: 'value2',
      })
    })

    it('handles empty values', () => {
      expect(parseEnvironmentVariables('KEY1=\nKEY2=value2')).toEqual({
        KEY1: '',
        KEY2: 'value2',
      })
    })
  })

  describe('formatCommaSeparated', () => {
    it('converts empty array to empty string', () => {
      expect(formatCommaSeparated([])).toBe('')
    })

    it('converts single item to single item', () => {
      expect(formatCommaSeparated(['item'])).toBe('item')
    })

    it('converts multiple items to comma-separated string', () => {
      expect(formatCommaSeparated(['item1', 'item2', 'item3'])).toBe('item1, item2, item3')
    })
  })

  describe('parseCommaSeparated', () => {
    it('parses empty string to empty array', () => {
      expect(parseCommaSeparated('')).toEqual([])
    })

    it('parses whitespace-only string to empty array', () => {
      expect(parseCommaSeparated('   \n\t  ')).toEqual([])
    })

    it('parses single item to single item array', () => {
      expect(parseCommaSeparated('item')).toEqual(['item'])
    })

    it('parses comma-separated string to array', () => {
      expect(parseCommaSeparated('item1, item2, item3')).toEqual(['item1', 'item2', 'item3'])
    })

    it('trims whitespace from items', () => {
      expect(parseCommaSeparated('  item1  ,  item2\t,\titem3  ')).toEqual(['item1', 'item2', 'item3'])
    })

    it('filters out empty items', () => {
      expect(parseCommaSeparated('item1,, item2,')).toEqual(['item1', 'item2'])
    })
  })

  describe('roundtrip conversion', () => {
    it('env vars roundtrip correctly', () => {
      const original = { KEY1: 'value1', KEY2: 'value=with=equals', KEY3: '' }
      const formatted = formatEnvironmentVariables(original)
      const parsed = parseEnvironmentVariables(formatted)
      expect(parsed).toEqual(original)
    })

    it('comma separated values roundtrip correctly', () => {
      const original = ['tag1', 'tag2', 'tag with spaces']
      const formatted = formatCommaSeparated(original)
      const parsed = parseCommaSeparated(formatted)
      expect(parsed).toEqual(original)
    })
  })
})
