# Phase 4: Prometheus Metrics - Context

**Gathered:** 2026-04-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Add `.Observe()` calls to all four binaries, start a `/metrics` HTTP server in each binary, and write `prometheus/prometheus.yml` with scrape targets labelled by protocol and role. No new packages — `internal/metrics` is already complete from Phase 1. Phase 4 is purely additive: instrument existing code without changing behavior.

</domain>

<decisions>
## Implementation Decisions

### /metrics HTTP Server (OBS-02, OBS-03)

- **D-01:** Each binary starts `http.ListenAndServe` on its own port:
  | Binary | Default port |
  |--------|-------------|
  | alice-classical | 9091 |
  | bob-classical | 9092 |
  | alice-pq | 9093 |
  | bob-pq | 9094 |
  Mount `metrics.Handler()` on `"/metrics"`.
- **D-02:** Port is read from `METRICS_PORT` env var with per-binary default:
  ```go
  port := os.Getenv("METRICS_PORT")
  if port == "" {
      port = "9091" // different default per binary
  }
  go http.ListenAndServe(":"+port, metrics.Handler())
  ```
  Start the HTTP server in a goroutine immediately after `Connect()`, before any subscribe/publish calls.
- **D-03:** The `/metrics` HTTP server doubles as the Docker healthcheck URL (Phase 5: `GET http://localhost:PORT/metrics`). Starting it before message exchange ensures it's up when Docker marks the container healthy.

### Wire Size Measurement (OBS-01)

- **D-04:** `ratchet_message_wire_bytes.Observe(float64(len(raw)))` where `raw` is the JSON-serialized outer `Envelope` — the exact byte slice passed to `cl.Publish()`. This is the actual bytes on the Centrifugo wire, including the envelope wrapper. Captures the full protocol overhead difference.
- **D-05:** Observe on both send (in alice binaries, after each `cl.Publish` of a `ratchet_msg`) and receive (in bob binaries, on each received `ratchet_msg` before Decrypt). Both sides observe so both `/metrics` endpoints populate the histogram.
- **D-06:** Wire bytes for `prekey_bundle` and `initial_msg` are NOT observed — only `ratchet_msg` messages count. These are the comparable messages between classical and PQ.

### Handshake Duration (OBS-02)

- **D-07:** Time the complete handshake:
  - Alice side: start timer before `InitiatorHandshake()`, stop after it returns. `metrics.HandshakeDurationSeconds.Observe(elapsed.Seconds())`
  - Bob side: start timer when `TypeInitialMsg` arrives (at top of handler goroutine), stop after `ResponderHandshake()` completes.
- **D-08:** Use `time.Now()` / `time.Since()` — no external timing library.

### Encrypt/Decrypt Duration (OBS-03)

- **D-09:** Wrap every `sess.Encrypt()` call:
  ```go
  t0 := time.Now()
  msg, err := sess.Encrypt(payloadJSON)
  metrics.EncryptDurationSeconds.Observe(time.Since(t0).Seconds())
  ```
- **D-10:** Wrap every `sess.Decrypt()` call similarly with `metrics.DecryptDurationSeconds`.
- **D-11:** Observe in ALL four binaries (alice encrypt, bob encrypt for echo, alice decrypt for echo, bob decrypt).

### prometheus.yml (OBS-04)

- **D-12:** Location: `prometheus/prometheus.yml`. Docker compose mounts `./prometheus:/etc/prometheus`.
- **D-13:** One scrape job per binary, `protocol` and `role` labels attached via `static_configs.labels`. Binaries emit NO protocol or role labels:
  ```yaml
  global:
    scrape_interval: 5s

  scrape_configs:
    - job_name: alice-classical
      static_configs:
        - targets: ["alice-classical:9091"]
          labels:
            protocol: classical
            role: alice

    - job_name: bob-classical
      static_configs:
        - targets: ["bob-classical:9092"]
          labels:
            protocol: classical
            role: bob

    - job_name: alice-pq
      static_configs:
        - targets: ["alice-pq:9093"]
          labels:
            protocol: pq
            role: alice

    - job_name: bob-pq
      static_configs:
        - targets: ["bob-pq:9094"]
          labels:
            protocol: pq
            role: bob
  ```
  Targets use Docker service names (Phase 5 sets these). For local dev, use `localhost:909X`.

### Data Race Fix (from Phase 3 CR-01/CR-02)

- **D-14:** Fix the `sess` data race in all four binaries while adding metrics instrumentation. Add a `sync.Mutex` that guards both `sess` assignment and all `Encrypt`/`Decrypt` calls. The metrics `Observe()` calls go inside the mutex-protected section (they don't need the mutex themselves, but they're adjacent to the protected code). This bundles the Phase 3 correctness fix into Phase 4 — no separate gap phase.

### Claude's Discretion

- Whether to define a `startMetricsServer(port string)` helper or inline the `go http.ListenAndServe` in each binary
- Exact placement of `time.Now()` relative to function arguments (before the call, not after)
- Whether `prekey_bundle` send also observes wire bytes (D-06 says no — only ratchet_msg)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Infrastructure
- `internal/metrics/metrics.go` — all 4 histograms already registered; Phase 4 ONLY calls `.Observe()`. Never edit this file.
- `cmd/alice-classical/main.go` — current binary structure to instrument
- `cmd/bob-classical/main.go` — current binary structure
- `cmd/alice-pq/main.go` — current binary structure
- `cmd/bob-pq/main.go` — current binary structure

### Requirements
- `.planning/REQUIREMENTS.md` §Observability (OBS-01 through OBS-04) — exact histogram names and behavioral contracts
- `.planning/PROJECT.md` §Constraints — binaries must have `/metrics` as Docker healthcheck URL (Phase 5)

### Prior Phase Decisions
- `.planning/phases/01-foundation-classical-session/01-CONTEXT.md` D-03 — all histogram names pre-registered in Phase 1; confirmed `ratchet_message_wire_bytes`, `ratchet_handshake_duration_seconds`, `ratchet_encrypt_duration_seconds`, `ratchet_decrypt_duration_seconds`
- `.planning/phases/02-pq-session/02-CONTEXT.md` D-13, D-14 — `encoding/json` for serialization; `len(json.Marshal(msg))` formula (now upgraded to outer envelope per D-04 above)
- `.planning/phases/03-centrifugo-integration/03-REVIEW.md` CR-01/CR-02 — `sess` data race fix (D-14 above bundles into Phase 4)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `metrics.MessageWireBytes`, `metrics.HandshakeDurationSeconds`, `metrics.EncryptDurationSeconds`, `metrics.DecryptDurationSeconds` — call `.Observe(float64)` directly
- `metrics.Handler()` — returns `http.Handler` for `/metrics`; mount with `http.Handle("/metrics", metrics.Handler())`

### Established Patterns
- Env var with default: `os.Getenv("CENTRIFUGO_URL")` pattern already in all 4 binaries — same pattern for `METRICS_PORT`
- `time.Now()` / `time.Since()` — standard Go timing
- `go http.ListenAndServe(":"+port, handler)` — goroutine HTTP server pattern

### Integration Points
- Phase 4 instruments `cmd/*/main.go` files already created in Phase 3
- `prometheus/prometheus.yml` is a new file consumed by Phase 5 Docker Compose
- The `/metrics` endpoint port is used by Phase 5 Dockerfile `HEALTHCHECK` directive

</code_context>

<specifics>
## Specific Ideas

- D-14 (sess mutex fix): `var mu sync.Mutex` declared alongside `var sess`. Lock before `sess = s` assignment; lock before any `Encrypt`/`Decrypt` call. This fixes CR-01/CR-02 from Phase 3 review.
- Wire bytes observation pattern:
  ```go
  raw, _ := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, msg)
  metrics.MessageWireBytes.Observe(float64(len(raw)))
  cl.Publish(ctx, sub, raw)
  ```
  Observe AFTER marshal, BEFORE publish. Same `raw` slice used for both.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 04-prometheus-metrics*
*Context gathered: 2026-04-28*
