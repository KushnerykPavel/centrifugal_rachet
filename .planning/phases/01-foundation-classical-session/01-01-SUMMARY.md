---
phase: 01-foundation-classical-session
plan: 01
subsystem: infra
tags: [go, monorepo, go-mod, cmd-binaries]

requires: []
provides:
  - Single Go module root with pinned dependencies
  - Four stub cmd/ binary entrypoints
  - internal/deps for dependency tracking

affects: [02-pq-protocol, 03-centrifugo-integration, 04-observability]

tech-stack:
  added: [go-doubleratchet v0.0.2, centrifuge-go v0.10.12, prometheus/client_golang v1.23.2, stretchr/testify v1.11.1]
  patterns: [single-module monorepo, blank-import dependency tracking]

key-files:
  created: [go.mod, go.sum, cmd/alice-classical/main.go, cmd/bob-classical/main.go, cmd/alice-pq/main.go, cmd/bob-pq/main.go, internal/deps/deps.go]
  modified: []

key-decisions:
  - "Go version directive set to 1.25.0 (auto-bumped by go mod tidy due to go-doubleratchet requiring Go 1.25)"
  - "Created internal/deps/deps.go with blank imports to prevent go mod tidy from stripping unused require directives"

patterns-established:
  - "Single go.mod at repo root — no go.work"
  - "Blank-import tracking file for dependencies not yet referenced by production code"

requirements-completed: [FOUND-01]

duration: 7min
completed: 2026-04-27
---

# Phase 1 Plan 1: Bootstrap Monorepo Summary

**Go monorepo scaffolded with pinned deps (go-doubleratchet v0.0.2, centrifuge-go v0.10.12, prometheus v1.23.2, testify v1.11.1) and four stub cmd/ binaries**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-27T10:46:58Z
- **Completed:** 2026-04-27T10:54:07Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Go module initialized with all four pinned direct dependencies
- Four stub cmd/ binaries (alice-classical, bob-classical, alice-pq, bob-pq) building successfully
- `go build ./...` and `go vet ./...` pass cleanly from repo root

## Task Commits

1. **Task 1+2: Initialise go.mod and create stub cmd/ binaries** - `b9d0621` (feat)

## Files Created/Modified
- `go.mod` - Module root with four pinned direct requires and transitive deps
- `go.sum` - Checksums for all dependencies
- `cmd/alice-classical/main.go` - Stub binary entrypoint (package main, blank main)
- `cmd/bob-classical/main.go` - Stub binary entrypoint
- `cmd/alice-pq/main.go` - Stub binary entrypoint
- `cmd/bob-pq/main.go` - Stub binary entrypoint
- `internal/deps/deps.go` - Blank-import tracking file to preserve requires after tidy

## Decisions Made
- **Go 1.25.0 minimum** instead of planned 1.23: go-doubleratchet v0.0.2 declares `go 1.25.0` in its own go.mod; `go mod tidy` auto-bumps the minimum. Go 1.26.1 toolchain on the machine handles this correctly.
- **internal/deps/deps.go** created: `go mod tidy` strips require directives for packages not imported by any source file. Since stub mains have no imports, a blank-import tracking file is the standard Go pattern to keep deps pinned.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] go mod tidy strips unused require directives**
- **Found during:** Task 1 (go.mod initialization)
- **Issue:** `go mod tidy` removes require directives not referenced by any Go source file. Stub cmd/ mains have no imports, so all four pinned deps were stripped.
- **Fix:** Created `internal/deps/deps.go` with blank imports for all four direct dependencies. This is the standard Go pattern for tracking dependencies.
- **Files modified:** internal/deps/deps.go (new)
- **Verification:** `go mod tidy` now preserves all four direct requires; `go build ./...` passes
- **Committed in:** b9d0621 (task commit)

**2. [Rule 1 - Bug] Go version auto-bumped from 1.23 to 1.25.0**
- **Found during:** Task 1 (go.mod initialization)
- **Issue:** Plan specified `go 1.23` but `go mod tidy` auto-bumped to `go 1.25.0` because go-doubleratchet v0.0.2 requires Go 1.25.0 in its own go.mod.
- **Fix:** Accepted 1.25.0 as the minimum — this is the correct behavior per Go toolchain semantics. Go 1.26.1 toolchain on the machine supports this.
- **Files modified:** go.mod (go directive)
- **Verification:** `go build ./...` and `go vet ./...` pass
- **Committed in:** b9d0621 (task commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both deviations necessary for correctness. `internal/deps/deps.go` will be replaced by actual imports in later plans. Go 1.25.0 minimum is correct per dependency requirements.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Module root buildable, all dependencies fetched and checksummed
- Ready for Plan 01-02 (internal/keys and internal/metrics packages)
- Note: `internal/deps/deps.go` can be removed once real packages import the dependencies directly

## Self-Check: PASSED

All files verified present:
- go.mod ✓
- go.sum ✓
- cmd/alice-classical/main.go ✓
- cmd/bob-classical/main.go ✓
- cmd/alice-pq/main.go ✓
- cmd/bob-pq/main.go ✓
- internal/deps/deps.go ✓
- 01-01-SUMMARY.md ✓

Commit b9d0621 verified in git log.

---
*Phase: 01-foundation-classical-session*
*Completed: 2026-04-27*
