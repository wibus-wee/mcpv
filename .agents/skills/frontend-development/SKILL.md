---
name: frontend-development
description: Guidelines for Wails 3 GUI frontend development with React and TypeScript. Use when working on frontend/, UI components, or Wails bindings.
user-invocable: false
---

# Frontend Development Guidelines

Guidelines for developing the Wails 3 GUI frontend of mcpd, built with React and TypeScript.

## Prerequisites

- Node.js (version specified in frontend/package.json)
- Wails 3.0.0-alpha.53
- Go 1.25+ (for Wails bindings generation)

## Development Commands

### Wails GUI Development

```bash
# Generate TypeScript bindings for Go services
make wails-bindings

# Run Wails in development mode (hot reload)
make wails-dev

# Build Wails application
make wails-build

# Package Wails application for distribution
make wails-package
```

## Frontend Structure

The frontend is located in the `frontend/` directory and follows standard React/TypeScript conventions.

**Key Directories**:
- `frontend/src/`: Source code
  - `components/`: Reusable UI components
  - `routes/`: Page components and routing
  - `modules/`: Feature modules with hooks and utilities
  - `bindings/`: Auto-generated TypeScript bindings from Go services
- `frontend/public/`: Static assets

## Working with Wails Bindings

### Generating Bindings

Whenever you modify Go service methods in `internal/ui/`, regenerate TypeScript bindings:

```bash
make wails-bindings
```

This creates TypeScript interfaces and functions in `frontend/src/bindings/` that match your Go service methods.

### Using Bindings in Frontend

Import and use the generated bindings in your React components:

```typescript
import { ServiceMethod } from '@/bindings/...'

// Use in component
const result = await ServiceMethod(params)
```

## Development Workflow

1. **Modify Go Services**: Add or update methods in `internal/ui/`
2. **Regenerate Bindings**: Run `make wails-bindings`
3. **Update Frontend**: Use the new bindings in your React components
4. **Test in Dev Mode**: Run `make wails-dev` for hot reload
5. **Build**: Run `make wails-build` when ready

## Frontend-Specific Guidelines

For detailed React/TypeScript frontend-specific guidance, see `frontend/CLAUDE.md`.

## Integration with Backend

The Wails bridge layer (`internal/ui/`) exposes core functionality to the GUI:

- Service methods in `internal/ui/` become callable from frontend
- TypeScript bindings provide type-safe access to Go functions
- Wails handles serialization/deserialization automatically
- Frontend can access all core control plane features through bindings

## Common Tasks

### Adding a New UI Feature

1. Implement Go service methods in `internal/ui/`
2. Regenerate TypeScript bindings: `make wails-bindings`
3. Create React components in `frontend/src/`
4. Use bindings to call Go services: `import { Method } from '@/bindings/...'`
5. Test in development mode: `make wails-dev`

### Updating Existing Features

1. Modify Go service methods in `internal/ui/` if needed
2. Regenerate bindings if Go signatures changed: `make wails-bindings`
3. Update React components in `frontend/src/`
4. Test changes: `make wails-dev`

### Debugging

- Use browser DevTools in Wails dev mode
- Check console for TypeScript/React errors
- Verify bindings are up-to-date if Go methods changed
- Check `internal/ui/` for Go-side errors

## Best Practices

- Always regenerate bindings after modifying Go services
- Keep UI logic in React components, business logic in Go services
- Use TypeScript types from bindings for type safety
- Follow React best practices (hooks, component composition, etc.)
- See `frontend/CLAUDE.md` for detailed frontend conventions

## Related Documentation

- **Frontend Guide**: See `frontend/CLAUDE.md` for React/TypeScript frontend-specific guidance
- **Wails Documentation**: https://wails.io/
- **Project Overview**: Use the `project-overview` skill for architecture details
- **Internal Development**: Use the `internal-development` skill for Go backend work
