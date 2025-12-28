<!-- Once this directory changes, update this README.md -->

# Modules/Dashboard

Dashboard feature module displaying core status and system overview.
Contains all dashboard-specific components and logic.
This is the main landing page after app launch.

## Files

- **dashboard-page.tsx**: Main dashboard page component with header, tabs, and content sections
- **hooks.ts**: SWR-based data fetching hooks (useAppInfo, useCoreState, useTools, useResources, usePrompts)

## Components

- **components/status-cards.tsx**: Status overview cards showing core status, uptime, tools/resources/prompts counts
- **components/tools-table.tsx**: Searchable table of available MCP tools with detail dialog
- **components/resources-list.tsx**: Collapsible list of available MCP resources
- **components/logs-panel.tsx**: Real-time logs panel with filtering and auto-scroll
- **components/settings-sheet.tsx**: Settings sheet with theme, refresh interval, notifications, log level options
- **components/index.ts**: Barrel export for all dashboard components
