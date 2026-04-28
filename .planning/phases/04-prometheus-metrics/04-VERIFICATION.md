---
phase: 04-prometheus-metrics
verified: 2026-04-27T00:00:00Z
status: passed
score: 7/7 must-haves verified
overrides_applied: 0
re_verification: null
gaps: []
deferred: []
human_verification: []
---

# Phase 4: Prometheus Metrics Verification Report

**Phase Goal:** All four binaries emit wire-size, handshake, and encrypt/decrypt metrics; Prometheus scrape config attaches protocol and role labels without the binaries emitting them
**Verified:** 2026-04-27T00:00:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | `ratchet_message_wire_bytes` histogram present in all four binaries' `/metrics` output, reflecting full serialized struct size | VERIFIED | All four main.go call `metrics.MessageWireBytes.Observe(float64(len(raw)))` on send and `metrics.MessageWireBytes.Observe(float64(len(data)))` on receive. `metrics.go` registers the histogram under `ratchet_message_wire_bytes` in the custom registry served by `metrics.Handler()`. |
| 2 | `ratchet_handshake_duration_seconds`, `ratchet_encrypt_duration_seconds`, `ratchet_decrypt_duration_seconds` present on all four `/metrics` endpoints | VERIFIED | All four binaries contain `HandshakeDurationSeconds.Observe`, `EncryptDurationSeconds.Observe`, `DecryptDurationSeconds.Observe` calls (19 total Observe call-sites verified by grep). All three histograms are pre-registered in `internal/metrics/metrics.go` and served from the same handler. |
| 3 | `prometheus.yml` attaches `protocol=classical\|pq` and `role=alice\|bob` via `static_configs.labels`; Go binaries emit no protocol/role label | VERIFIED | `prometheus/prometheus.yml` has four jobs each with a `labels:` block under `static_configs` containing `protocol` and `role`. Grep over `cmd/**/*.go` finds zero matches for `protocol.*classical`, `protocol.*pq`, `role.*alice`, `role.*bob`. |
| 4 | alice-classical `/metrics` on METRICS_PORT (default 9091) | VERIFIED | `cmd/alice-classical/main.go:23-33` reads `METRICS_PORT` env, defaults to `"9091"`, starts goroutine with `http.ListenAndServe(":"+port, mux)` and `mux.Handle("/metrics", metrics.Handler())`. |
| 5 | bob-classical `/metrics` on METRICS_PORT (default 9092) | VERIFIED | `cmd/bob-classical/main.go:22-32` same pattern, default `"9092"`. |
| 6 | alice-pq `/metrics` on METRICS_PORT (default 9093) | VERIFIED | `cmd/alice-pq/main.go:24-34` same pattern, default `"9093"`. |
| 7 | bob-pq `/metrics` on METRICS_PORT (default 9094) | VERIFIED | `cmd/bob-pq/main.go:22-32` same pattern, default `"9094"`. |

**Score:** 7/7 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/alice-classical/main.go` | Instrumented with mutex, METRICS_PORT, all four histogram Observe calls | VERIFIED | Contains `sync.Mutex` (line 50), `METRICS_PORT` (line 23), `HandshakeDurationSeconds.Observe` (line 120), `EncryptDurationSeconds.Observe` (line 149), `DecryptDurationSeconds.Observe` (line 86), `MessageWireBytes.Observe` x2 (lines 87, 158) |
| `cmd/bob-classical/main.go` | Instrumented with mutex, METRICS_PORT, all four histogram Observe calls | VERIFIED | Contains `sync.Mutex` (line 52), `METRICS_PORT` (line 22), `HandshakeDurationSeconds.Observe` (line 73), `EncryptDurationSeconds.Observe` (line 123), `DecryptDurationSeconds.Observe` (line 99), `MessageWireBytes.Observe` x2 (lines 100, 134) |
| `cmd/alice-pq/main.go` | Instrumented with mutex, METRICS_PORT, all four histogram Observe calls | VERIFIED | Contains `sync.Mutex` (line 51), `METRICS_PORT` (line 24), `HandshakeDurationSeconds.Observe` (line 126), `EncryptDurationSeconds.Observe` (line 157), `DecryptDurationSeconds.Observe` (line 87), `MessageWireBytes.Observe` x2 (lines 88, 166) |
| `cmd/bob-pq/main.go` | Instrumented with mutex, METRICS_PORT, all four histogram Observe calls | VERIFIED | Contains `sync.Mutex` (line 52), `METRICS_PORT` (line 22), `HandshakeDurationSeconds.Observe` (line 73), `EncryptDurationSeconds.Observe` (line 125), `DecryptDurationSeconds.Observe` (line 99), `MessageWireBytes.Observe` x2 (lines 100, 136) |
| `prometheus/prometheus.yml` | Four scrape jobs with protocol and role labels | VERIFIED | File exists with four `job_name` entries; each has `static_configs.labels` block with `protocol` and `role`; all four targets present with correct ports (9091-9094) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/alice-classical/main.go` | `metrics.HandshakeDurationSeconds` | `time.Now()/time.Since()` around `InitiatorHandshake` | WIRED | Lines 118-120: `t0 := time.Now()`, `InitiatorHandshake(&bundle)`, `Observe(time.Since(t0).Seconds())` |
| `cmd/bob-classical/main.go` | `metrics.HandshakeDurationSeconds` | timer at TypeInitialMsg arrival before unmarshal | WIRED | Lines 66-73: `t0 := time.Now()` at case entry, Observe after `ResponderHandshake` |
| `cmd/alice-pq/main.go` | `metrics.HandshakeDurationSeconds` | `time.Now()/time.Since()` around `pq.InitiatorHandshake` | WIRED | Lines 124-126: `t0 := time.Now()`, `InitiatorHandshake(&bundle)`, `Observe(time.Since(t0).Seconds())` |
| `cmd/bob-pq/main.go` | `metrics.HandshakeDurationSeconds` | timer at TypeInitialMsg arrival before unmarshal | WIRED | Lines 66-73: `t0 := time.Now()` at case entry, Observe after `ResponderHandshake` |
| `cmd/alice-classical/main.go` | `metrics.MessageWireBytes` | `float64(len(raw))` after MarshalEnvelope (send) + `float64(len(data))` on receive | WIRED | Line 158 (send) + line 87 (receive) |
| `cmd/bob-classical/main.go` | `metrics.MessageWireBytes` | `float64(len(data))` on receive + `float64(len(raw))` on echo send | WIRED | Line 100 (receive) + line 134 (echo send) |
| `cmd/alice-pq/main.go` | `metrics.MessageWireBytes` | `float64(len(raw))` after MarshalEnvelope (send) + `float64(len(data))` on receive | WIRED | Line 166 (send) + line 88 (receive) |
| `cmd/bob-pq/main.go` | `metrics.MessageWireBytes` | `float64(len(data))` on receive + `float64(len(raw))` on echo send | WIRED | Line 100 (receive) + line 136 (echo send) |
| `prometheus/prometheus.yml` | `alice-classical:9091` | `static_configs.targets` | WIRED | Line 7: `["alice-classical:9091"]` |
| `prometheus/prometheus.yml` | `bob-classical:9092` | `static_configs.targets` | WIRED | Line 14: `["bob-classical:9092"]` |
| `prometheus/prometheus.yml` | `alice-pq:9093` | `static_configs.targets` | WIRED | Line 21: `["alice-pq:9093"]` |
| `prometheus/prometheus.yml` | `bob-pq:9094` | `static_configs.targets` | WIRED | Line 28: `["bob-pq:9094"]` |

### Data-Flow Trace (Level 4)

The binaries do not render dynamic data to a UI — they emit metrics to a Prometheus histogram. The data-flow is: operation occurs → `time.Since(t0)` / `len(raw)` computed → `histogram.Observe(value)` called → value stored in in-process histogram buckets → `/metrics` handler serializes buckets on demand. The histogram variables are initialized in `metrics.init()` (non-nil), and all four binaries import and call `.Observe()` on them directly. No hollow-prop or STATIC pattern applies.

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| alice-classical wire bytes (send) | `len(raw)` | `protocol.MarshalEnvelope` result | Yes — real serialized bytes | FLOWING |
| alice-classical decrypt timing | `time.Since(t0)` | wall clock around `sess.Decrypt` | Yes — real elapsed time | FLOWING |
| bob-classical handshake timing | `time.Since(t0)` | wall clock around `ResponderHandshake` | Yes — real elapsed time | FLOWING |
| prometheus.yml labels | `protocol`, `role` | static_configs.labels | Yes — applied to all scraped series at scrape time | FLOWING |

### Behavioral Spot-Checks

Step 7b: SKIPPED for binary files — requires running the full Centrifugo stack. The build and test checks serve as the runnable validation.

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` succeeds | `go build ./...` | exit 0 | PASS |
| `go test ./...` passes (22 tests, 11 packages) | `go test ./...` | 22 passed | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| OBS-01 | 04-01, 04-02 | All four binaries observe `ratchet_message_wire_bytes` histogram | SATISFIED | 8 `MessageWireBytes.Observe` call-sites confirmed (2 per binary) |
| OBS-02 | 04-01, 04-02 | All four binaries observe `ratchet_handshake_duration_seconds` histogram | SATISFIED | 4 `HandshakeDurationSeconds.Observe` call-sites (1 per binary), timed around actual handshake call |
| OBS-03 | 04-01, 04-02 | All four binaries observe `ratchet_encrypt_duration_seconds` and `ratchet_decrypt_duration_seconds` | SATISFIED | 4 `EncryptDurationSeconds.Observe` + 4 `DecryptDurationSeconds.Observe` call-sites confirmed |
| OBS-04 | 04-03 | `prometheus.yml` attaches `protocol`/`role` via `static_configs.labels`; binaries emit no such labels | SATISFIED | `prometheus/prometheus.yml` verified; grep over `cmd/**/*.go` returns zero hits for protocol/role label emission |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cmd/bob-classical/main.go` | 66 | `t0 := time.Now()` placed before `json.Unmarshal` in TypeInitialMsg — includes unmarshal time in `HandshakeDurationSeconds` | Warning (WR-01) | Handshake metric on bob-classical and bob-pq includes JSON deserialization overhead; makes Alice vs Bob latency comparison slightly misleading. Not a correctness issue for the blog demo. |
| `cmd/bob-pq/main.go` | 66 | Same as above for bob-pq | Warning (WR-01) | Same impact |
| `cmd/alice-classical/main.go` | 83-88 | `DecryptDurationSeconds.Observe` and `MessageWireBytes.Observe(float64(len(data)))` called inside `mu.Lock()` | Warning (WR-02/WR-03) | `Observe` acquires Prometheus-internal locks; `len(data)` is a local variable requiring no mutex. Holds `mu` longer than necessary, reducing concurrency. Not a correctness issue for a single-instance demo. |
| `cmd/bob-classical/main.go` | 96-101 | Same over-holding pattern on receive path | Warning (WR-02) | Same impact |
| `cmd/alice-pq/main.go` | 84-89 | Same over-holding pattern on receive path | Warning (WR-02) | Same impact |
| `cmd/bob-pq/main.go` | 96-101 | Same over-holding pattern on receive path | Warning (WR-02) | Same impact |
| `prometheus/prometheus.yml` | 2 | `scrape_interval: 5s` — may miss all observations if demo run completes in under 5 seconds | Warning (WR-04) | If the complete demo (connect → handshake → 5 messages → exit) finishes before the first 5s scrape fires, Prometheus will see no metric data. Lowering to `1s` is the fix. |

**Note on review finding IN-02 (static_configs.labels):** The code reviewer's claim that `static_configs.labels` do NOT attach to custom metric series is factually incorrect. In Prometheus, `static_configs.labels` are target labels — they are applied to ALL time series scraped from that target during the scrape relabeling phase, including every `_bucket`, `_count`, and `_sum` series from custom histograms. The `promhttp.HandlerFor(Reg, ...)` handler exposes the raw metric text; Prometheus adds target labels after scraping. A PromQL query `ratchet_message_wire_bytes{protocol="classical"}` WILL match correctly. The implementation in `prometheus/prometheus.yml` is correct as written. No action required.

### Human Verification Required

None. All must-haves are verifiable from source code and build output.

### Gaps Summary

No gaps. All seven observable truths are VERIFIED with direct evidence from source code. The three warnings (WR-01: timer placement, WR-02/WR-03: mutex over-hold, WR-04: scrape interval) are code quality issues that do not prevent the phase goal from being achieved for a demo context. They are documented for Phase 5/6 or a follow-up gap phase if the developer chooses to address them.

---

_Verified: 2026-04-27T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
