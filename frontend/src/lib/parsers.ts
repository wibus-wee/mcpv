/**
 * Utility functions for parsing and formatting structured data to/from UI strings
 */

/**
 * Converts environment variables object to a multiline string format
 * Format: KEY1=value1\nKEY2=value2
 */
export function formatEnvironmentVariables(env: Record<string, string>): string {
  if (!env || Object.keys(env).length === 0) {
    return ''
  }

  return Object.entries(env)
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

/**
 * Parses a multiline string of environment variables back to an object
 * Expected format: KEY1=value1\nKEY2=value2
 * Handles empty lines and trims whitespace
 */
export function parseEnvironmentVariables(value: string): Record<string, string> {
  if (!value.trim()) {
    return {}
  }

  const entries = value
    .split('\n')
    .map(line => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [key, ...rest] = line.split('=')
      return { key: key?.trim(), value: rest.join('=').trim(), originalLine: line }
    })
    .filter(({ key, originalLine }) => Boolean(key) && originalLine.includes('='))
    .map(({ key, value }) => [key, value] as const)

  return entries.reduce<Record<string, string>>((acc, [key, value]) => {
    if (key) {
      acc[key] = value ?? ''
    }
    return acc
  }, {})
}

/**
 * Converts an array of strings to a comma-separated string
 */
export function formatCommaSeparated(values: string[]): string {
  return values.join(', ')
}

/**
 * Parses a comma-separated string back to an array of strings
 * Handles empty values and trims whitespace
 */
export function parseCommaSeparated(value: string): string[] {
  return value
    .split(',')
    .map(item => item.trim())
    .filter(Boolean)
}
