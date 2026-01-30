# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the frontend for the mcpv Wails application, a cross-platform GUI for the MCP (Model Context Protocol) server orchestration core. The frontend is built as a modern React SPA that interfaces with the Go backend through Wails v3 bindings.

## Tech Stack

- **Framework**: React 19 with TypeScript
- **Build Tool**: Vite 7
- **Router**: TanStack Router (file-based routing)
- **State Management**: Jotai (atomic state management)
- **Styling**: Tailwind CSS v4
- **Animation**: Motion (Framer Motion fork) with LazyMotion
- **Theming**: next-themes
- **Desktop Runtime**: Wails v3 (@wailsio/runtime)
- **Testing**: Vitest + React Testing Library
- **Linting**: ESLint (hyoban config)

## Development Commands

**Package Manager**: This project uses `pnpm`. Do NOT use npm or yarn.

```bash
# Install dependencies
pnpm install

# Development server (standalone, port 3000)
pnpm dev

# Development with Wails (from project root)
make wails-dev

# Build for production
pnpm build

# Build for development mode
pnpm build:dev

# Preview production build
pnpm preview

# Run tests
pnpm test

# Lint code
pnpm lint
```

## Project Structure

```
frontend/
├── src/
│   ├── routes/          # TanStack Router file-based routes
│   │   ├── __root.tsx   # Root layout with Outlet
│   │   └── index.tsx    # Home page route
│   ├── components/
│   │   ├── ui/          # Universal base UI components (buttons, inputs, etc.)
│   │   └── common/      # App-specific shared components
│   ├── modules/         # Feature-specific components by domain
│   ├── atoms/           # Jotai atoms for state management
│   ├── lib/             # Utility functions and helpers
│   │   ├── utils.ts     # cn() utility for class merging
│   │   ├── jotai.ts     # jotaiStore and createAtomHooks
│   │   ├── spring.ts    # Animation spring presets
│   │   └── framer-lazy-feature.ts  # LazyMotion features
│   ├── providers/       # React context providers
│   │   └── root-provider.tsx  # Global providers (Jotai, Theme, LazyMotion)
│   ├── main.tsx         # App entry point
│   └── styles.css       # Global styles (Tailwind imports)
├── bindings/            # Auto-generated Wails Go bindings (DO NOT EDIT)
├── public/              # Static assets
├── dist/                # Build output
├── vite.config.ts       # Vite configuration
├── tsconfig.json        # TypeScript configuration
└── eslint.config.mjs    # ESLint configuration
```

## CRITICAL RULES

### 1. Animation with Motion - ALWAYS Use `m.` Prefix

**This project uses LazyMotion for optimized bundle size. You MUST use `m.` prefix instead of `motion.`:**

```tsx
// ✅ CORRECT - Use m. prefix
import { m } from 'motion/react'

function AnimatedComponent() {
  return (
    <m.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.3 }}
    >
      Content
    </m.div>
  )
}

// ❌ WRONG - Never use motion. prefix
import { motion } from 'motion/react'  // This breaks LazyMotion!
```

### 2. Styling - NO Dynamic Tailwind Classes

**All Tailwind classes MUST be statically defined. Never construct class names dynamically:**

```tsx
// ❌ WRONG - Dynamic class construction
const size = 'large'
const className = `text-${size}`  // Won't work with Tailwind purging!

const color = 'blue'
const className = `bg-${color}-500`  // Won't work!

// ✅ CORRECT - Static classes with conditional logic
import { cn } from '@/lib/utils'

const className = cn({
  'text-base': size === 'small',
  'text-lg': size === 'medium',
  'text-xl': size === 'large',
})

// ✅ CORRECT - Predefined class mappings
const sizeClasses = {
  small: 'text-base',
  medium: 'text-lg',
  large: 'text-xl',
}
const className = sizeClasses[size]
```

**Always use the `cn()` utility from `@/lib/utils` for combining classes:**

```tsx
import { cn } from '@/lib/utils'

function Button({ className, variant = 'primary', ...props }) {
  return (
    <button
      className={cn(
        // Base styles
        'px-4 py-2 rounded-md font-medium transition-colors',
        // Variant styles
        {
          'bg-primary text-white hover:bg-primary/90': variant === 'primary',
          'bg-secondary text-secondary-foreground': variant === 'secondary',
        },
        // External className override
        className
      )}
      {...props}
    />
  )
}
```

### 3. Component Organization - Domain-Based Structure

**Components are organized by reusability and domain:**

- **`components/ui/`** - Universal base UI components
  - Reusable primitives (buttons, inputs, modals)
  - Can be used in any React application
  - Pure UI components without business logic
  - Examples: `Button`, `Input`, `Select`, `Tooltip`

- **`components/common/`** - App-specific shared components
  - Used across multiple features but specific to this app
  - Contains app-specific logic
  - Examples: `ErrorElement`, `Footer`, `AppHeader`

- **`modules/{domain}/`** - Feature-specific components
  - Components specific to a business domain/feature
  - Contains domain-specific logic or data handling
  - Examples: `modules/feed/`, `modules/auth/`, `modules/user/`

**Placement rule**: If a component is specific to a business domain/feature, place it in the corresponding module directory.

### 4. State Management - Jotai Patterns

**Use the global `jotaiStore` and custom utilities:**

```tsx
// In atoms/user.ts
import { atom } from 'jotai'
import { createAtomHooks } from '@/lib/jotai'

export const userAtom = atom(null)
export const isLoggedInAtom = atom((get) => get(userAtom) !== null)

// Using createAtomHooks for consistent patterns
const [useMyAtom, useSetMyAtom, useMyAtomValue, myAtom] = createAtomHooks(atom(null))
```

**Reading state:**
```tsx
import { useAtomValue } from 'jotai'
import { userAtom } from '@/atoms/user'

function UserProfile() {
  const user = useAtomValue(userAtom)
  return <div>{user?.name}</div>
}
```

**Writing state:**
```tsx
import { useSetAtom } from 'jotai'
import { userAtom } from '@/atoms/user'

function LoginForm() {
  const setUser = useSetAtom(userAtom)
  const handleLogin = (userData) => setUser(userData)
}
```

**Reading and writing:**
```tsx
import { useAtom } from 'jotai'
import { userAtom } from '@/atoms/user'

function UserSettings() {
  const [user, setUser] = useAtom(userAtom)
  const updateUser = (updates) => setUser({ ...user, ...updates })
}
```

**The global Jotai provider is configured in `providers/root-provider.tsx`.**

## TypeScript Configuration

- **Strict mode enabled**: All strict TypeScript checks are on
- **Module resolution**: `bundler` mode (Vite-optimized)
- **Path alias**: `@/*` maps to `./src/*`
- **Target**: ES2022
- **JSX**: `react-jsx` (automatic React 19 transform)
- **No emitting**: TypeScript used for type checking only; Vite handles transpilation

## Testing

- **Framework**: Vitest (Vite-native test runner)
- **Utilities**: React Testing Library (@testing-library/react)
- **Environment**: jsdom
- Run tests with `pnpm test`
- Test files use `.test.ts(x)` or `.spec.ts(x)` naming

## Code Style & Linting

- **Linting**: ESLint with `eslint-config-hyoban`
- **Formatting**: Prettier (auto-formats on save)
- **Self-closing JSX**: Enforced via ESLint (`@stylistic/jsx-self-closing-comp`)
- Use standard React/TypeScript conventions:
  - PascalCase for components and types
  - camelCase for functions and variables
  - Prefer named exports over default exports (except for route files)
- Ignored paths: `**/api-gen/**`, `components/ui/**` (auto-generated or third-party)

## Important Conventions

1. **Never use npm or yarn**: This project uses `pnpm` exclusively
2. **Import path alias**: Always use `@/` for imports from `src/` (e.g., `import { cn } from '@/lib/utils'`)
3. **Bindings are auto-generated**: Do NOT manually edit files in `bindings/` directory
4. **Route files**: Let TanStack Router generate route files; edit only for custom logic
5. **Strict port**: Dev server uses port 9245 (strictPort: true) for Wails integration
6. **Motion animations**: ALWAYS use `m.` prefix, never `motion.`
7. **Tailwind classes**: NO dynamic class construction - use static classes only
8. **Component placement**: Follow the domain-based structure (ui/common/modules)

## Development Workflow

### Working on Frontend Only

```bash
cd frontend
pnpm dev
```

This runs the frontend in isolation on port 3000 (useful for UI development without Wails).

### Working with Full Application

From project root:

```bash
make wails-dev
```

This starts both the Go backend and frontend in hot-reload mode with Wails integration.

### Regenerating Go Bindings

When Go backend services change:

```bash
# From project root
make wails-bindings
```

This updates TypeScript bindings in `frontend/bindings/`.

### Adding a New Route

1. Create a new file in `src/routes/` (e.g., `about.tsx`)
2. TanStack Router automatically generates route configuration
3. Use the route file template:

```tsx
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/about')({
  component: AboutComponent,
})

function AboutComponent() {
  return <div>About Page</div>
}
```

### Creating an Animated Component

```tsx
import { m } from 'motion/react'
import { cn } from '@/lib/utils'

function AnimatedCard({ className, children, ...props }) {
  return (
    <m.div
      className={cn(
        'p-4 rounded-lg bg-background border border-border',
        className
      )}
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
      whileHover={{ scale: 1.02 }}
      {...props}
    >
      {children}
    </m.div>
  )
}
```

### Creating a Styled Component with Variants

```tsx
import { cn } from '@/lib/utils'

interface ButtonProps {
  variant?: 'primary' | 'secondary' | 'outline'
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

function Button({ variant = 'primary', size = 'md', className, ...props }: ButtonProps) {
  return (
    <button
      className={cn(
        // Base styles
        'inline-flex items-center justify-center rounded-md font-medium',
        'transition-colors focus-visible:outline-none focus-visible:ring-2',
        'disabled:pointer-events-none disabled:opacity-50',

        // Size variants
        {
          'h-8 px-3 text-sm': size === 'sm',
          'h-10 px-4': size === 'md',
          'h-12 px-6 text-lg': size === 'lg',
        },

        // Color variants
        {
          'bg-primary text-white hover:bg-primary/90': variant === 'primary',
          'bg-secondary text-secondary-foreground hover:bg-secondary/80': variant === 'secondary',
          'border border-border bg-background hover:bg-fill': variant === 'outline',
        },

        className
      )}
      {...props}
    />
  )
}
```

## Build Output

- **Development build**: `pnpm build:dev` (development mode, larger bundle, source maps)
- **Production build**: `pnpm build` (minified, optimized)
- Output directory: `dist/`
- Assets are hashed for cache busting
- Build includes both Vite bundle and TypeScript type checking

## Integration with Wails

- The Vite plugin `@wailsio/runtime/plugins/vite` handles Wails integration
- Bindings directory must be specified in plugin config: `wails("./bindings")`
- Frontend communicates with Go backend through imported bindings
- No manual HTTP/WebSocket setup needed; Wails handles IPC automatically
- Dev server runs on port 9245 for Wails integration

## File and Directory Documentation Requirements

**CRITICAL**: Every file and directory MUST have proper documentation to maintain codebase clarity.

### Directory Documentation (`README.md`)

Every directory (except `bindings/`, which is auto-generated) MUST have a `README.md` with:

1. **Architecture Summary** (3 lines max):
   - Purpose of this directory
   - How it fits in the overall system
   - Key patterns or conventions

2. **File Inventory**:
   - List each file with its name, role, and function
   - Format: `- **filename.ext**: Role description`

3. **Update Reminder**:
   ```markdown
   <!-- Once this directory changes, update this README.md -->
   ```

Example `README.md`:
```markdown
<!-- Once this directory changes, update this README.md -->

# Components/Common

App-specific shared components used across multiple features.
These components contain app-specific logic but are reusable.
Place universal UI components in `components/ui/` instead.

## Files

- **ErrorElement.tsx**: Error boundary component for route errors
- **Footer.tsx**: Application footer with links and metadata
- **AppHeader.tsx**: Main navigation header with theme switcher
```

### File Documentation (Header Comments)

Every `.ts`, `.tsx`, `.js`, `.jsx` file MUST start with a 3-line header comment:

```typescript
// Input: [What this file depends on - external APIs, atoms, services, etc.]
// Output: [What this file exports - components, hooks, utilities, types, etc.]
// Position: [Role in the system - entry point, shared utility, feature component, etc.]
```

After any file modification, you MUST:
1. Update the file's header comment if its role changed
2. Update the parent directory's `README.md` if files were added/removed/changed

Example file header:
```typescript
// Input: useAtomValue from jotai, userAtom from @/atoms/user
// Output: UserProfile component displaying user information
// Position: Feature component in user module

import { useAtomValue } from 'jotai'
import { userAtom } from '@/atoms/user'

export function UserProfile() {
  // ...
}
```

### Enforcement

- **Before committing**: Ensure all new files have header comments
- **Before committing**: Ensure modified directories have updated `README.md`
- **During code review**: Check for missing or outdated documentation
- **When refactoring**: Update all affected file headers and directory READMEs

This documentation system ensures every developer can quickly understand:
- What a file does (Output)
- What it depends on (Input)
- Where it fits in the architecture (Position)
- What's in a directory (README.md file inventory)
