# Custom URL Protocol (Deep Links)

mcpv supports custom URL protocols for deep linking into the application:
- **Production**: `mcpv://`
- **Development**: `mcpvev://`

This separation prevents conflicts between production and development versions running on the same machine.

## Protocol Scheme

The application registers the `mcpv://` URL scheme, allowing external applications, browser links, and scripts to open mcpv with specific navigation paths.

## URL Format

**Production:**
```
mcpv://<path>?<query-params>
```

**Development:**
```
mcpvev://<path>?<query-params>
```

### Examples

**Production:**
- **Just open the app**: `mcpv://` or `mcpv://open`
- Open servers page: `mcpv://servers`
- Open servers with specific tab: `mcpv://servers?tab=overview`
- Open specific server: `mcpv://servers?server=my-server&tab=config`
- Open settings: `mcpv://settings`

**Development:**
- **Just open the app**: `mcpvev://` or `mcpvev://open`
- Open servers page: `mcpvev://servers`
- Open servers with specific tab: `mcpvev://servers?tab=overview`
- Open settings: `mcpvev://settings`

**Note**: Using the scheme without a path (or with root path `/`) will simply open/focus the application without navigating to any specific page. This is useful for just launching the app from external triggers.

## Implementation Details

### Backend

1. **URL Parsing** ([internal/ui/deeplink.go](../../internal/ui/deeplink.go))
   - Validates URL scheme is `mcpv://`
   - Normalizes path from host and path segments
   - Extracts query parameters

2. **Manager Integration** ([internal/ui/manager.go](../../internal/ui/manager.go))
   - `HandleDeepLink(rawURL string)` validates and emits deep link events
   - Emits `deep-link` event with path and params to frontend

3. **Event System** ([internal/ui/events.go](../../internal/ui/events.go))
   - `EventDeepLink` constant for event name
   - `DeepLinkEvent` struct containing path and params
   - `emitDeepLink()` helper function

4. **Application Entry** ([app.go](../../app.go))
   - Listens for `ApplicationOpenURL` custom events
   - Delegates to `Manager.HandleDeepLink()`
   - Shows and focuses window when deep link is triggered

### Frontend

**Event Bridge** ([frontend/src/providers/root-provider.tsx](../../frontend/src/providers/root-provider.tsx))
- Listens for `deep-link` events from backend
- Extracts path and params from event data
- Navigates to target route using TanStack Router

## Testing

Run tests with:

```bash
go test ./internal/ui -v -run "TestParseDeepLink"
go test ./internal/ui -v -run "TestManager_HandleDeepLink"
```

Test the functionality:

```bash
# Production: Just open/focus the app without navigation
open "mcpv://"

# Production: Navigate to specific page
open "mcpv://servers?tab=overview"

# Development: Just open/focus the app without navigation
open "mcpvev://"

# Development: Navigate to specific page
open "mcpvev://servers?tab=overview"
```

## Platform Registration

### macOS

The custom protocol is registered via the app's `Info.plist` during the build process. Once built and installed, the system recognizes `mcpv://` URLs.

Test examples:
```bash
# Production build
open "mcpv://"
open "mcpv://servers?tab=overview"

# Development build
open "mcpvev://"
open "mcpvev://servers?tab=overview"
open "mcpvev://settings"
```

### Security

- All URLs are validated before processing
- Only `mcpv://` and `mcpvev://` schemes are accepted
- Production and development versions use different schemes to avoid conflicts
- Query parameters are sanitized
- Invalid URLs generate error events but don't crash the app

## Extension Points

To add new deep link targets:

1. Define route in frontend router
2. URL will automatically route via the deep link handler
3. No backend changes needed for simple navigation

For custom actions requiring backend logic:

1. Extend `HandleDeepLink` in [internal/ui/manager.go](../../internal/ui/manager.go)
2. Add action-specific handlers
3. Emit custom events as needed
