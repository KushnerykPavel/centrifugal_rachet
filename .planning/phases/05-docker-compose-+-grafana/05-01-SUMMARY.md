---
phase: 05-docker-compose-+-grafana
plan: "01"
subsystem: deployment
tags: [docker, dockerfile, multi-stage, healthcheck]
dependency_graph:
  requires: []
  provides: [cmd/alice-classical/Dockerfile, cmd/bob-classical/Dockerfile, cmd/alice-pq/Dockerfile, cmd/bob-pq/Dockerfile]
  affects: [docker-compose.yml (plan 05-02)]
tech_stack:
  added: [golang:1.25-alpine builder, alpine:latest final stage, wget healthcheck]
  patterns: [multi-stage Dockerfile, CGO_ENABLED=0 static binary]
key_files:
  created:
    - cmd/alice-classical/Dockerfile
    - cmd/bob-classical/Dockerfile
    - cmd/alice-pq/Dockerfile
    - cmd/bob-pq/Dockerfile
  modified: []
decisions:
  - "golang:1.25-alpine used as builder (not 1.23) — go.mod enforces Go 1.25.0 from Phase 1"
  - "wget installed in final alpine stage for HEALTHCHECK — not present in bare alpine"
  - "ca-certificates included for centrifuge-go TLS WebSocket dial"
  - "Build context is repo root — go.mod accessible from COPY go.mod go.sum ./"
metrics:
  duration: 3min
  completed: "2026-04-28T09:05:53Z"
  tasks_completed: 2
  files_created: 4
---

# Phase 05 Plan 01: Dockerfiles for Four Client Binaries Summary

**One-liner:** Multi-stage Dockerfiles for all four client binaries using golang:1.25-alpine builder and alpine final stage with CGO_ENABLED=0 and wget-based healthchecks on metrics ports.

## What Was Built

Four multi-stage Dockerfiles — one per `cmd/` directory — satisfying DEPL-03. Each Dockerfile:

1. Uses `golang:1.25-alpine` as the builder stage (matching go.mod's go 1.25.0 requirement)
2. Uses `alpine:latest` as the minimal final stage
3. Sets `CGO_ENABLED=0` in the `go build` command to produce a fully static binary
4. Installs `ca-certificates` and `wget` in the final stage (ca-certs for TLS, wget for HEALTHCHECK)
5. EXPOSEs the binary's metrics port and declares a HEALTHCHECK via `wget` on that port
6. Uses repo root as build context so `go.mod` is accessible

Port assignments:
| Binary | Metrics Port |
|--------|-------------|
| alice-classical | 9091 |
| bob-classical | 9092 |
| alice-pq | 9093 |
| bob-pq | 9094 |

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Dockerfiles for classical pair | 4735020 | cmd/alice-classical/Dockerfile, cmd/bob-classical/Dockerfile |
| 2 | Dockerfiles for PQ pair | 49bd7ee | cmd/alice-pq/Dockerfile, cmd/bob-pq/Dockerfile |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — Dockerfiles are complete and fully functional.

## Threat Flags

No new security surface introduced beyond what is documented in the plan's threat model (T-05-01-01 through T-05-01-03 accepted as demo scope).

## Self-Check: PASSED

- cmd/alice-classical/Dockerfile — FOUND, contains golang:1.25-alpine and CGO_ENABLED=0
- cmd/bob-classical/Dockerfile — FOUND, contains golang:1.25-alpine and CGO_ENABLED=0
- cmd/alice-pq/Dockerfile — FOUND, contains golang:1.25-alpine and CGO_ENABLED=0
- cmd/bob-pq/Dockerfile — FOUND, contains golang:1.25-alpine and CGO_ENABLED=0
- Commits 4735020 and 49bd7ee verified in git log
