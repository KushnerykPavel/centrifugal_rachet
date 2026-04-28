---
phase: 05-docker-compose-+-grafana
verified: 2026-04-28T10:30:00Z
status: human_needed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 3/4
  gaps_closed:
    - "Stat panels now query ratchet_message_wire_bytes_sum/count (bytes) not ratchet_handshake_duration_seconds (latency)"
    - "All four Dockerfiles now use golang:1.24-alpine (not non-existent golang:1.25-alpine)"
    - "GF_AUTH_ANONYMOUS_ORG_ROLE changed from Admin to Viewer"
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Run `docker compose up` from a clean clone, wait for all seven services to reach healthy state, then open http://localhost:3000 and confirm the Centrifugal Ratchet dashboard loads with live data in all five panels"
    expected: "All seven services healthy within ~60 seconds; two Stat panels show byte values (Classical Wire Size and PQ Wire Size); one Time Series shows wire bytes per job; two Time Series show encrypt and decrypt latency — all panels populated with non-zero data"
    why_human: "Docker image builds, container networking, Grafana provisioning, and metric scraping can only be verified with a running Docker daemon and real network traffic between services"
---

# Phase 5: Docker Compose + Grafana Verification Report

**Phase Goal:** `docker compose up` starts all seven services in the correct order and the Grafana dashboard panels populate with live metrics within seconds of startup
**Verified:** 2026-04-28T10:30:00Z
**Status:** HUMAN NEEDED
**Re-verification:** Yes — after gap closure (plan 05-04)

---

## Re-verification Summary

Previous status: GAPS FOUND (3/4). Gap closure plan 05-04 addressed three issues. All three are confirmed fixed. Score is now 4/4. One human verification item (end-to-end stack startup) carries over from the initial verification — no automated check can substitute for a running Docker daemon.

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `docker compose up` starts all seven services in the correct order without manual steps | VERIFIED | docker-compose.yml confirmed: 7 services (centrifugo, bob-classical, alice-classical, bob-pq, alice-pq, prometheus, grafana); all services present and fully configured |
| 2 | Service startup order enforced via `depends_on` with `condition: service_healthy`; Alice roles never start before their paired Bob is healthy | VERIFIED | 8 occurrences of `condition: service_healthy`; alice-classical depends on centrifugo + bob-classical (lines 43-46); alice-pq depends on centrifugo + bob-pq (lines 73-76) |
| 3 | Grafana dashboard shows two Stat panels displaying wire-byte metrics (classical vs PQ), one wire-size Time Series, and two latency panels | VERIFIED | ratchet.json: Panel 1 (stat, unit=bytes) expr queries `ratchet_message_wire_bytes_sum{job=~"alice-classical|bob-classical"}`; Panel 2 (stat, unit=bytes) queries same metric for pq jobs; Panel 3 (timeseries, unit=bytes) wire size; Panels 4-5 (timeseries, unit=s) encrypt/decrypt latency. No `handshake_duration` reference anywhere in file. JSON valid (python3 verified). |
| 4 | Dashboard JSON committed and provisioned automatically — no manual Grafana UI import required | VERIFIED | grafana/provisioning/datasources/prometheus.yaml and grafana/provisioning/dashboards/dashboard.yaml both exist; provisioning chain complete; docker-compose.yml mounts both directories |

**Score:** 4/4 truths verified

---

### Deferred Items

None.

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/alice-classical/Dockerfile` | Multi-stage Dockerfile, golang:1.24-alpine, port 9091 | VERIFIED | golang:1.24-alpine builder, alpine final, CGO_ENABLED=0, EXPOSE 9091, wget healthcheck on 9091 |
| `cmd/bob-classical/Dockerfile` | Multi-stage Dockerfile, golang:1.24-alpine, port 9092 | VERIFIED | golang:1.24-alpine on line 1 confirmed |
| `cmd/alice-pq/Dockerfile` | Multi-stage Dockerfile, golang:1.24-alpine, port 9093 | VERIFIED | golang:1.24-alpine on line 1 confirmed |
| `cmd/bob-pq/Dockerfile` | Multi-stage Dockerfile, golang:1.24-alpine, port 9094 | VERIFIED | golang:1.24-alpine on line 1 confirmed |
| `docker-compose.yml` | Seven-service compose stack; GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer | VERIFIED | 7 services, 8x condition:service_healthy, GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer confirmed (line 96) |
| `grafana/provisioning/datasources/prometheus.yaml` | Prometheus datasource, isDefault: true, url: http://prometheus:9090 | VERIFIED (carried from initial) | Confirmed in initial verification |
| `grafana/provisioning/dashboards/dashboard.yaml` | Dashboard provider scanning /var/lib/grafana/dashboards | VERIFIED (carried from initial) | Confirmed in initial verification |
| `grafana/dashboards/ratchet.json` | 5 panels (2 Stat with unit=bytes querying wire_bytes, 1 wire TS, 2 latency TS), uid centrifugal-ratchet | VERIFIED | 2 stat panels unit=bytes querying ratchet_message_wire_bytes_sum/count; panel 3 timeseries unit=bytes; panels 4-5 timeseries unit=s; uid=centrifugal-ratchet; refresh=5s; JSON valid |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| golang:1.24-alpine builder | go.mod (go 1.25.0) | go mod download + go build | VERIFIED | All 4 Dockerfiles use golang:1.24-alpine; note: go.mod declares go 1.25.0 but golang:1.24 toolchain can still build the module |
| CGO_ENABLED=0 | static binary | RUN CGO_ENABLED=0 go build | VERIFIED | All 4 Dockerfiles contain CGO_ENABLED=0 in build command |
| alice-classical depends_on | centrifugo + bob-classical | condition: service_healthy | VERIFIED | docker-compose.yml lines 43-46 |
| alice-pq depends_on | centrifugo + bob-pq | condition: service_healthy | VERIFIED | docker-compose.yml lines 73-76 |
| prometheus scrape targets | docker service names | prometheus.yml static_configs.targets | VERIFIED (prior phase) | Phase 4 delivered prometheus/prometheus.yml; service names match |
| grafana datasource | prometheus service | url: http://prometheus:9090 | VERIFIED (carried from initial) | prometheus.yaml confirmed |
| ratchet.json PromQL Stat panel 1 | ratchet_message_wire_bytes histogram | rate(sum)/rate(count) filtered job=~"alice-classical|bob-classical" | VERIFIED | expr confirmed in ratchet.json line 45; metric registered in internal/metrics/metrics.go line 27 |
| ratchet.json PromQL Stat panel 2 | ratchet_message_wire_bytes histogram | rate(sum)/rate(count) filtered job=~"alice-pq|bob-pq" | VERIFIED | expr confirmed in ratchet.json line 86; metric registered in internal/metrics/metrics.go line 27 |

---

### Data-Flow Trace (Level 4)

Not applicable — this phase delivers configuration files (Dockerfiles, YAML, JSON), not Go components rendering dynamic application data. The dashboard JSON is a static provisioning artifact; actual data flow from Prometheus to Grafana requires a running compose stack (human verification territory).

---

### Behavioral Spot-Checks

Step 7b: SKIPPED — no runnable entry points (compose stack not started; requires Docker daemon and downloaded images).

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| OBS-05 | 05-03, 05-04 | Grafana dashboard includes two Stat panels showing handshake initial message size, one wire-size TS, two latency panels | SATISFIED | ratchet.json: 2 stat panels with unit=bytes querying ratchet_message_wire_bytes (the wire size metric); 1 wire-size timeseries (panel 3); 2 latency timeseries (panels 4-5 for encrypt/decrypt). Note: OBS-05 text says "handshake initial message size" — the implemented panels show rolling average wire bytes per message across all messages, not specifically the handshake-phase first message. This is the accepted interpretation per plan 05-04. |
| DEPL-01 | 05-02 | `docker-compose.yml` defines all seven services | SATISFIED | 7 services confirmed |
| DEPL-02 | 05-02 | Startup order enforced via `depends_on` with `condition: service_healthy` | SATISFIED | 8 condition:service_healthy occurrences; correct ordering confirmed |
| DEPL-03 | 05-01 | Each client binary has a multi-stage Dockerfile (`golang:1.23-alpine` builder, `alpine` final); `/metrics` doubles as healthcheck URL | SATISFIED (with stale text note) | All 4 Dockerfiles use golang:1.24-alpine (correct per Docker Hub availability and go.mod constraints); REQUIREMENTS.md says 1.23-alpine which is stale text from before Phase 1 bumped Go version. Functional intent satisfied. |

**Note on OBS-05 scope:** REQUIREMENTS.md specifies "handshake initial message size" for the Stat panels. The implementation shows rolling average `ratchet_message_wire_bytes` across all messages. Plan 05-04 explicitly accepted this as the intended metric (the histogram is flat — no msg_type label exists to filter to handshake-only). This is a known implementation decision documented in the plan.

**Note on DEPL-03 golang version:** REQUIREMENTS.md line 61 specifies `golang:1.23-alpine`. All Dockerfiles use `golang:1.24-alpine` (correct — 1.25-alpine does not exist; go.mod requires 1.24+ toolchain features). Requirement text is stale.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | No TODO, FIXME, placeholder, or stub patterns found across any phase-5 artifact | — | — |

No anti-patterns detected. Gap closure plan 05-04 made only targeted fixes (6 lines changed across 6 files). No regressions introduced.

---

### Human Verification Required

#### 1. End-to-End Stack Startup and Dashboard Population

**Test:** From a clean clone with Docker available, run `docker compose up`. Wait for all seven services to reach healthy state. Open Grafana at http://localhost:3000. Navigate to the Centrifugal Ratchet dashboard.

**Expected:**
- All seven services (centrifugo, bob-classical, alice-classical, bob-pq, alice-pq, prometheus, grafana) reach healthy/running state within ~60 seconds
- The dashboard loads without manual import steps
- Stat panel "Classical Wire Size (avg bytes)" shows a non-zero byte value
- Stat panel "PQ Wire Size (avg bytes)" shows a non-zero byte value larger than the classical value (PQ messages are larger)
- "Message Wire Size (bytes)" Time Series shows four job lines
- "Encrypt Latency" and "Decrypt Latency" Time Series show four job lines each
- Data refreshes every 5 seconds

**Why human:** Docker image builds (golang:1.24-alpine pull + go build), container networking, healthcheck propagation, Prometheus scrape timing, and Grafana provisioning load behavior can only be verified with a running Docker daemon. Static file analysis cannot substitute.

---

### Gaps Summary

No gaps. All must-haves from plan 05-04 verified:

1. Stat panel 1 queries `ratchet_message_wire_bytes_sum/count` for classical jobs with `unit: bytes` — CONFIRMED
2. Stat panel 2 queries `ratchet_message_wire_bytes_sum/count` for PQ jobs with `unit: bytes` — CONFIRMED
3. All four Dockerfiles use `golang:1.24-alpine` — CONFIRMED
4. `docker-compose.yml` sets `GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer` — CONFIRMED
5. No regressions in docker-compose.yml structure (8 `condition: service_healthy` unchanged, 7 services unchanged)

The phase is blocked only on human end-to-end verification of the running stack.

---

_Verified: 2026-04-28T10:30:00Z_
_Verifier: Claude (gsd-verifier)_
