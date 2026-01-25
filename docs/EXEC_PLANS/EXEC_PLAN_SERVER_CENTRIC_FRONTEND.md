# Server-Centric Config Frontend Redesign

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan must be maintained in accordance with `.agent/PLANS.md` in the repository root. It depends on `docs/EXEC_PLANS/EXEC_PLAN_SERVER_CENTRIC_BACKEND.md` being implemented first, because the frontend relies on new server/client APIs and binding types.

## Purpose / Big Picture

After this change, the frontend presents a server-centric configuration experience: users see a single list of MCP servers, can manage tags, and can view active clients (formerly callers) without any profile concepts. The UI language is rewritten to match the new mental model (“Servers”, “Clients”, “Tags”), and topology/overview views reflect client-tag-server relationships. This should be observable by running the app, opening the Configuration page, and seeing server-centric tabs and detail views, plus a Clients list that shows tag chips for active connections.

## Progress

- [x] (2026-01-24 09:02Z) Drafted the server-centric frontend ExecPlan aligned with the backend migration.
- [x] (2026-01-24 09:02Z) Refined UI information architecture and binding expectations for server-centric redesign.
- [x] (2026-01-24 12:38Z) Replaced profile/caller hooks, atoms, and bindings with server/client equivalents.
- [x] (2026-01-24 12:38Z) Redesigned Configuration page UI to center on servers and clients, including new detail panels.
- [x] (2026-01-24 12:38Z) Updated topology visualization and dashboard widgets to show client-tag-server relationships.
- [x] (2026-01-24 12:38Z) Updated tools, settings, and connect-IDE flows to use server-centric APIs and copy.
- [x] (2026-01-24 12:38Z) Ran `pnpm -C frontend typecheck` and resolved all errors.

## Surprises & Discoveries

- Observation: Runtime settings no longer have a read API; the frontend now initializes with defaults and relies on writes + reloads. This should be revisited if a runtime read API returns.

## Decision Log

- Decision: Split the migration into two ExecPlans, backend first, frontend second.
  Rationale: Reduces risk and keeps UI refactor aligned with finalized API shapes.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: Use union semantics for multi-tag visibility and reflect this in UI filters.
  Rationale: Matches the server-centric visibility rule and keeps filters intuitive.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: Rename “callers” to “clients” in all UI labels and types.
  Rationale: Aligns the interface with user expectations.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: Add a tag filter row to the Servers list and tag pills in rows.
  Rationale: Keeps tag-based visibility discoverable without adding another tab.
  Date/Author: 2026-01-24 09:02Z, Codex.
- Decision: Show known tags in the Connect IDE sheet as copyable `--tag` suggestions.
  Rationale: Lowers friction for configuring clients without introducing new UI flows.
  Date/Author: 2026-01-24 09:02Z, Codex.

## Outcomes & Retrospective

- Delivered server-centric Configuration/Topology/Tools/Settings UI with tags and active clients.
- `pnpm -C frontend typecheck` passes; remaining work is visual QA in Wails dev runtime.

## Context and Orientation

The frontend currently relies on profile-based APIs (`ProfileService`, `useProfiles`, `useCallers`) and renders configuration in `frontend/src/modules/config/config-page.tsx` using profile and caller tabs. Active callers are displayed in `frontend/src/modules/dashboard/components/active-callers-panel.tsx` and other components via `use-active-callers`. Topology is defined in `frontend/src/modules/topology/config-flow.tsx` and associated layout helpers. Settings runtime configuration uses profile-derived runtime configuration in `frontend/src/modules/settings/hooks/use-runtime-settings.ts`.

After the backend migration, bindings will provide server-centric types such as `ServerSummary`, `ServerDetail`, and `ActiveClient` (with tags). The UI should remove profile terminology and present tags as the primary grouping and filtering mechanism.

## Milestones

Milestone 1: Binding refresh and data hooks. Run `make wails-bindings` after the backend plan lands, then update `frontend/src/modules/config/atoms.ts`, `frontend/src/modules/config/hooks.ts`, and `frontend/src/hooks/use-active-clients.ts` (renamed from callers) to use the new server/client bindings. Run `rg -n "profile|profiles|caller|callers" frontend/src` and expect the results to be limited to legacy files you have not touched yet.

Milestone 2: Configuration page redesign. Update `frontend/src/modules/config/config-page.tsx` and add new components under `frontend/src/modules/config/components/servers-*` and `frontend/src/modules/config/components/clients-*` for the Servers/Clients tabs. Add the tag filter row and tag pills. Run the app (`make wails-dev`) and confirm the Configuration page renders with “Servers” and “Clients” tabs and the new filter row.

Milestone 3: Topology and dashboard updates. Update `frontend/src/modules/topology/config-flow.tsx`, `frontend/src/modules/dashboard/components/active-callers-panel.tsx`, and `frontend/src/modules/dashboard/components/status-cards.tsx` to reflect clients/tags. Run the app and confirm the topology shows Tags → Servers and that the dashboard label reads “Active Clients” with tag chips.

Milestone 4: Tools, settings, and connect IDE. Update `frontend/src/modules/tools/hooks.ts`, `frontend/src/modules/settings/hooks/use-runtime-settings.ts`, and `frontend/src/components/common/connect-ide-sheet.tsx` to remove profile logic, expose tags, and show `--tag` suggestions. Run the app and confirm the Tools and Settings pages work without profile selection.

Milestone 5: Copy sweep and navigation. Update `frontend/src/components/common/app-sidebar.tsx` and any remaining copy strings. Run `rg -n "profile|profiles|caller|callers" frontend/src` and expect no matches. Do a final UI sweep for consistent “Servers/Clients/Tags” language.

## Plan of Work

First, replace bindings, hooks, and atoms. Update `frontend/src/modules/config/atoms.ts` to track servers, selected server, and clients instead of profiles and callers. Replace `frontend/src/modules/config/hooks.ts` with `useServers`, `useServer`, and `useClients`, powered by the new Go bindings (`ServerService.ListServers`, `ServerService.GetServer`, `ClientService.ListActiveClients`). Update any SWR keys that include “profiles” or “callers” to new keys such as `servers` and `clients`, and update `use-active-callers` to `use-active-clients` with the new event name (`clients:active`).

Second, redesign the Configuration page. Rewrite `frontend/src/modules/config/config-page.tsx` to use tabs `Servers` and `Clients`, update the header copy, and replace `ProfilesList`/`ProfileDetailPanel` with new `ServersList` and `ServerDetailPanel` components. Add a tag filter row above the server list (chips generated from server tags) and show tag pills in each server row. The Clients tab should be read-only, showing active clients with tag chips and last heartbeat. Update empty states and helper text to describe the single-file configuration model.

Third, update topology and dashboard surfaces. Replace the profile/caller topology in `frontend/src/modules/topology/config-flow.tsx` with a tag-centric graph that shows Tags → Servers, and optionally Clients → Tags for active connections (only when active clients exist). Update `frontend/src/modules/dashboard/components/active-callers-panel.tsx` to “Active Clients”, show tags, and adjust status cards in `frontend/src/modules/dashboard/components/status-cards.tsx` to count servers/clients instead of profiles/callers. Add an optional tag filter in runtime status views by reusing the selected client’s tags.

Fourth, refactor tools and settings flows. Update `frontend/src/modules/tools/hooks.ts` to map tools directly to servers (and their tags) without profile details. Update `frontend/src/modules/settings/hooks/use-runtime-settings.ts` to load runtime configuration from the new server-centric API rather than picking a runtime profile. Update `frontend/src/components/common/connect-ide-sheet.tsx` to remove caller selection and instead display instructions for passing `--tag` flags in client configuration, including a list of known tags derived from server data.

Finally, adjust labels, copy, and navigation. Update `frontend/src/components/common/app-sidebar.tsx` and any inline text that mentions profiles/callers. Ensure the new terminology is consistent across the app, especially in onboarding and empty states.

## Concrete Steps

From the repository root (`/Users/wibus/dev/mcpd`), update the bindings and then refactor UI modules in order: config → topology → dashboard → tools → settings → shared components. Use `rg -n "profile|profiles|caller|callers"` to find and replace stale terms. After each module change, run the app and validate the corresponding page manually.

Suggested commands during implementation:

    rg -n "profile|profiles|caller|callers" frontend/src
    make wails-bindings

If the frontend build pipeline exists, run it after large refactors; otherwise use the Wails dev workflow already used in this repo.

## Validation and Acceptance

Start the app and navigate through the Configuration, Dashboard, Tools, Topology, and Settings pages. Confirm that:

    - The Configuration page shows “Servers” and “Clients” tabs with no profile/caller mentions.
    - Active clients show tag chips, and server lists show tags with a tag filter row.
    - Topology shows Tags → Servers (and active Clients if enabled), with filtering by tag.
    - Tools sidebar groups or labels tools by server and tag without profile references.
    - Runtime settings load successfully without profile selection.

Manual validation is sufficient; there are no existing frontend tests to update in this repository.

## Idempotence and Recovery

UI refactors are safe to repeat; if a page breaks, revert only the most recent module-level edits and re-apply changes incrementally. Keep the backend plan and frontend plan aligned by confirming the binding names after regenerating Wails bindings.

## Artifacts and Notes

New UI labels to use consistently (examples, not code fences):

    Configuration → Servers / Clients
    Active Callers → Active Clients
    Profiles → Tags or Servers (depending on context)

## Interfaces and Dependencies

Frontend must consume the new binding types and methods defined in the backend plan, including:

    ServerService.ListServers() -> ServerSummary[]
    ServerService.GetServer(name: string) -> ServerDetail
    ServerService.SetServerDisabled(...)
    ServerService.DeleteServer(...)
    ConfigService.UpdateRuntimeConfig(...)
    ClientService.ListActiveClients() -> ActiveClient[]

Types must include tag metadata, for example:

    type ServerSummary = { name: string; tagCount: number; disabled: boolean; }
    type ServerDetail = { name: string; tags: string[]; ... }
    type ActiveClient = { client: string; tags: string[]; lastHeartbeat: string; }

Change Note: Refined the UI information architecture (tag filters, client views) and aligned binding expectations with the server-centric backend plan.
