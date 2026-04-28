# Phase 5: Docker Compose + Grafana - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-04-28
**Phase:** 05-docker-compose-+-grafana
**Areas discussed:** Dockerfile builder image, Grafana dashboard panels, Grafana provisioning layout, Dockerfile build context

---

## Dockerfile Builder Image

| Option | Description | Selected |
|--------|-------------|----------|
| golang:1.25-alpine | Matches go.mod 1.25.0 | ✓ |
| golang:1.25 | Debian, larger | |
| golang:1.23-alpine | Incompatible with go 1.25.0 | |

**Notes:** DEPL-03 originally specified 1.23-alpine but go-doubleratchet forced 1.25.0 minimum in Phase 1.

---

## Grafana Dashboard Panels

| Option | Description | Selected |
|--------|-------------|----------|
| Exact OBS-05 spec | 2 Stat + 1 Time Series + 2 latency panels | ✓ |
| Custom layout | Different arrangement | |

**Stat queries:** rate(ratchet_handshake_duration_seconds_sum/count) filtered by job label.

---

## Grafana Provisioning Layout

| Option | Description | Selected |
|--------|-------------|----------|
| grafana/provisioning/ + grafana/dashboards/ | Standard Grafana auto-provisioning | ✓ |
| Different path | | |

---

## Dockerfile Build Context

| Option | Description | Selected |
|--------|-------------|----------|
| One Dockerfile per binary, context = repo root | Clear for blog, go.mod accessible | ✓ |
| Single Dockerfile with ARG BINARY | Fewer files but harder to read | |

---

## Claude's Discretion

- Dashboard JSON grid positions
- restart policy on client binaries
- Docker network name
- Panel colors and thresholds

## Deferred Ideas

None.
