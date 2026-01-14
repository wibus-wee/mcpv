# Tools Page Redesign Specification

## Overview

Redesign the `/tools` page to organize tools by MCP Server, displaying each server as a glass-effect card in a responsive grid. Clicking a server card opens a right-side sheet showing tool list, runtime details, and server configuration. Clicking a tool within the sheet opens a nested sheet displaying parameter details.

**Key Goals:**
- Organize tools by server for better discoverability
- Provide visual runtime status feedback
- Create an aesthetically pleasing, modern interface
- Leverage all available UI components and icons

---

## Backend Changes Required

### Problem
Current `ToolEntry` type lacks server association:
```typescript
interface ToolEntry {
  name: string
  toolJson: json.RawMessage  // Only MCP schema, no server info
}
```

### Solution
Add server metadata to tools:

**File: `/Users/wibus/dev/mcpd/internal/ui/types.go`**
```go
type ToolEntry struct {
    Name       string          `json:"name"`
    ToolJson   json.RawMessage `json:"toolJson"`
    SpecKey    string          `json:"specKey"`    // NEW: Server fingerprint
    ServerName string          `json:"serverName"` // NEW: Human-readable name
}
```

**File: `/Users/wibus/dev/mcpd/internal/ui/service.go`**
Update `GetTools()` method to populate `SpecKey` and `ServerName` from the server spec when collecting tools.

---

## Layout & Structure

### Page Layout
```
┌─────────────────────────────────────────────────────────────┐
│ Tools                                                        │
│ Available MCP tools from all connected servers              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Server 1 │  │ Server 2 │  │ Server 3 │  │ Server 4 │   │
│  │ ●●●○     │  │ ●●       │  │ ●●●●●    │  │ ○        │   │
│  │ 5 tools  │  │ 3 tools  │  │ 8 tools  │  │ 0 tools  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│                                                              │
│  ┌──────────┐  ┌──────────┐                                │
│  │ Server 5 │  │ Server 6 │                                │
│  │ ●●●      │  │ ●●●●     │                                │
│  │ 4 tools  │  │ 6 tools  │                                │
│  └──────────┘  └──────────┘                                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Responsive Grid
- **Desktop (≥1280px)**: 4 columns
- **Tablet (≥768px)**: 3 columns
- **Mobile (<768px)**: 2 columns

**Tailwind Classes:**
```tsx
<div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4">
```

---

## Visual Design

### Server Card

**Glass Effect:**
```tsx
className={cn(
  'relative overflow-hidden rounded-xl border border-border/50',
  'bg-background/80 backdrop-blur-sm',
  'transition-all duration-300',
  'hover:border-primary/50 hover:shadow-[0_0_20px_rgba(var(--primary-rgb),0.15)]',
  'cursor-pointer'
)}
```

**Structure:**
```
┌─────────────────────────────┐
│ [ServerIcon]                │
│ Server Name                 │
│ ●●●○○ (Runtime Status)      │
│ [Badge: 5 tools]            │
└─────────────────────────────┘
```

**Card Content:**
- **Icon**: `ServerIcon` from Lucide React (top, centered)
- **Server Name**: `text-lg font-semibold` (centered)
- **Runtime Status**: `ServerRuntimeIndicator` component (centered, shows up to 5 dots)
- **Tool Count**: `Badge` component with neutral variant (centered)

**Empty State Card:**
```
┌─────────────────────────────┐
│ [ServerIcon]                │
│ Server Name                 │
│ ●●●○○                       │
│ [Badge: 0 tools]            │
│ No tools available          │
└─────────────────────────────┘
```

**Hover Effect:**
- Border color changes to `border-primary/50`
- Glow shadow: `shadow-[0_0_20px_rgba(var(--primary-rgb),0.15)]`
- Smooth transition: `duration-300`

**Colors:**
- Neutral only (no health-based colors)
- Use theme colors: `bg-background`, `text-foreground`, `border-border`

---

## Main Sheet (Server Details)

### Sheet Configuration
- **Side**: Right
- **Width**: 40% of viewport (`max-w-[40vw]`)
- **Backdrop**: Yes (with blur)
- **Inset**: Yes (rounded corners on desktop)

**Component:**
```tsx
<Sheet>
  <SheetContent side="right" className="max-w-[40vw]">
    <SheetHeader>
      <SheetTitle>{serverName}</SheetTitle>
      <SheetDescription>Server tools and runtime status</SheetDescription>
    </SheetHeader>
    <SheetPanel>
      {/* Content sections */}
    </SheetPanel>
  </SheetContent>
</Sheet>
```

### Content Sections

#### 1. Tools List
**Layout:**
```
Tools (8)
─────────────────────────────
┌─────────────────────────────┐
│ [ToolIcon] tool_name        │
│ Tool description here...    │
└─────────────────────────────┘
┌─────────────────────────────┐
│ [ToolIcon] another_tool     │
│ Another description...      │
└─────────────────────────────┘
```

**Component:**
- Use `Card` component for each tool
- Icon: `WrenchIcon` from Lucide React
- Click opens nested sheet
- Hover effect: `hover:bg-muted/50`

#### 2. Runtime Details
**Component:** `ServerRuntimeDetails` (existing)
- Shows instance counts by state
- Color-coded dots with labels

#### 3. Server Configuration
**Layout:**
```
Configuration
─────────────────────────────
Command: /path/to/server
Arguments: --arg1 --arg2
Environment: KEY=value
```

**Component:**
- Use `Card` component
- Display spec details from backend
- Monospace font for paths/commands: `font-mono text-sm`

---

## Nested Sheet (Tool Details)

### Sheet Configuration
- **Side**: Right
- **Width**: 35% of viewport (`max-w-[35vw]`)
- **Backdrop**: No (parent sheet remains visible)
- **Inset**: Yes

**Pattern (BaseUI Nested Dialog):**
```tsx
<Sheet> {/* Main sheet */}
  <SheetContent>
    {tools.map(tool => (
      <Sheet key={tool.name}> {/* Nested sheet */}
        <SheetTrigger asChild>
          <Card>{tool.name}</Card>
        </SheetTrigger>
        <SheetContent side="right" className="max-w-[35vw]">
          {/* Tool details */}
        </SheetContent>
      </Sheet>
    ))}
  </SheetContent>
</Sheet>
```

### Content

#### Tool Header
- **Title**: Tool name
- **Description**: Tool description from schema

#### Parameter Table
**Layout:**
```
Parameters
─────────────────────────────────────────
│ Name      │ Type    │ Required │ Desc  │
├───────────┼─────────┼──────────┼───────┤
│ file_path │ string  │ Yes      │ Path  │
│ limit     │ number  │ No       │ Limit │
```

**Component:**
- Use `Table` component from `components/ui/table.tsx`
- Parse `toolJson` to extract parameters
- Display: name, type, required, description
- Use `Badge` for required/optional indicator

#### Schema Display
**Raw JSON (Collapsible):**
```tsx
<Collapsible>
  <CollapsibleTrigger>
    <CodeIcon /> View Raw Schema
  </CollapsibleTrigger>
  <CollapsibleContent>
    <pre className="font-mono text-xs bg-muted p-4 rounded-lg overflow-auto">
      {JSON.stringify(toolJson, null, 2)}
    </pre>
  </CollapsibleContent>
</Collapsible>
```

---

## Animations

### Page Entry
```tsx
<m.div
  initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
  animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
  transition={Spring.smooth(0.4)}
>
```

### Card Stagger
```tsx
{servers.map((server, index) => (
  <m.div
    key={server.specKey}
    initial={{ opacity: 0, scale: 0.95 }}
    animate={{ opacity: 1, scale: 1 }}
    transition={Spring.smooth(0.3, index * 0.05)}
  >
    <ServerCard server={server} />
  </m.div>
))}
```

### Sheet Transitions
- Use default Sheet component transitions (built-in)
- Smooth slide-in from right
- Backdrop fade-in

---

## Loading States

### Page Loading
```tsx
<div className="flex items-center justify-center h-full">
  <Spinner className="size-8" />
  <span className="ml-2 text-muted-foreground">Loading servers...</span>
</div>
```

### Card Skeleton (if needed)
```tsx
<div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4">
  {Array.from({ length: 6 }).map((_, i) => (
    <Skeleton key={i} className="h-48 rounded-xl" />
  ))}
</div>
```

---

## Empty States

### No Servers
```tsx
<div className="flex flex-col items-center justify-center h-full gap-4">
  <ServerOffIcon className="size-16 text-muted-foreground" />
  <div className="text-center">
    <h3 className="text-lg font-semibold">No servers configured</h3>
    <p className="text-sm text-muted-foreground">
      Add MCP servers to your configuration to see tools
    </p>
  </div>
</div>
```

### Server with No Tools
Display card normally with:
- Tool count badge showing "0 tools"
- Message: "No tools available"
- Card is still clickable (shows empty sheet)

---

## Icons (Lucide React)

**Primary Icons:**
- `ServerIcon` - Server cards
- `WrenchIcon` - Tool items
- `CodeIcon` - Raw schema toggle
- `ServerOffIcon` - Empty state
- `LoaderIcon` - Loading spinner
- `ChevronRightIcon` - Navigation hints
- `XIcon` - Close buttons (built into Sheet)

**Import:**
```tsx
import {
  ServerIcon,
  WrenchIcon,
  CodeIcon,
  ServerOffIcon,
  LoaderIcon,
  ChevronRightIcon,
} from 'lucide-react'
```

---

## Component Breakdown

### New Components

#### 1. `ServerCard.tsx`
**Location:** `frontend/src/modules/tools/components/server-card.tsx`

**Props:**
```typescript
interface ServerCardProps {
  specKey: string
  serverName: string
  toolCount: number
  className?: string
  onClick: () => void
}
```

**Responsibilities:**
- Display server icon, name, runtime status, tool count
- Handle click to open sheet
- Apply glass effect styling
- Show hover effects

#### 2. `ServerDetailsSheet.tsx`
**Location:** `frontend/src/modules/tools/components/server-details-sheet.tsx`

**Props:**
```typescript
interface ServerDetailsSheetProps {
  specKey: string
  serverName: string
  tools: ToolEntry[]
  open: boolean
  onOpenChange: (open: boolean) => void
}
```

**Responsibilities:**
- Render main sheet with tools list, runtime details, config
- Manage nested sheet state for tool details
- Fetch server configuration from backend

#### 3. `ToolDetailsSheet.tsx`
**Location:** `frontend/src/modules/tools/components/tool-details-sheet.tsx`

**Props:**
```typescript
interface ToolDetailsSheetProps {
  tool: ToolEntry
  open: boolean
  onOpenChange: (open: boolean) => void
}
```

**Responsibilities:**
- Parse tool schema from `toolJson`
- Display parameter table
- Show raw schema in collapsible section

#### 4. `ToolsGrid.tsx`
**Location:** `frontend/src/modules/tools/components/tools-grid.tsx`

**Props:**
```typescript
interface ToolsGridProps {
  className?: string
}
```

**Responsibilities:**
- Fetch tools from backend
- Group tools by server
- Render responsive grid of ServerCards
- Handle loading and empty states
- Manage sheet open/close state

---

## Data Flow

### 1. Data Fetching
```typescript
// hooks.ts
export function useToolsByServer() {
  const { data: tools, isLoading } = useSWR<ToolEntry[]>(
    'tools',
    () => DiscoveryService.ListTools()
  )

  const { data: runtimeStatus } = useRuntimeStatus()

  const serverMap = useMemo(() => {
    if (!tools) return new Map()

    const map = new Map<string, {
      specKey: string
      serverName: string
      tools: ToolEntry[]
    }>()

    tools.forEach(tool => {
      if (!map.has(tool.specKey)) {
        map.set(tool.specKey, {
          specKey: tool.specKey,
          serverName: tool.serverName,
          tools: []
        })
      }
      map.get(tool.specKey)!.tools.push(tool)
    })

    return map
  }, [tools])

  return { serverMap, isLoading, runtimeStatus }
}
```

### 2. Component Hierarchy
```
ToolsPage (routes/tools.tsx)
└── ToolsGrid
    ├── ServerCard (×N)
    └── ServerDetailsSheet
        ├── ToolsList
        │   └── ToolDetailsSheet (nested)
        ├── ServerRuntimeDetails
        └── ServerConfiguration
```

### 3. State Management
- **Sheet State**: Local state in ToolsGrid (controlled)
- **Tools Data**: SWR cache (auto-refresh)
- **Runtime Status**: SWR cache (2s refresh interval)

---

## Implementation Order

### Phase 1: Backend Changes
1. Add `SpecKey` and `ServerName` fields to `ToolEntry` in `internal/ui/types.go`
2. Update `GetTools()` in `internal/ui/service.go` to populate new fields
3. Run `make wails-bindings` to regenerate TypeScript types
4. Run `make test` to verify

### Phase 2: Data Layer
1. Create `frontend/src/modules/tools/hooks.ts`
2. Implement `useToolsByServer()` hook
3. Test data grouping logic

### Phase 3: Base Components
1. Create `ServerCard.tsx` with glass effect styling
2. Create `ToolDetailsSheet.tsx` with parameter table
3. Test components in isolation

### Phase 4: Main Components
1. Create `ServerDetailsSheet.tsx` with nested sheet pattern
2. Integrate `ServerRuntimeDetails` component
3. Add server configuration display
4. Test nested sheet behavior

### Phase 5: Grid Layout
1. Create `ToolsGrid.tsx` with responsive grid
2. Implement loading and empty states
3. Add Motion animations with stagger
4. Wire up sheet state management

### Phase 6: Integration
1. Update `routes/tools.tsx` to use `ToolsGrid`
2. Remove old `ToolsTable` component
3. Test full flow: page load → card click → sheet open → tool click → nested sheet
4. Verify animations and transitions

### Phase 7: Polish
1. Fine-tune glass effect and hover states
2. Adjust spacing and typography
3. Test responsive behavior on different screen sizes
4. Verify accessibility (keyboard navigation, screen readers)

---

## Testing Strategy

### Unit Tests
- `useToolsByServer()` hook: Test data grouping logic
- `ToolDetailsSheet`: Test schema parsing and parameter extraction

### Integration Tests
- Full page flow: Load → Click card → View tools → Click tool → View details
- Empty states: No servers, server with no tools
- Loading states: Skeleton/spinner display

### Visual Tests
- Glass effect rendering
- Hover states and transitions
- Responsive grid layout (2/3/4 columns)
- Sheet animations and nested behavior

### Accessibility Tests
- Keyboard navigation (Tab, Enter, Esc)
- Screen reader announcements
- Focus management in sheets
- ARIA labels and descriptions

---

## Technical Constraints

### Performance
- Use `useMemo` for server grouping to avoid re-computation
- Lazy load sheet content (only render when open)
- Limit runtime status dots to 5 per card ("+N more" indicator)

### Browser Support
- Modern browsers with CSS backdrop-filter support
- Fallback: Solid background if backdrop-blur not supported

### Accessibility
- All interactive elements keyboard accessible
- Proper ARIA labels on sheets and cards
- Focus trap in sheets
- Esc key closes sheets

---

## Design Tokens

### Colors
```css
--background: hsl(var(--background))
--foreground: hsl(var(--foreground))
--border: hsl(var(--border))
--primary: hsl(var(--primary))
--muted: hsl(var(--muted))
--muted-foreground: hsl(var(--muted-foreground))
```

### Spacing
- Card padding: `p-6`
- Grid gap: `gap-4`
- Section spacing: `space-y-4`

### Typography
- Page title: `text-2xl font-bold`
- Card title: `text-lg font-semibold`
- Body text: `text-sm`
- Monospace: `font-mono text-xs`

### Borders
- Card border: `border border-border/50`
- Hover border: `border-primary/50`
- Radius: `rounded-xl` (cards), `rounded-lg` (nested elements)

### Shadows
- Hover glow: `shadow-[0_0_20px_rgba(var(--primary-rgb),0.15)]`

---

## Future Enhancements (Out of Scope)

- Search/filter tools across all servers
- Favorite/pin frequently used tools
- Tool execution interface (call tools directly from UI)
- Server health indicators (beyond runtime status)
- Tool usage analytics
- Bulk operations on tools

---

## Success Criteria

✅ Tools are organized by server in a clear, visual layout
✅ Runtime status is visible at a glance on each card
✅ Glass effect creates a modern, polished aesthetic
✅ Sheets provide detailed information without navigation
✅ Nested sheets work smoothly for tool parameter details
✅ Animations are smooth and enhance UX
✅ Loading and empty states are handled gracefully
✅ All UI components from `components/ui/` are utilized
✅ Lucide React icons are used throughout (no custom SVGs)
✅ Responsive design works on all screen sizes
✅ Keyboard navigation and accessibility are fully supported

---

**Document Version:** 1.0
**Last Updated:** 2025-12-28
**Status:** Ready for Implementation
