<!-- Once this directory changes, update this README.md -->

# Profile Detail Components

Modular components for displaying and managing profile details in the config page.

This directory was refactored from a single 800+ line file into smaller, focused components.

## Architecture

```
profile-detail/
├── index.tsx           # Main entry point (ProfileDetailPanel)
├── profile-content.tsx # Main content layout (presentational)
├── subagent-section.tsx# SubAgent toggle accordion
├── server-item.tsx     # Individual server accordion item
├── detail-row.tsx      # Key-value display component
└── use-profile-actions.ts # Business logic hook
```

## Files

- **index.tsx**: Main `ProfileDetailPanel` component - orchestrates data fetching and state
- **profile-content.tsx**: Presentational component rendering profile details
- **subagent-section.tsx**: SubAgent toggle with local state management
- **server-item.tsx**: Individual server display with actions (toggle, delete)
- **detail-row.tsx**: Reusable key-value row component
- **use-profile-actions.ts**: Custom hook for profile/server actions with notifications

## Usage

```tsx
import { ProfileDetailPanel } from '@/modules/config/components/profile-detail'

<ProfileDetailPanel profileName={selectedProfile} />
```

## Design Decisions

1. **Separation of Concerns**: Business logic in hook, presentation in components
2. **Local State**: SubAgent section manages its own loading/error state
3. **Shared Types**: `ServerSpecWithKey` exported from server-item.tsx
4. **Consistent Patterns**: All sections use Accordion for expandable content
