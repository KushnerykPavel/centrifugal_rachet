---
phase: 05-docker-compose-+-grafana
plan: "03"
subsystem: observability
tags: [grafana, prometheus, dashboard, provisioning, visualization]
dependency_graph:
  requires: [docker-compose.yml (05-02), prometheus/prometheus.yml (04-03), internal/metrics/metrics.go (04-01)]
  provides: [grafana/provisioning/datasources/prometheus.yaml, grafana/provisioning/dashboards/dashboard.yaml, grafana/dashboards/ratchet.json]
  affects: [docker compose up full stack, OBS-05 satisfied]
tech_stack:
  added: []
  patterns: [Grafana provisioning YAML, Grafana dashboard JSON schemaVersion 38, PromQL histogram rate queries]
key_files:
  created:
    - grafana/provisioning/datasources/prometheus.yaml
    - grafana/provisioning/dashboards/dashboard.yaml
    - grafana/dashboards/ratchet.json
  modified: []
decisions:
  - "Dashboard UID centrifugal-ratchet per D-16"
  - "-- Default -- datasource uid in panel JSON relies on isDefault: true in prometheus.yaml — no explicit datasource UID required"
  - "Stat panels use rate([1m]) for smoother averages; Time Series panels use rate([30s]) for more responsive rolling window"
  - "Grid layout: 2 Stat panels split 12+12 at y=0, Wire Size full-width w=24 at y=6, Encrypt+Decrypt split 12+12 at y=14"
metrics:
  duration: 1min
  completed: "2026-04-28T09:11:00Z"
  tasks_completed: 2
  files_created: 3
---

# Phase 05 Plan 03: Grafana Provisioning + Dashboard JSON Summary

**One-liner:** Three Grafana config files enabling fully automatic Prometheus datasource provisioning and five-panel ratchet dashboard load on container startup — no manual UI import required.

## What Was Built

Three files satisfying OBS-05: Grafana auto-configures its Prometheus datasource and loads the `ratchet.json` dashboard on startup via provisioning.

### Files Created

| File | Purpose |
|------|---------|
| `grafana/provisioning/datasources/prometheus.yaml` | Registers Prometheus at http://prometheus:9090 as default datasource |
| `grafana/provisioning/dashboards/dashboard.yaml` | Instructs Grafana to scan /var/lib/grafana/dashboards for JSON dashboards |
| `grafana/dashboards/ratchet.json` | Five-panel dashboard comparing classical vs PQ ratchet metrics |

### Dashboard Layout

| Panel ID | Type | Title | PromQL Window |
|----------|------|-------|---------------|
| 1 | Stat | Classical Handshake | rate([1m]) |
| 2 | Stat | PQ Handshake | rate([1m]) |
| 3 | Time Series | Message Wire Size (bytes) | rate([30s]) × 4 jobs |
| 4 | Time Series | Encrypt Latency | rate([30s]) × 4 jobs |
| 5 | Time Series | Decrypt Latency | rate([30s]) × 4 jobs |

### Provisioning Chain

```
docker-compose.yml volume mounts:
  ./grafana/provisioning → /etc/grafana/provisioning
  ./grafana/dashboards   → /var/lib/grafana/dashboards

Grafana startup:
  1. reads /etc/grafana/provisioning/datasources/prometheus.yaml → registers Prometheus default datasource
  2. reads /etc/grafana/provisioning/dashboards/dashboard.yaml   → scans /var/lib/grafana/dashboards
  3. loads /var/lib/grafana/dashboards/ratchet.json             → dashboard available at http://localhost:3000
```

### PromQL Metric Coverage

All four histogram names from `internal/metrics/metrics.go` are used:
- `ratchet_handshake_duration_seconds` — Stat panels 1 & 2
- `ratchet_message_wire_bytes` — Time Series panel 3
- `ratchet_encrypt_duration_seconds` — Time Series panel 4
- `ratchet_decrypt_duration_seconds` — Time Series panel 5

Job label selectors match `prometheus/prometheus.yml` scrape job names exactly: `alice-classical`, `bob-classical`, `alice-pq`, `bob-pq`.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Grafana provisioning YAML files (datasource + dashboard provider) | 820556d | grafana/provisioning/datasources/prometheus.yaml, grafana/provisioning/dashboards/dashboard.yaml |
| 2 | Grafana dashboard JSON with 5 panels | 261efc6 | grafana/dashboards/ratchet.json |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — all three files are complete and fully functional. PromQL expressions reference real metric names from internal/metrics/metrics.go. Datasource URL uses the real Docker Compose service name.

## Threat Flags

None — threat model for this plan has only accepted risks (anonymous Grafana admin, unauthenticated Prometheus scrape, editable dashboard) which are all intentional for the blog demo scope and documented in the plan's threat_model section.

## Self-Check: PASSED

- grafana/provisioning/datasources/prometheus.yaml — FOUND, contains url: http://prometheus:9090 and isDefault: true
- grafana/provisioning/dashboards/dashboard.yaml — FOUND, contains path: /var/lib/grafana/dashboards
- grafana/dashboards/ratchet.json — FOUND, valid JSON, 5 panels (2 stat + 3 timeseries), uid=centrifugal-ratchet, refresh=5s
- Commit 820556d — Task 1 provisioning YAMLs
- Commit 261efc6 — Task 2 dashboard JSON
