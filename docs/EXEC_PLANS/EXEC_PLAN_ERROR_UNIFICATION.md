# Unified Error Model and Boundary Mapping

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan follows `.agent/PLANS.md` from the repository root and must be maintained in accordance with it.

## Purpose / Big Picture

After this change, errors across the control plane, gateway, and UI follow a single, predictable model. A caller can trigger a failure (for example, calling a non-existent tool) and see the same error code and category regardless of which boundary they use (RPC, UI, or gateway). Operators can also interpret errors consistently in logs because the error carries a stable code and an operation name. This is visible by running existing tests and by observing that error codes returned from gRPC are stable across tools, prompts, resources, and runtime watches.

## Progress

- [x] (2026-02-08 11:12) Create this ExecPlan with a full repo-wide error unification design and milestones.
- [x] (2026-02-08 11:40) Introduce a domain error type with stable codes and helpers; add mapping helpers for RPC and UI boundaries.
- [x] (2026-02-08 11:40) Migrate RPC error mapping to the new unified mapper and remove per-endpoint ad-hoc mapping.
- [x] (2026-02-08 12:10) Migrate infra layers (router, scheduler, lifecycle, plugin, gateway) to emit domain errors or wrap them consistently.
- [x] (2026-02-08 11:40) Align UI errors to the new domain error codes and update tests.
- [x] (2026-02-08 12:20) Run full validation and document outcomes.

## Surprises & Discoveries

- Observation: UI tests link with a macOS version mismatch warning in this environment.
  Evidence: go test ./internal/ui/... logs ld warnings about object files built for newer macOS versions.

## Decision Log

- Decision: Use a domain-level error type with stable codes plus boundary mappers, rather than only extending existing sentinel errors.
  Rationale: This provides a single source of truth for error semantics while keeping compatibility with sentinel errors through wrapping and mapping.
  Date/Author: 2026-02-08 / Codex
- Decision: Replace per-endpoint RPC error mapping with a unified `statusFromError` mapper.
  Rationale: This reduces duplication and makes gRPC status code behavior consistent across tools, prompts, resources, tasks, and runtime watches.
  Date/Author: 2026-02-08 / Codex
- Decision: Add a lint rule that forbids `errors.New` in `internal/infra/rpc` to keep RPC errors wrapped in the unified domain model.
  Rationale: This prevents new ad-hoc RPC errors from bypassing the unified error mapper.
  Date/Author: 2026-02-08 / Codex

## Outcomes & Retrospective

The repository now has a unified domain error type, a single RPC error mapper, and a UI error mapper tied to domain codes. All RPC handlers route errors through a single mapping path, and core infra layers (router, scheduler, lifecycle, plugin, gateway) wrap their errors with stable domain codes. Full test runs succeeded, with repeated macOS linker warnings in the UI tests that appear to be environmental rather than code-related.

## Context and Orientation

This repository currently uses multiple error styles. The domain layer defines many sentinel errors in `internal/domain/types.go` (for example `ErrToolNotFound`, `ErrInvalidRequest`, `ErrClientNotRegistered`), plus special structured errors such as `RouteError` in `internal/domain/route_error.go` and `ProtocolError` in `internal/domain/protocol_error.go`. The RPC layer maps errors to gRPC status codes in `internal/infra/rpc/control_service_errors.go` with several different mapping functions. The UI layer defines its own error codes in `internal/ui/errors.go` and performs direct `errors.Is` checks against domain sentinels. The gateway layer (`internal/infra/gateway/rpc_client.go`) reacts to gRPC status codes to reset connections or re-register clients. Many infra components produce errors with `errors.New` or `fmt.Errorf` messages without a stable code, which makes mapping inconsistent.

The goal is to introduce a single error model at the domain layer and then map it at boundaries (RPC and UI) so that the same error consistently turns into the same transport code and message. The new model must coexist with current sentinel errors during the migration and must not change external API signatures.

## Plan of Work

First, add a domain-level error type with stable codes and a small helper API for wrapping existing errors. This should live in `internal/domain/error.go` (or a similar new file) and include a code enumeration, an error struct that carries `Code`, `Op`, `Message`, `Cause`, `Retryable`, and `Meta`, and helper constructors such as `E` and `Wrap`. Provide a `CodeFrom(err)` helper that can extract a code from either the new error type or known sentinel errors. Keep sentinel errors for compatibility but make them map to the new codes.

Second, introduce boundary mappers: in `internal/infra/rpc` add a `statusFromError(op, err)` function that converts domain codes and known context errors into gRPC status codes. This function will replace the per-endpoint mapping functions in `control_service_errors.go`. In `internal/ui/errors.go`, add a `MapError(err)` function that converts the domain codes into the UI’s error codes, and keep the existing `Error` struct as the UI transport format. The mapping should be deterministic and easy to audit.

Third, migrate the RPC services to use the unified mapper. Replace calls to `mapCallToolError`, `mapReadResourceError`, `mapGetPromptError`, `mapListError`, and `mapClientError` with `statusFromError`. Remove the old mapping helpers once all call sites are updated. Ensure governance rejections still map to the correct gRPC codes (using the existing rejection mapping logic).

Fourth, update infra components that are most visible to callers (router, scheduler, lifecycle, plugin manager, gateway) so they return either domain errors or wrapped errors using `domain.Wrap`. The intent is not to replace every `fmt.Errorf` in one pass but to ensure that errors that cross API boundaries are wrapped with a stable code and operation. This step should focus on error paths that currently map inconsistently (for example scheduler capacity failures and invalid request paths in the router).

Fifth, update UI error mapping to rely on domain error codes instead of direct sentinel comparisons. Adjust UI tests to assert on the new codes.

Finally, run tests for RPC, UI, and any affected infra packages. Document observed outputs and update the plan with outcomes.

## Concrete Steps

Work from the repository root. After each milestone, run targeted tests to ensure changes are safe.

1) Add domain error type and helper APIs.
   - Create `internal/domain/error.go` with the new error type and helpers.
   - Add code-to-sentinel mapping in the same file or a small helper file.

2) Add RPC error mapper.
   - Create or update `internal/infra/rpc/error_mapper.go` with `statusFromError`.
   - Migrate RPC service code to use it.

3) Update UI mapper.
   - Update `internal/ui/errors.go` to map from domain error codes.

4) Migrate infra layers in priority order.
   - Router: `internal/infra/router/router.go`.
   - Scheduler: `internal/infra/scheduler/*`.
   - Lifecycle and plugin manager: `internal/infra/lifecycle/manager.go`, `internal/infra/plugin/manager.go`.
   - Gateway client handling if needed.

5) Run validation.
   - `go test ./internal/infra/rpc/...`
   - `go test ./internal/ui/...`
   - `go test ./internal/infra/router/...` and `go test ./internal/infra/scheduler/...`.

Expected sample output (shortened):

    ok   mcpv/internal/infra/rpc  0.6s
    ok   mcpv/internal/ui         0.3s

## Validation and Acceptance

Acceptance is reached when:

- Calling a missing tool via RPC returns a stable gRPC code (NotFound) that is derived from the domain error code rather than ad-hoc mappings.
- The same missing tool error in the UI surfaces the same stable UI error code.
- Existing unit tests pass and any new tests that assert error codes succeed.

A human can verify this by running RPC tests and by exercising a simple call with a fake control plane in `internal/infra/rpc/control_service_test.go` to confirm error codes are stable and consistent across tools/resources/prompts.

## Idempotence and Recovery

All changes are additive and can be applied incrementally. Each milestone can be re-run without damaging state. If a migration step causes unexpected behavior, revert only the affected files and continue with earlier milestones; the new error type and mapper can coexist with the old mapping until the final cleanup step.

## Artifacts and Notes

Record key diffs and test outputs here as the work proceeds. Keep examples short and focused on error code mapping and test success.

    go test ./internal/infra/rpc/... ./internal/ui/...
    ok   mcpv/internal/infra/rpc  0.6s
    ld: warning: object file (...) was built for newer 'macOS' version (26.0) than being linked (11.0)
    ok   mcpv/internal/ui  0.6s

    go test ./internal/infra/router/...
    ok   mcpv/internal/infra/router  0.5s

    go test ./internal/infra/scheduler/... ./internal/infra/lifecycle/...
    ok   mcpv/internal/infra/scheduler  1.0s
    ok   mcpv/internal/infra/lifecycle  5.5s

    go test ./internal/infra/plugin/... ./internal/infra/gateway/...
    ok   mcpv/internal/infra/plugin   4.6s
    ok   mcpv/internal/infra/gateway  1.0s

    go test ./...
    ok   mcpv/internal/infra/rpc  0.4s
    ld: warning: object file (...) was built for newer 'macOS' version (26.0) than being linked (11.0)
    ok   mcpv/internal/ui  0.6s

    go test ./internal/infra/rpc/...
    ok   mcpv/internal/infra/rpc  0.5s

## Interfaces and Dependencies

Define the following in `internal/domain/error.go`:

    type ErrorCode string
    const (
        CodeInvalidArgument ErrorCode = "INVALID_ARGUMENT"
        CodeNotFound        ErrorCode = "NOT_FOUND"
        CodeUnavailable     ErrorCode = "UNAVAILABLE"
        CodeFailedPrecond   ErrorCode = "FAILED_PRECONDITION"
        CodePermissionDenied ErrorCode = "PERMISSION_DENIED"
        CodeUnauthenticated ErrorCode = "UNAUTHENTICATED"
        CodeInternal        ErrorCode = "INTERNAL"
        CodeCanceled        ErrorCode = "CANCELED"
        CodeDeadlineExceeded ErrorCode = "DEADLINE_EXCEEDED"
        CodeNotImplemented  ErrorCode = "NOT_IMPLEMENTED"
    )

    type Error struct {
        Code ErrorCode
        Op string
        Message string
        Cause error
        Retryable bool
        Meta map[string]string
    }

    func (e *Error) Error() string
    func (e *Error) Unwrap() error
    func E(code ErrorCode, op, msg string, cause error) *Error
    func Wrap(code ErrorCode, op string, err error) *Error
    func CodeFrom(err error) (ErrorCode, bool)

In `internal/infra/rpc/error_mapper.go`, define:

    func statusFromError(op string, err error) error

This function must map domain error codes to gRPC status codes consistently and must preserve existing governance rejection behavior (by calling the existing governance rejection mapping helper).

In `internal/ui/errors.go`, define:

    func MapError(err error) *Error

This must translate domain error codes to the existing UI error codes, keeping the UI `Error` struct unchanged.

---

Change Log: Initial creation of ExecPlan for repository-wide error unification.
Change Log: Marked domain error type, RPC mapper, and UI mapping milestones complete after implementing new helpers and updating call sites.
Change Log: Completed infra migration for scheduler, lifecycle, plugin, and gateway; recorded test outputs.
Change Log: Marked full validation complete after `go test ./...` with environment linker warnings noted.
Change Log: Added a forbidigo lint rule scoped to `internal/infra/rpc` to prevent `errors.New` usage.
