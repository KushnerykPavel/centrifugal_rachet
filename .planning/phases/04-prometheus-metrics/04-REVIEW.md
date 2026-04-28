---
phase: 04-prometheus-metrics
reviewed: 2026-04-27T00:00:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - cmd/alice-classical/main.go
  - cmd/bob-classical/main.go
  - cmd/alice-pq/main.go
  - cmd/bob-pq/main.go
  - prometheus/prometheus.yml
findings:
  critical: 0
  warning: 4
  info: 2
  total: 6
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-04-27
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

All four binaries correctly instrument the three histogram families (`ratchet_handshake_duration_seconds`, `ratchet_encrypt_duration_seconds`, `ratchet_decrypt_duration_seconds`) and `ratchet_message_wire_bytes`. No `protocol` or `role` labels are emitted from binary code — those are properly attached only by the Prometheus scrape config. Wire-bytes measurements use the outer envelope (`raw` / `data`) on both send and receive paths, which is correct.

Two categories of issues were found:

1. **Measurement accuracy (Warning):** Both Bob binaries start their handshake timer before JSON deserialization, so `HandshakeDurationSeconds` on the responder side includes unmarshal time in addition to the cryptographic handshake.

2. **Mutex over-holding (Warning):** `Observe` calls for `DecryptDurationSeconds`, `EncryptDurationSeconds`, and `MessageWireBytes` (receive path) are made while `mu` is held. None of these observations require the session mutex — they touch only local variables and Prometheus-internal state — so they hold the lock longer than necessary, reducing concurrency.

3. **`scrape_interval: 5s` (Info):** For a demo run of exactly 5 messages that completes in well under a second, a 5-second scrape interval risks missing all non-zero observations if the process exits before the first scrape fires.

4. **`protocol`/`role` label placement (clean):** Confirmed correct — labels are scrape-config-only.

---

## Warnings

### WR-01: Bob responder handshake timer starts before JSON unmarshal

**Files:** `cmd/bob-classical/main.go:66-73`, `cmd/bob-pq/main.go:66-73`

**Issue:** `t0 := time.Now()` is set on line 66 in both bob binaries, *before* `json.Unmarshal(env.Payload, &initMsg)` on line 68. The `ResponderHandshake` call only begins on line 72. As a result, `HandshakeDurationSeconds.Observe` measures JSON deserialization + handshake together, while the Alice-side equivalent (`InitiatorHandshake`) measures only the cryptographic handshake. This makes bob vs alice latency comparisons in Prometheus misleading.

**Fix:** Move `t0` to immediately before the handshake call:

```go
// bob-classical/main.go and bob-pq/main.go — TypeInitialMsg branch

var initMsg classical.InitialMessage   // (or pq.InitialMessage for bob-pq)
if err := json.Unmarshal(env.Payload, &initMsg); err != nil {
    log.Printf("bob-classical: unmarshal initial_msg: %v", err)
    return
}
t0 := time.Now()                              // moved to here
s, err := classical.ResponderHandshake(priv, initMsg)
metrics.HandshakeDurationSeconds.Observe(time.Since(t0).Seconds())
```

---

### WR-02: `Observe` calls for Decrypt and MessageWireBytes made while session mutex is held (receive path)

**Files:**
- `cmd/alice-classical/main.go:86-87`
- `cmd/bob-classical/main.go:99-100`
- `cmd/alice-pq/main.go:87-88`
- `cmd/bob-pq/main.go:99-100`

**Issue:** On the receive path, `DecryptDurationSeconds.Observe(...)` and `MessageWireBytes.Observe(float64(len(data)))` are both called inside the `mu.Lock()` / `mu.Unlock()` block. `Observe` acquires Prometheus-internal locks. `len(data)` is a local variable — it does not require the session mutex at all. `DecryptDurationSeconds.Observe` only needs the already-computed `time.Since(t0)` value, which is also a local. Holding `mu` across these calls unnecessarily serializes concurrent goroutines that might be processing different messages.

**Fix:** Move both `Observe` calls to after `mu.Unlock()`:

```go
mu.Lock()
t0 := time.Now()
plain, err := sess.Decrypt(&msg)
elapsed := time.Since(t0)
mu.Unlock()

metrics.DecryptDurationSeconds.Observe(elapsed.Seconds())
metrics.MessageWireBytes.Observe(float64(len(data)))

if err != nil {
    log.Printf("...: Decrypt: %v", err)
    return
}
```

---

### WR-03: `Observe` calls for Encrypt made while session mutex is held (send path)

**Files:**
- `cmd/alice-classical/main.go:147-149`
- `cmd/bob-classical/main.go:121-123`
- `cmd/alice-pq/main.go:155-157`
- `cmd/bob-pq/main.go:123-125`

**Issue:** `EncryptDurationSeconds.Observe(...)` is called inside `mu.Lock()`. The observation value is derived from `time.Since(t0)`, a local variable that does not require the mutex. This is the same class of problem as WR-02.

**Fix:** Capture elapsed before unlocking, then observe after:

```go
mu.Lock()
t0 := time.Now()
msg, err := sess.Encrypt(payloadJSON)
elapsed := time.Since(t0)
mu.Unlock()

metrics.EncryptDurationSeconds.Observe(elapsed.Seconds())

if err != nil {
    log.Fatalf("...: Encrypt msg %d: %v", i, err)
}
```

---

### WR-04: 5-second scrape interval may miss all observations for a short-lived demo run

**File:** `prometheus/prometheus.yml:2`

**Issue:** `scrape_interval: 5s` is the only scrape timing configured. The demo runs exchange exactly 5 messages and then calls `os.Exit(0)`. If the entire run (connect → handshake → 5 messages → exit) completes in under 5 seconds — which is plausible on a local Docker network — Prometheus may not fire a single scrape before the process disappears, resulting in empty histograms in Grafana.

**Fix (option A — tighter interval):**
```yaml
global:
  scrape_interval: 1s
```

**Fix (option B — per-job override for demo targets):**
```yaml
scrape_configs:
  - job_name: alice-classical
    scrape_interval: 1s
    static_configs:
      - targets: ["alice-classical:9091"]
        labels:
          protocol: classical
          role: alice
```

Option A is simpler and appropriate for a demo environment.

---

## Info

### IN-01: `MessageWireBytes` histogram mixes inbound and outbound observations without distinction

**Files:** all four `main.go` send and receive paths

**Issue:** The same `ratchet_message_wire_bytes` histogram is observed for both messages sent by a binary and messages received by it. For the Alice binaries this means outbound Alice→Bob messages and inbound echo replies both land in the same bucket series. While this is not a bug, it makes the histogram ambiguous — a spike in large values could be outbound ratchet headers or inbound echoes. The metrics package comment ("Wire size of serialized ratchet messages") does not clarify direction.

**Suggestion:** Either add a `direction` label to the histogram (`sent`/`received`), or document clearly that the histogram intentionally aggregates both directions for a "total message wire size" picture.

---

### IN-02: No `protocol`/`role` labels in binary code — confirmed correct, but scrape-config labels create a gotcha

**File:** `prometheus/prometheus.yml:8-10`, `15-17`, `22-24`, `29-31`

**Issue:** Labels `protocol` and `role` are attached via Prometheus `static_configs.labels`, which means they appear on all time series including the internal Prometheus `up`, `scrape_duration_seconds`, and `scrape_samples_*` series — but not on any custom histograms scraped from the `/metrics` endpoint. Custom histogram time series only receive the `job` and `instance` labels (plus any exported labels, of which there are none here). If a PromQL query filters on `protocol="classical"` it will match `up{...}` but will find no `ratchet_message_wire_bytes{protocol="classical"}` series.

This is a known Prometheus behaviour: `static_configs.labels` are target labels, applied to scrape metadata series. They do NOT get attached to metric series scraped from the target unless the metric is relabeled.

**Suggestion:** If queries need `protocol`/`role` filtering on histogram data, use a `metric_relabel_configs` rule to propagate those labels, or accept that `job` label (`alice-classical`, `bob-classical`, etc.) serves as the discriminator (which is sufficient and simpler).

---

_Reviewed: 2026-04-27_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
