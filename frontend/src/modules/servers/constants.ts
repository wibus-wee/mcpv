// Server tab definitions for consistent usage across components and routing
export const SERVER_TABS = ['overview', 'tools', 'configuration'] as const

export type ServerTab = typeof SERVER_TABS[number]
