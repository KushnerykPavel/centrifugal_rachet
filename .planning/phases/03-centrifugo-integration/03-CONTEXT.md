# Phase 3: Centrifugo Integration - Context

**Gathered:** 2026-04-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Wire all four `cmd/` binaries into live Centrifugo pub/sub. Each binary connects to Centrifugo, performs in-band key exchange (Bob publishes prekey bundle, Alice waits up to 30s, handshakes, then both exchange 5 ratchet messages), and exits cleanly. No metrics instrumentation (Phase 4), no Docker compose (Phase 5). Phase 3 delivers the real protocol over a real WebSocket transport.

</domain>

<decisions>
## Implementation Decisions

### Message Envelope (CENT-02)

- **D-01:** New package `internal/protocol` with `envelope.go`. All four cmd/ binaries import it. Keeps envelope struct in one place — changes propagate without touching 4 binaries.
- **D-02:** Envelope struct:
  ```go
  // envelope.go
  package protocol

  import "encoding/json"

  // Envelope is the JSON wrapper for all Centrifugo channel messages.
  // Type field drives dispatch; Payload carries the inner struct as raw JSON.
  type Envelope struct {
      Type    string          `json:"type"`
      Payload json.RawMessage `json:"payload"`
  }

  // Type constants
  const (
      TypePrekeyBundle = "prekey_bundle"
      TypeInitialMsg   = "initial_msg"
      TypeRatchetMsg   = "ratchet_msg"
  )
  ```
- **D-03:** Sender: `json.Marshal(inner)` → put bytes in `Envelope.Payload`, marshal outer Envelope, publish. Receiver: unmarshal Envelope, switch on `Type`, unmarshal `Payload` into correct concrete type.

### Alice/Bob Exchange Count

- **D-04:** Alice sends **5 ratchet messages** after handshake completes. Bob echoes each one back (5 outbound + 5 echoes = 10 messages total per channel pair). After receiving the 5th echo, Alice calls `os.Exit(0)`. Bob also exits after sending the 5th echo. Provides 10 Prometheus histogram observations per binary pair — sufficient for Grafana to show trend.
- **D-05:** Ratchet message payload: `{"seq": N, "text": "hello from alice-classical N"}` (or alice-pq). Bob echo: `{"seq": N, "text": "echo: hello from alice-classical N"}`.

### Centrifugo Configuration (CENT-01)

- **D-06:** Config file: `centrifugo/config.json` — dedicated directory, Docker compose mounts `./centrifugo:/centrifugo` as volume. Centrifugo starts with `centrifugo -c /centrifugo/config.json`.
- **D-07:** Minimal config:
  ```json
  {
    "client": {
      "insecure": true
    },
    "health": true,
    "address": "0.0.0.0",
    "port": 8000,
    "log_level": "info"
  }
  ```
  No channel-level config needed — insecure mode allows any channel without tokens. `health: true` enables `GET /health` for Docker healthcheck.

### Alice Retry / 30s Wait (CENT-03)

- **D-08:** Alice uses channel + select pattern:
  ```go
  bundleCh := make(chan []byte, 1)
  // Subscribe, OnPublication sends to bundleCh (non-blocking, buffered)
  select {
  case data := <-bundleCh:
      // unmarshal and handshake
  case <-time.After(30 * time.Second):
      log.Fatalf("alice: timed out waiting for Bob's prekey bundle on %s", channel)
  }
  ```
- **D-09:** On timeout: `log.Fatalf` → `os.Exit(1)`. Clear error message naming the channel. Docker compose logs show the failure plainly.

### OnPublication Dispatch (CENT-04)

- **D-10:** Every `OnPublication` handler wraps its body in `go func() { ... }()`. No blocking call (marshal, unmarshal, publish, encrypt, decrypt) runs on the centrifuge-go read goroutine. `internal/transport.Subscribe()` already documents this requirement in its comment; Phase 3 enforces it in every binary's handler.

### Channel Isolation (CENT-05)

- **D-11:** `ch-classical` — used exclusively by alice-classical and bob-classical. `ch-pq` — used exclusively by alice-pq and bob-pq. Hardcoded channel name constants in each binary (or in `internal/protocol`).

### cmd/ Binary Structure

- **D-12:** Each binary follows this pattern:
  1. Create `transport.Client`, `Connect()`
  2. Set up subscription with `OnPublication` handler dispatching to `go func()`
  3. (Bob) Publish prekey bundle envelope, then wait for messages
  4. (Alice) Wait for prekey bundle with 30s timeout, handshake, send 5 messages
  5. Exchange complete → `os.Exit(0)`
- **D-13:** Centrifugo address: read from environment variable `CENTRIFUGO_URL` with default `ws://localhost:8000/connection/websocket`. Allows Docker compose to override without rebuilding.

### Claude's Discretion

- Error handling style within binaries (log.Printf vs log.Fatal for non-fatal errors)
- Exact test coverage for integration (binaries are integration-tested in Phase 5 via docker compose, not unit-tested in Phase 3)
- `internal/protocol` package layout beyond envelope.go (helper functions, if any)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` §Centrifugo Integration (CENT-01 through CENT-05) — exact behavioral contracts
- `.planning/PROJECT.md` §Constraints — no CGo, no go.work, single `docker compose up` entry point

### Prior Phase Patterns
- `internal/classical/classical.go` — `NewResponderBundle`, `InitiatorHandshake`, `ResponderHandshake`, `Encrypt`, `Decrypt` — exact API used in alice-classical/bob-classical
- `internal/pq/pq.go` — same API for alice-pq/bob-pq
- `internal/transport/transport.go` — `NewClient`, `Connect`, `Subscribe`, `Publish`, `Disconnect` — exact API for all 4 binaries
- `internal/metrics/metrics.go` — `/metrics` endpoint (already registered; Phase 4 calls Observe())

### Library
- `github.com/centrifugal/centrifuge-go` v0.10.12 (in go.mod) — `OnPublication` runs on read goroutine; must dispatch blocking work via `go func()`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/transport.Client` — Connect/Subscribe/Publish/Disconnect already implemented; Phase 3 just calls it
- `internal/classical.Session`, `internal/pq.Session` — Encrypt/Decrypt facades ready
- `internal/classical.NewResponderBundle`, `internal/pq.NewResponderBundle` — prekey bundle generation
- `internal/classical.InitiatorHandshake`, `internal/pq.InitiatorHandshake` — Alice-side handshake
- `internal/classical.ResponderHandshake`, `internal/pq.ResponderHandshake` — Bob-side handshake
- `internal/metrics` — Prometheus registry and `/metrics` endpoint (Phase 4 wires Observe calls)

### Established Patterns
- Error wrapping: `fmt.Errorf("component: FunctionName: %w", err)` — from all prior internal packages
- Test guard: `//go:build integration` — transport test pattern from Phase 1
- JSON serialization: `encoding/json` — established in PQ-04 test

### Integration Points
- `cmd/alice-classical` and `cmd/bob-classical` import `internal/classical`, `internal/transport`, `internal/protocol`
- `cmd/alice-pq` and `cmd/bob-pq` import `internal/pq`, `internal/transport`, `internal/protocol`
- All 4 binaries also import `internal/metrics` (starts `/metrics` server for Phase 4)

</code_context>

<specifics>
## Specific Ideas

- `internal/protocol` package name chosen over `internal/envelope` — keeps door open for future message types without renaming the package
- `CENTRIFUGO_URL` env var default `ws://localhost:8000/connection/websocket` — dev default matches Docker compose service name `centrifugo:8000`
- Bob exits after sending the 5th echo; Alice exits after receiving the 5th echo. Both exit cleanly so Docker compose can show `exited with code 0`

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 03-centrifugo-integration*
*Context gathered: 2026-04-27*
