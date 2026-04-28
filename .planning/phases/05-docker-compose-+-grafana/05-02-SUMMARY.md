---
phase: 05-docker-compose-+-grafana
plan: "02"
subsystem: deployment
tags: [docker, docker-compose, healthcheck, service-ordering, grafana, prometheus]
dependency_graph:
  requires: [cmd/alice-classical/Dockerfile, cmd/bob-classical/Dockerfile, cmd/alice-pq/Dockerfile, cmd/bob-pq/Dockerfile, centrifugo/config.json, prometheus/prometheus.yml]
  provides: [docker-compose.yml]
  affects: [grafana provisioning (plan 05-03), grafana dashboard (plan 05-03)]
tech_stack:
  added: [centrifugo/centrifugo:latest, prom/prometheus:latest, grafana/grafana:latest]
  patterns: [depends_on with condition service_healthy, health-ordered startup, anonymous grafana access for demo]
key_files:
  created:
    - docker-compose.yml
  modified: []
decisions:
  - "8 total service_healthy conditions (plan estimated 6, but prometheus and grafana also use condition: service_healthy — correct per spec)"
  - "client binaries use restart: on-failure (exit after 5 echoes); infrastructure uses restart: unless-stopped"
  - "All service names match prometheus.yml scrape targets exactly — no renaming needed"
  - "Grafana anonymous Admin access via GF_AUTH_ANONYMOUS_ENABLED + GF_AUTH_ANONYMOUS_ORG_ROLE for demo"
metrics:
  duration: 2min
  completed: "2026-04-28T09:08:22Z"
  tasks_completed: 1
  files_created: 1
---

# Phase 05 Plan 02: docker-compose.yml Summary

**One-liner:** Seven-service Docker Compose stack with healthcheck-enforced startup ordering — centrifugo starts first, Bob roles wait for centrifugo health, Alice roles wait for centrifugo and their paired Bob, Prometheus and Grafana wait for centrifugo.

## What Was Built

`docker-compose.yml` at repo root defining all seven services satisfying DEPL-01 and DEPL-02.

### Services and Startup Order

1. **centrifugo** — starts first with a wget healthcheck on `/health` (port 8000)
2. **bob-classical** and **bob-pq** — depend on `centrifugo: service_healthy`
3. **alice-classical** — depends on `centrifugo: service_healthy` AND `bob-classical: service_healthy`
4. **alice-pq** — depends on `centrifugo: service_healthy` AND `bob-pq: service_healthy`
5. **prometheus** — depends on `centrifugo: service_healthy`
6. **grafana** — depends on `centrifugo: service_healthy`

### Port Assignments

| Service | Host Port | Container Port | Purpose |
|---------|-----------|----------------|---------|
| centrifugo | 8000 | 8000 | WebSocket + health endpoint |
| alice-classical | 9091 | 9091 | Prometheus metrics |
| bob-classical | 9092 | 9092 | Prometheus metrics |
| alice-pq | 9093 | 9093 | Prometheus metrics |
| bob-pq | 9094 | 9094 | Prometheus metrics |
| prometheus | 9090 | 9090 | Metrics scraping |
| grafana | 3000 | 3000 | Dashboard UI |

### Volume Mounts

- `centrifugo`: `./centrifugo:/centrifugo` — config.json mount
- `prometheus`: `./prometheus:/etc/prometheus` — prometheus.yml mount
- `grafana provisioning`: `./grafana/provisioning:/etc/grafana/provisioning`
- `grafana dashboards`: `./grafana/dashboards:/var/lib/grafana/dashboards`

### Environment Variables

All four client binaries receive:
- `CENTRIFUGO_URL=ws://centrifugo:8000/connection/websocket`
- `METRICS_PORT=<their respective port>`

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | docker-compose.yml with all seven services and health-ordered startup | 0ef6da3 | docker-compose.yml |

## Deviations from Plan

### Minor Discrepancy: service_healthy count

The plan's automated verify step expected `grep -c "condition: service_healthy"` to return 6, but the actual file has 8 occurrences. The plan text correctly specifies that prometheus and grafana both depend on centrifugo with `condition: service_healthy` — which adds 2 more conditions beyond the 6 inter-client dependencies. The file is correct per the specification; the verification check had an off-by-two in its expected count. No change was made to the file content.

## Known Stubs

None — docker-compose.yml is complete and fully functional. Grafana provisioning files and dashboard JSON referenced by volume mounts will be created in plan 05-03.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: information_disclosure | docker-compose.yml | GF_AUTH_ANONYMOUS_ENABLED=true exposes Grafana without auth — intentional for demo (D-10) |
| threat_flag: insecure_service | docker-compose.yml | centrifugo client.insecure=true in config.json mounted at /centrifugo — intentional for demo (T-05-02-01) |

Both threats are accepted per the plan's threat model as this is a blog demo, not a production service.

## Self-Check: PASSED

- docker-compose.yml — FOUND at repo root
- Commit 0ef6da3 — verified
- 8 `condition: service_healthy` lines present
- GF_AUTH_ANONYMOUS_ENABLED=true — present
- GF_AUTH_ANONYMOUS_ORG_ROLE=Admin — present
- All four client ports 9091-9094 present
- alice-classical depends_on bob-classical — present (line 45)
- alice-pq depends_on bob-pq — present
