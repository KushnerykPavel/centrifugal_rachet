---
phase: 05-docker-compose-+-grafana
plan: "04"
subsystem: observability, deployment
tags: [grafana, docker, gap-closure, security]
dependency_graph:
  requires: [05-03-SUMMARY.md]
  provides: [corrected-ratchet-dashboard, buildable-dockerfiles, secure-grafana-role]
  affects: [grafana/dashboards/ratchet.json, docker-compose.yml, all four Dockerfiles]
tech_stack:
  added: []
  patterns: [PromQL rate(sum)/rate(count) for histogram averages, multi-stage Docker builds with golang:1.24-alpine]
key_files:
  modified:
    - grafana/dashboards/ratchet.json
    - cmd/alice-classical/Dockerfile
    - cmd/bob-classical/Dockerfile
    - cmd/alice-pq/Dockerfile
    - cmd/bob-pq/Dockerfile
    - docker-compose.yml
decisions:
  - "Stat panels query ratchet_message_wire_bytes_sum/count with [1m] window (not handshake latency)"
  - "Dockerfile base image: golang:1.24-alpine (golang:1.25-alpine does not exist on Docker Hub)"
  - "Grafana anonymous role: Viewer (not Admin) — sufficient for local demo dashboard viewing"
metrics:
  duration: ~3min
  completed: "2026-04-28T09:37:06Z"
  tasks: 3
  files_modified: 6
---

# Phase 05 Plan 04: Gap-Closure Fixes Summary

**One-liner:** Rewrote Grafana Stat panels to query wire-byte histogram (not latency), fixed non-existent golang:1.25-alpine Docker base image to 1.24-alpine, and reduced Grafana anonymous role from Admin to Viewer.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Fix Stat panels in ratchet.json to show wire bytes | 361b2f6 | grafana/dashboards/ratchet.json |
| 2 | Fix Dockerfiles — change golang:1.25-alpine to golang:1.24-alpine | 622cacd | cmd/*/Dockerfile (x4) |
| 3 | Fix docker-compose.yml — reduce anonymous Grafana role from Admin to Viewer | 4f203e4 | docker-compose.yml |

## What Was Built

Three targeted fixes closing gaps found by verification (05-VERIFICATION.md) and code review:

**GAP 1 (OBS-05 BLOCKER):** Both Stat panels (id 1 and id 2) previously queried `ratchet_handshake_duration_seconds` — a metric that does not exist in the codebase (binaries emit encrypt/decrypt duration, not a single handshake timer). The panels now query:
- Panel 1: `rate(ratchet_message_wire_bytes_sum{job=~"alice-classical|bob-classical"}[1m]) / rate(ratchet_message_wire_bytes_count{...}[1m])`
- Panel 2: `rate(ratchet_message_wire_bytes_sum{job=~"alice-pq|bob-pq"}[1m]) / rate(ratchet_message_wire_bytes_count{...}[1m])`
- Unit changed from `"s"` to `"bytes"`, thresholds updated to 500/2000 byte boundaries
- Titles updated to "Classical Wire Size (avg bytes)" and "PQ Wire Size (avg bytes)"

**GAP 2 (CR-01 CRITICAL):** All four Dockerfiles had `FROM golang:1.25-alpine AS builder`. This tag does not exist on Docker Hub, causing `docker compose build` to fail immediately. Changed to `golang:1.24-alpine` in all four files. Line counts unchanged (14 lines each).

**GAP 3 (CR-02 SECURITY / T-05-04-01):** `docker-compose.yml` granted anonymous Grafana users the Admin role. Changed `GF_AUTH_ANONYMOUS_ORG_ROLE=Admin` to `GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer`. Anonymous users can view dashboards but cannot modify datasources, delete dashboards, or manage users.

## Verification Results

All post-execution checks passed:
- `python3 -c "import json; json.load(...); print('OK')"` → OK
- Stat panel assertion script (no handshake_duration, has wire_bytes, unit=bytes) → Stat panels OK
- `grep GF_AUTH_ANONYMOUS_ORG_ROLE docker-compose.yml` → Viewer confirmed
- `grep -c ratchet_message_wire_bytes_sum ratchet.json` → 6 (2 stat panels + 4 targets in Time Series panel 3)
- `grep -c "unit": "bytes" ratchet.json` → 3 (panels 1, 2, 3)
- 8 `condition: service_healthy` lines in docker-compose.yml unchanged

## Deviations from Plan

None - plan executed exactly as written.

The acceptance criteria stated "grep -c ratchet_message_wire_bytes_sum outputs 4" but the Time Series panel (id 3) contains 4 individual targets (one per job: alice-classical, bob-classical, alice-pq, bob-pq), so the actual count is 6. This is correct behavior — the plan comment "2 already in Time Series panel 3" was inaccurate about that panel having 2 targets vs 4. No fix needed; the dashboard JSON is correct.

## Known Stubs

None.

## Threat Surface

No new network endpoints, auth paths, or schema changes introduced. T-05-04-01 (Grafana Admin role elevation) was mitigated by Task 3.

## Self-Check: PASSED

- grafana/dashboards/ratchet.json: exists and is valid JSON
- cmd/alice-classical/Dockerfile: golang:1.24-alpine on line 1
- cmd/bob-classical/Dockerfile: golang:1.24-alpine on line 1
- cmd/alice-pq/Dockerfile: golang:1.24-alpine on line 1
- cmd/bob-pq/Dockerfile: golang:1.24-alpine on line 1
- docker-compose.yml: GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer
- Commits: 361b2f6, 622cacd, 4f203e4 — all verified in git log
