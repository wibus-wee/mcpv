// Input: mcp-import.ts parseMcpServersJson function
// Output: Unit tests for MCP server JSON parsing
// Position: Test file for MCP import utilities

import { describe, expect, it } from 'vitest'

import { parseMcpServersJson } from './mcp-import'

describe('parseMcpServersJson', () => {
  describe('empty and invalid input', () => {
    it('returns error for empty string', () => {
      const result = parseMcpServersJson('')
      expect(result.servers).toEqual([])
      expect(result.errors).toContain('Paste JSON to continue.')
    })

    it('returns error for whitespace-only string', () => {
      const result = parseMcpServersJson('   \n\t  ')
      expect(result.servers).toEqual([])
      expect(result.errors).toContain('Paste JSON to continue.')
    })

    it('returns error for invalid JSON', () => {
      const result = parseMcpServersJson('{ invalid json }')
      expect(result.servers).toEqual([])
      expect(result.errors).toContain('Invalid JSON format.')
    })

    it('returns error for non-object JSON', () => {
      const result = parseMcpServersJson('"just a string"')
      expect(result.servers).toEqual([])
      expect(result.errors).toContain('JSON must be an object with mcpServers.')
    })

    it('returns error for array JSON', () => {
      const result = parseMcpServersJson('[]')
      expect(result.servers).toEqual([])
      expect(result.errors).toContain('JSON must be an object with mcpServers.')
    })
  })

  describe('mcpServers structure validation', () => {
    it('returns error when mcpServers is missing', () => {
      const result = parseMcpServersJson('{}')
      expect(result.servers).toEqual([])
      expect(result.errors).toContain('mcpServers must be an object map.')
    })

    it('returns error when mcpServers is not an object', () => {
      const result = parseMcpServersJson('{"mcpServers": "not an object"}')
      expect(result.servers).toEqual([])
      expect(result.errors).toContain('mcpServers must be an object map.')
    })

    it('returns error when mcpServers is an array', () => {
      const result = parseMcpServersJson('{"mcpServers": []}')
      expect(result.servers).toEqual([])
      expect(result.errors).toContain('mcpServers must be an object map.')
    })

    it('returns error for empty mcpServers object', () => {
      const result = parseMcpServersJson('{"mcpServers": {}}')
      expect(result.servers).toEqual([])
      expect(result.errors).toContain('No servers found in mcpServers.')
    })
  })

  describe('server entry validation', () => {
    it('returns error for empty server name', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          '': { command: 'node' },
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('server name is required')
    })

    it('returns error when entry is not an object', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: 'not an object',
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('entry must be an object')
    })

    it('returns error when command is missing', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: {},
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('command is required')
    })

    it('returns error when command is empty string', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: '   ' },
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('command is required')
    })

    it('returns error for non-stdio transport', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', transport: 'sse' },
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('only stdio transport is supported')
    })

    it('accepts stdio transport explicitly', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', transport: 'stdio' },
        },
      }))
      expect(result.servers).toHaveLength(1)
      expect(result.errors).toEqual([])
    })
  })

  describe('args validation', () => {
    it('returns error when args is not an array', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', args: 'not-an-array' },
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('args must be an array')
    })

    it('returns error when args contains non-string', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', args: ['valid', 123] },
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('args[1] must be a string')
    })

    it('accepts valid string args array', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', args: ['script.js', '--flag'] },
        },
      }))
      expect(result.servers).toHaveLength(1)
      expect(result.servers[0].cmd).toEqual(['node', 'script.js', '--flag'])
    })
  })

  describe('env validation', () => {
    it('returns error when env is not an object', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', env: 'not-an-object' },
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('env must be an object')
    })

    it('returns error when env value is not a string', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', env: { KEY: 123 } },
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('env.KEY must be a string')
    })

    it('accepts valid env object', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', env: { NODE_ENV: 'production' } },
        },
      }))
      expect(result.servers).toHaveLength(1)
      expect(result.servers[0].env).toEqual({ NODE_ENV: 'production' })
    })
  })

  describe('cwd validation', () => {
    it('returns error when cwd is not a string', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', cwd: 123 },
        },
      }))
      expect(result.servers).toEqual([])
      expect(result.errors[0]).toContain('cwd must be a string')
    })

    it('accepts valid cwd string', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          myServer: { command: 'node', cwd: '/path/to/dir' },
        },
      }))
      expect(result.servers).toHaveLength(1)
      expect(result.servers[0].cwd).toBe('/path/to/dir')
    })
  })

  describe('successful parsing', () => {
    it('parses minimal server config', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          simple: { command: 'node' },
        },
      }))

      expect(result.errors).toEqual([])
      expect(result.servers).toHaveLength(1)
      expect(result.servers[0]).toEqual({
        id: '0-simple',
        name: 'simple',
        cmd: ['node'],
        env: {},
        cwd: '',
      })
    })

    it('parses full server config', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          fullServer: {
            command: 'npx',
            args: ['-y', '@mcp/server'],
            env: { API_KEY: 'secret' },
            cwd: '/home/user/project',
          },
        },
      }))

      expect(result.errors).toEqual([])
      expect(result.servers).toHaveLength(1)
      expect(result.servers[0]).toEqual({
        id: '0-fullServer',
        name: 'fullServer',
        cmd: ['npx', '-y', '@mcp/server'],
        env: { API_KEY: 'secret' },
        cwd: '/home/user/project',
      })
    })

    it('parses multiple servers', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          server1: { command: 'node', args: ['server1.js'] },
          server2: { command: 'python', args: ['server2.py'] },
          server3: { command: 'go', args: ['run', 'server3.go'] },
        },
      }))

      expect(result.errors).toEqual([])
      expect(result.servers).toHaveLength(3)
      expect(result.servers.map(s => s.name)).toEqual(['server1', 'server2', 'server3'])
    })
  })

  describe('error aggregation', () => {
    it('collects multiple errors and returns no servers', () => {
      const result = parseMcpServersJson(JSON.stringify({
        mcpServers: {
          server1: { command: 'node', args: 'invalid' },
          server2: { command: 'python', env: 123 },
        },
      }))

      expect(result.servers).toEqual([])
      expect(result.errors.length).toBeGreaterThanOrEqual(2)
    })
  })
})
