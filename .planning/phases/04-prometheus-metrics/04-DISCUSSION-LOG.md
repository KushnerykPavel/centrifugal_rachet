# Phase 4: Prometheus Metrics - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-04-28
**Phase:** 04-prometheus-metrics
**Areas discussed:** /metrics port per binary, Wire bytes scope, prometheus.yml location

---

## /metrics Port Per Binary

| Option | Description | Selected |
|--------|-------------|----------|
| 9091/9092/9093/9094 | Non-conflicting, standard exporter range | ✓ |
| All on 9090 | Conflict in Docker, harder Prometheus config | |

| Option | Description | Selected |
|--------|-------------|----------|
| METRICS_PORT env var | Same pattern as CENTRIFUGO_URL, overridable | ✓ |
| Hardcoded per binary | Can't override without rebuild | |

---

## Wire Bytes Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Outer envelope JSON | len(json.Marshal(envelope)) — actual bytes on wire | ✓ |
| Inner message JSON only | Misses envelope overhead | |

---

## prometheus.yml Location

| Option | Description | Selected |
|--------|-------------|----------|
| prometheus/prometheus.yml + static_configs labels | One job per binary, protocol+role labels at scrape level | ✓ |
| Different structure | relabel_configs or other directory | |

---

## Claude's Discretion

- startMetricsServer helper vs inline goroutine
- Exact timer placement
- prekey_bundle wire size NOT observed (only ratchet_msg)

## Deferred Ideas

None.
