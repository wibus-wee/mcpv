// Input: None (pure configuration)
// Output: Centralized application configuration constants
// Position: Configuration layer - single source of truth for app-wide settings

/**
 * Application-wide configuration constants.
 * Centralized to ensure consistency and easy maintenance.
 */
export const appConfig = {
  /**
   * Logging configuration
   */
  logs: {
    /** Maximum number of log entries to keep in memory */
    maxEntries: 1000,
    /** Height of each log row in pixels (for virtual list) */
    rowHeight: 28,
    /** Number of extra items to render outside viewport */
    overscanCount: 12,
  },

  /**
   * Animation configuration
   */
  animation: {
    /** Default animation duration in seconds */
    duration: 0.3,
    /** Stagger delay between animated items */
    staggerDelay: 0.02,
    /** Duration for quick micro-interactions */
    micro: 0.15,
    /** Duration for slower page transitions */
    page: 0.4,
  },

  /**
   * Polling intervals (in milliseconds)
   */
  polling: {
    /** Core state polling interval */
    coreState: 5000,
    /** Default deduping interval for cached requests */
    dedupingDefault: 10000,
    /** Fast deduping interval for more responsive data */
    dedupingFast: 5000,
  },

  /**
   * UI configuration
   */
  ui: {
    /** Default debounce delay for search inputs */
    searchDebounce: 300,
    /** Toast notification duration */
    toastDuration: 3000,
    /** Sidebar width in pixels */
    sidebarWidth: 240,
  },

  /**
   * Feature flags (for gradual rollout)
   */
  features: {
    /** Enable SubAgent functionality */
    subAgentEnabled: true,
    /** Enable verbose logging in development */
    verboseLogging: import.meta.env.DEV,
  },
} as const

/**
 * Type-safe accessor for nested config values
 */
export type AppConfig = typeof appConfig
