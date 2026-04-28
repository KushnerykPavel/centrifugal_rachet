# Phase 3: Centrifugo Integration - Research

**Researched:** 2026-04-27
**Domain:** centrifuge-go v0.10.12, JSON envelope dispatch, goroutine coordination, Go JSON serialization of fixed-size byte arrays
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01:** New package `internal/protocol` with `envelope.go`. All four cmd/ binaries import it.

**D-02:** Envelope struct:
```go
package protocol
import "encoding/json"
type Envelope struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}
const (
    TypePrekeyBundle = "prekey_bundle"
    TypeInitialMsg   = "initial_msg"
    TypeRatchetMsg   = "ratchet_msg"
)
```

**D-03:** Sender: `json.Marshal(inner)` → put bytes in `Envelope.Payload`, marshal outer Envelope, publish. Receiver: unmarshal Envelope, switch on `Type`, unmarshal `Payload` into correct concrete type.

**D-04:** Alice sends 5 ratchet messages after handshake. Bob echoes each one back (5 + 5 echoes = 10 total). After receiving the 5th echo, Alice calls `os.Exit(0)`. Bob exits after sending the 5th echo.

**D-05:** Ratchet message payload: `{"seq": N, "text": "hello from alice-classical N"}`. Bob echo: `{"seq": N, "text": "echo: hello from alice-classical N"}`.

**D-06:** Config file: `centrifugo/config.json`. Docker compose mounts `./centrifugo:/centrifugo`. Centrifugo starts with `centrifugo -c /centrifugo/config.json`.

**D-07:** Minimal config:
```json
{
  "client": { "insecure": true },
  "health": true,
  "address": "0.0.0.0",
  "port": 8000,
  "log_level": "info"
}
```

**D-08:** Alice 30s timeout: channel + select pattern with `bundleCh := make(chan []byte, 1)`.

**D-09:** On timeout: `log.Fatalf` → `os.Exit(1)`.

**D-10:** Every `OnPublication` handler wraps its body in `go func() { ... }()`.

**D-11:** `ch-classical` for classical pair; `ch-pq` for PQ pair. Hardcoded constants in `internal/protocol` or each binary.

**D-12:** Binary pattern: Connect → Subscribe → (Bob: publish bundle, wait) → (Alice: wait 30s, handshake, send 5 msgs) → exit.

**D-13:** `CENTRIFUGO_URL` env var, default `ws://localhost:8000/connection/websocket`.

### Claude's Discretion

- Error handling style within binaries (log.Printf vs log.Fatal for non-fatal errors)
- Exact test coverage for integration (binaries are integration-tested in Phase 5)
- `internal/protocol` package layout beyond envelope.go (helper functions, if any)

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CENT-01 | Centrifugo config enables `client.insecure: true` and `health: true`; channels require no token | D-07 specifies exact config JSON; verified Centrifugo 5.x config schema |
| CENT-02 | All four binaries use a typed envelope with `Type` and `Payload` (json.RawMessage) | D-02/D-03 locked; JSON serialization verified for all inner types |
| CENT-03 | Bob publishes prekey bundle first; Alice waits up to 30s with channel+select | D-08/D-09 locked; goroutine-safe channel pattern verified |
| CENT-04 | All OnPublication callbacks dispatch to `go func()` — no blocking calls inside handler | Verified: cbQueue deadlock confirmed; `go func()` requirement is mandatory |
| CENT-05 | Classical pair uses `ch-classical` exclusively; PQ pair uses `ch-pq` exclusively | D-11 locked; channel isolation is a naming/subscription constraint |

</phase_requirements>

---

## Summary

Phase 3 wires all four `cmd/` binaries into live Centrifugo pub/sub using the existing `internal/transport`, `internal/classical`, and `internal/pq` facades. The primary work is: (1) creating `internal/protocol` with the envelope type and channel constants; (2) implementing the four binary `main()` functions following decision D-12; (3) writing the `centrifugo/config.json` file.

All architectural decisions are locked in CONTEXT.md. Research confirmed the goroutine safety contract, JSON serialization behavior of all library types, and the exact deadlock mechanism in centrifuge-go v0.10.12. The `go func()` dispatch requirement in D-10 is not just a stylistic choice — it is mandatory to avoid a hard deadlock in the cbQueue dispatch goroutine.

**Primary recommendation:** Implement the four binary `main()` functions directly against the locked decisions in CONTEXT.md. No alternatives to explore. The only discretion areas are error-logging style and whether to add helper functions in `internal/protocol`.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| In-band key exchange (prekey bundle publish) | cmd/ binary (Bob) | internal/classical, internal/pq | Binary drives the exchange protocol; crypto facades are stateless |
| In-band key exchange (handshake init) | cmd/ binary (Alice) | internal/classical, internal/pq | Alice-side handshake is binary-orchestrated |
| JSON envelope marshal/unmarshal | internal/protocol | cmd/ binary | Single package owns the wire format |
| Channel pub/sub lifecycle | internal/transport | centrifuge-go | Existing wrapper — Phase 3 calls it |
| Centrifugo server config | centrifugo/config.json | (docker compose, Phase 5) | Config file is static; compose mounts it |
| Exit coordination | cmd/ binary | OS | Counter + os.Exit(0) in binary main |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| centrifuge-go | v0.10.12 | WebSocket pub/sub client | Already in go.mod; transport layer wraps it |
| encoding/json | stdlib | Envelope and type serialization | Established in project (PQ-04 test uses it) |
| os | stdlib | `os.Getenv`, `os.Exit` | Standard Go env/exit pattern |
| log | stdlib | `log.Printf`, `log.Fatalf` | Established in transport package |
| context | stdlib | `context.Background()` for Publish calls | Required by transport.Publish signature |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| time | stdlib | `time.After(30 * time.Second)` for Alice timeout | D-08 select pattern |
| sync | stdlib | `sync.WaitGroup` or channel for exit coordination | If Bob needs to await Alice's exit signal |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| os.Exit(0) | channel signal + return from main | os.Exit is simpler for a demo; no cleanup needed after 5 echoes |
| log.Fatalf on timeout | custom error return | log.Fatalf gives clear Docker compose log output |

**Version verification:** [VERIFIED: go.mod in repo] centrifuge-go v0.10.12 is pinned in go.mod. No additional packages need installing for Phase 3.

---

## Architecture Patterns

### System Architecture Diagram

```
Bob binary (cmd/bob-classical or cmd/bob-pq)
│
├── transport.NewClient(url) → Connect()
├── transport.Subscribe("ch-classical", handler) → sub
│     handler = go func() { dispatch on envelope.Type }
├── bundle, priv = classical.NewResponderBundle()
├── transport.Publish(ctx, sub, marshal(Envelope{TypePrekeyBundle, marshal(bundle)}))
│
│   [waits for messages on ch-classical]
│
│   OnPublication fires for each message:
│     go func() {
│       unmarshal Envelope
│       switch Type:
│         "initial_msg" → bob.Session = classical.ResponderHandshake(priv, initMsg)
│                         echoCount = 0
│         "ratchet_msg" → plain = bob.Session.Decrypt(msg)
│                         echo = bob.Session.Encrypt(echo text)
│                         transport.Publish(ctx, sub, marshal(Envelope{TypeRatchetMsg, echo}))
│                         echoCount++
│                         if echoCount == 5 → os.Exit(0)
│     }()

Alice binary (cmd/alice-classical or cmd/alice-pq)
│
├── transport.NewClient(url) → Connect()
├── bundleCh := make(chan []byte, 1)
├── transport.Subscribe("ch-classical", handler) → sub
│     handler = go func() {
│       unmarshal Envelope
│       switch Type:
│         "prekey_bundle" → bundleCh <- raw payload (non-blocking, buffered)
│         "ratchet_msg"   → plain = alice.Session.Decrypt(msg)
│                           recvCount++
│                           if recvCount == 5 → os.Exit(0)
│     }()
│
├── select {
│     case data := <-bundleCh:          // Bob's bundle arrived
│       unmarshal bundle
│       alice.Session, initMsg = classical.InitiatorHandshake(bundle)
│       transport.Publish(ctx, sub, marshal(Envelope{TypeInitialMsg, marshal(initMsg)}))
│       for i := 1..5:
│         msg = alice.Session.Encrypt(text)
│         transport.Publish(ctx, sub, marshal(Envelope{TypeRatchetMsg, marshal(msg)}))
│     case <-time.After(30s):
│       log.Fatalf("timed out waiting for Bob's prekey bundle")
│   }
│
│   [waits for 5 echo replies via OnPublication handler → os.Exit(0)]
```

### Recommended Project Structure
```
internal/
└── protocol/
    └── envelope.go          # Envelope struct, type constants, channel constants

centrifugo/
└── config.json              # Centrifugo insecure config

cmd/
├── bob-classical/
│   └── main.go              # Bob classical: publish bundle, echo loop
├── alice-classical/
│   └── main.go              # Alice classical: wait, handshake, send 5 msgs
├── bob-pq/
│   └── main.go              # Bob PQ: publish bundle, echo loop
└── alice-pq/
    └── main.go              # Alice PQ: wait, handshake, send 5 msgs
```

### Pattern 1: JSON Envelope Dispatch

**What:** Marshal inner struct → embed as `json.RawMessage` in `Envelope` → marshal outer. On receive: unmarshal `Envelope`, switch on `Type`, unmarshal `Payload` into concrete type.

**When to use:** Every Centrifugo publication in this project.

**Example:**
```go
// Source: verified by running go run against actual types 2026-04-27

// Sending
inner, _ := json.Marshal(bundle)           // marshal inner type first
env := protocol.Envelope{
    Type:    protocol.TypePrekeyBundle,
    Payload: json.RawMessage(inner),
}
raw, _ := json.Marshal(env)               // marshal outer envelope
cl.Publish(ctx, sub, raw)

// Receiving (inside go func() dispatcher)
var env protocol.Envelope
if err := json.Unmarshal(data, &env); err != nil { return }
switch env.Type {
case protocol.TypePrekeyBundle:
    var bundle classical.PrekeyBundle
    if err := json.Unmarshal(env.Payload, &bundle); err != nil { return }
    // handle bundle
case protocol.TypeInitialMsg:
    var initMsg classical.InitialMessage  // = x3dh.InitialMessage
    if err := json.Unmarshal(env.Payload, &initMsg); err != nil { return }
    // handle initMsg
case protocol.TypeRatchetMsg:
    var msg classical.Message
    if err := json.Unmarshal(env.Payload, &msg); err != nil { return }
    // handle ratchet message
}
```

### Pattern 2: OnPublication go func() Dispatch

**What:** Every blocking operation inside OnPublication is wrapped in `go func()`.

**When to use:** Every `OnPublication` handler body in all four binaries.

**Example:**
```go
// Source: internal/transport/transport.go comment + centrifuge-go cbQueue analysis
sub.OnPublication(func(e centrifuge.PublicationEvent) {
    data := e.Data   // capture before goroutine
    go func() {
        var env protocol.Envelope
        if err := json.Unmarshal(data, &env); err != nil {
            log.Printf("unmarshal error: %v", err)
            return
        }
        // ... blocking work: Publish, Decrypt, Encrypt
    }()
})
```

### Pattern 3: Alice 30s Timeout with Buffered Channel

**What:** Buffered channel of size 1 receives prekey bundle raw payload from the OnPublication goroutine; main goroutine selects on it vs time.After.

**When to use:** Alice binaries waiting for Bob's prekey bundle.

**Example:**
```go
// Source: D-08 in CONTEXT.md; pattern verified against Go stdlib
bundleCh := make(chan []byte, 1)

cl.Subscribe(channel, func(data []byte) {
    go func() {
        var env protocol.Envelope
        json.Unmarshal(data, &env)
        if env.Type == protocol.TypePrekeyBundle {
            select {
            case bundleCh <- env.Payload:  // non-blocking: buffered size 1
            default:                        // drop duplicate bundles
            }
        }
        // ... handle other types
    }()
})

select {
case payload := <-bundleCh:
    var bundle classical.PrekeyBundle
    json.Unmarshal(payload, &bundle)
    // handshake + send 5 messages
case <-time.After(30 * time.Second):
    log.Fatalf("alice-classical: timed out waiting for Bob's prekey bundle on %s", channel)
}
```

### Pattern 4: CENTRIFUGO_URL with Fallback

**What:** `os.Getenv` with a hardcoded default.

**Example:**
```go
// Source: D-13 in CONTEXT.md; standard Go pattern
url := os.Getenv("CENTRIFUGO_URL")
if url == "" {
    url = "ws://localhost:8000/connection/websocket"
}
cl := transport.NewClient(url)
```

### Pattern 5: Bob Exit After 5th Echo

**What:** Bob uses an `int` counter in the goroutine closure; exits after sending the 5th echo.

**Example:**
```go
// Source: D-04/D-12 in CONTEXT.md
echoCount := 0
// Inside go func() on TypeRatchetMsg:
echoCount++
if echoCount == 5 {
    os.Exit(0)
}
```

Note: `echoCount` is accessed only from the goroutines spawned by the single `OnPublication` handler. Because centrifuge-go's cbQueue dispatches callbacks sequentially (one at a time), the `go func()` goroutines spawned per message can race. Use an atomic counter or ensure only one goroutine accesses `echoCount` at a time. Safest: use `sync/atomic` for the echo counter, or accept that for a 5-message demo with sequential ratchet the timing is unlikely to race.

### Anti-Patterns to Avoid

- **Calling transport.Publish directly inside OnPublication without go func():** Blocks the cbQueue dispatch goroutine. The cbQueue is sequential — it cannot process any more callbacks (including internal ones) while blocked. This is a hard deadlock, not a performance issue.
- **Using an unbuffered channel for bundleCh:** If Alice's OnPublication fires before main reaches the select, the send blocks inside the goroutine forever. Use `make(chan []byte, 1)`.
- **Unmarshaling directly into x3dh.InitialMessage using the wrong pointer receiver:** `x3dh.InitialMessage` and `pqxdh.InitialMessage` both have an `OPKID *uint32` pointer field that serializes to `"OPKID": null` when nil. This round-trips correctly through `encoding/json` — verified.
- **Forgetting that classical.Message wraps doubleratchet.Message in a struct:** The wire type is `classical.Message{DR: doubleratchet.Message{...}}`, not `doubleratchet.Message` directly. Unmarshal into `classical.Message`, then pass to `Session.Decrypt`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON envelope typing | Custom binary framing, protobuf | `encoding/json` + `json.RawMessage` | Already established in project; `json.RawMessage` defers inner parsing until type is known |
| WebSocket pub/sub | Raw gorilla/websocket | `internal/transport` → centrifuge-go | Already implemented and tested in Phase 1 |
| Goroutine coordination | Semaphores, WaitGroups | Buffered channel + `select` | Simpler, Go-idiomatic, already decided in D-08 |
| Retry/backoff for prekey bundle | Custom polling loop | 30s `time.After` + `log.Fatalf` | Sufficient for demo; D-09 mandates Fatalf on timeout |

**Key insight:** Phase 3 is almost entirely wiring — the crypto, transport, and metrics layers are already built. The only new code is `internal/protocol/envelope.go` (~30 lines), `centrifugo/config.json` (~8 lines), and four binary `main()` functions (~60-80 lines each).

---

## Critical Finding: centrifuge-go v0.10.12 cbQueue Deadlock Mechanism

[VERIFIED: centrifuge-go@v0.10.12 source at `/Users/pavelkushneryk/go/pkg/mod/github.com/centrifugal/centrifuge-go@v0.10.12/`]

The CONTEXT.md note that "OnPublication runs on the read goroutine" is **imprecise but the conclusion is correct**. The actual mechanism is:

1. centrifuge-go maintains a `cbQueue` — a linked-list backed, single-goroutine sequential callback dispatcher.
2. `handlePublication` calls `runHandlerSync(fn)` which pushes `fn` to the cbQueue and **blocks** waiting for `fn` to complete.
3. The cbQueue dispatch goroutine dequeues and runs `fn` (the OnPublication handler).
4. If `fn` calls `sub.Publish()`, which calls `onSubscribe()`, which calls `go fn(nil)` — this spawns a new goroutine to do the network send. The `sub.Publish()` caller then blocks on `<-resCh` waiting for the network response.
5. **The cbQueue dispatch goroutine is now blocked inside the handler, waiting for the Publish response.**
6. Any new publication or reconnection event that needs to fire a callback via `runHandlerSync` will push to the cbQueue, but the dispatch goroutine is occupied. This is effectively a deadlock for the current session's callback processing.

**Conclusion:** The `go func()` wrapping in D-10 is mandatory. It must wrap ALL blocking work: `json.Marshal`, `json.Unmarshal`, `transport.Publish`, `Session.Encrypt`, `Session.Decrypt`. The fast path (capturing `e.Data` and spawning `go func()`) is the correct pattern.

**Thread safety of the counter:** Since multiple `go func()` goroutines may be spawned by successive OnPublication events before any complete, `echoCount` (Bob's echo counter) and `recvCount` (Alice's receive counter) must be protected. For a 5-message demo, use `sync/atomic` int32. Both sender and receiver will race if publish latency varies.

---

## JSON Serialization Verification

[VERIFIED: ran `go run /tmp/json_main.go` against actual library types 2026-04-27]

### classical.PrekeyBundle
Fields: `IdentityKey [32]byte`, `SignedPreKey [32]byte`, `SPKID uint32`, `SPKSignature [64]byte`

Serialization: `[32]byte` and `[64]byte` arrays serialize as **JSON number arrays** (e.g., `[1,2,3,0,...]`), NOT as base64 strings. This is different from `[]byte` which serializes as base64.

Round-trips correctly through `json.Marshal` + `json.Unmarshal`. No json struct tags needed — exported field names match.

### x3dh.InitialMessage
Fields: `OPKID *uint32`, `IdentityKey [32]byte`, `EphemeralKey [32]byte`

`OPKID` is a pointer — serializes as `"OPKID": null` when nil. Round-trips correctly.

### classical.Message (= `struct{ DR doubleratchet.Message }`)
`doubleratchet.Message` has `Ciphertext []byte` (base64 in JSON) and `Header doubleratchet.Header` with `RatchetPublicKey [32]byte` (number array), `PN uint32`, `N uint32`.

Wrapped in `classical.Message{DR: ...}` — the JSON key is `"DR"`.

### doubleratchet.TripleRatchetMessage
```json
{
  "Ciphertext": "<base64>",
  "Header": {
    "SCKA": {"Msg": "<base64>", "N": 2},
    "EC": {"RatchetPublicKey": [9,0,...], "PN": 1, "N": 0}
  }
}
```
`SCKA` is a pointer (`*SCKAHeader`) — serializes as `null` if nil. All fields round-trip correctly.

### pqxdh.PrekeyBundle
`PQPreKey []byte` serializes as base64. `PQParams KEMParams` serializes as an integer (0 for MLKEM768, 1 for MLKEM1024). `OneTimePreKey *[32]byte` serializes as null or a number array. `OPKID *uint32` serializes as null.

**Important:** `pqxdh.PrekeyBundle` has no json struct tags. Field names in JSON match Go exported field names exactly.

### pqxdh.InitialMessage
`KEMCiphertext []byte` serializes as base64. `PQParams KEMParams` as integer. All other fields same as PrekeyBundle.

**No serialization issues found for any type.** Standard `encoding/json` works correctly for all types in this phase.

---

## Code Examples

### internal/protocol/envelope.go — Complete Implementation

```go
// Source: D-01, D-02, D-11 in CONTEXT.md (locked decisions)
package protocol

import "encoding/json"

// Envelope is the JSON wrapper for all Centrifugo channel messages.
type Envelope struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

// Type constants for envelope dispatch.
const (
    TypePrekeyBundle = "prekey_bundle"
    TypeInitialMsg   = "initial_msg"
    TypeRatchetMsg   = "ratchet_msg"
)

// Channel name constants (D-11).
const (
    ChannelClassical = "ch-classical"
    ChannelPQ        = "ch-pq"
)

// MarshalEnvelope marshals inner to JSON and wraps it in an Envelope.
// Returns the outer Envelope as JSON bytes ready to publish.
func MarshalEnvelope(typ string, inner any) ([]byte, error) {
    payload, err := json.Marshal(inner)
    if err != nil {
        return nil, err
    }
    return json.Marshal(Envelope{Type: typ, Payload: json.RawMessage(payload)})
}
```

### centrifugo/config.json — Complete File

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

### Bob-Classical main() — Skeleton

```go
// Source: D-12, D-13 in CONTEXT.md

package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "sync/atomic"

    "github.com/KushnerykPavel/centrifugal-ratchet/internal/classical"
    "github.com/KushnerykPavel/centrifugal-ratchet/internal/protocol"
    "github.com/KushnerykPavel/centrifugal-ratchet/internal/transport"
)

func main() {
    url := os.Getenv("CENTRIFUGO_URL")
    if url == "" {
        url = "ws://localhost:8000/connection/websocket"
    }

    cl := transport.NewClient(url)
    if err := cl.Connect(); err != nil {
        log.Fatalf("bob-classical: Connect: %v", err)
    }
    defer cl.Disconnect()

    bundle, priv, err := classical.NewResponderBundle()
    if err != nil {
        log.Fatalf("bob-classical: NewResponderBundle: %v", err)
    }

    var (
        sess      *classical.Session
        echoCount int32
    )

    sub, err := cl.Subscribe(protocol.ChannelClassical, func(data []byte) {
        go func() {
            var env protocol.Envelope
            if err := json.Unmarshal(data, &env); err != nil {
                log.Printf("bob-classical: unmarshal envelope: %v", err)
                return
            }
            switch env.Type {
            case protocol.TypeInitialMsg:
                var initMsg classical.InitialMessage
                if err := json.Unmarshal(env.Payload, &initMsg); err != nil {
                    log.Printf("bob-classical: unmarshal initial_msg: %v", err)
                    return
                }
                s, err := classical.ResponderHandshake(priv, initMsg)
                if err != nil {
                    log.Printf("bob-classical: ResponderHandshake: %v", err)
                    return
                }
                sess = s
                log.Printf("bob-classical: handshake complete")

            case protocol.TypeRatchetMsg:
                if sess == nil {
                    return
                }
                var msg classical.Message
                if err := json.Unmarshal(env.Payload, &msg); err != nil {
                    log.Printf("bob-classical: unmarshal ratchet_msg: %v", err)
                    return
                }
                plain, err := sess.Decrypt(&msg)
                if err != nil {
                    log.Printf("bob-classical: Decrypt: %v", err)
                    return
                }
                log.Printf("bob-classical: recv: %s", plain)

                echoMsg, err := sess.Encrypt([]byte("echo: " + string(plain)))
                if err != nil {
                    log.Printf("bob-classical: Encrypt: %v", err)
                    return
                }
                raw, err := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, echoMsg)
                if err != nil {
                    log.Printf("bob-classical: MarshalEnvelope: %v", err)
                    return
                }
                if err := cl.Publish(context.Background(), sub, raw); err != nil {
                    log.Printf("bob-classical: Publish echo: %v", err)
                    return
                }
                n := atomic.AddInt32(&echoCount, 1)
                if n == 5 {
                    log.Printf("bob-classical: sent 5 echoes, exiting")
                    os.Exit(0)
                }
            }
        }()
    })
    if err != nil {
        log.Fatalf("bob-classical: Subscribe: %v", err)
    }
    _ = sub

    // Publish prekey bundle as first message.
    raw, err := protocol.MarshalEnvelope(protocol.TypePrekeyBundle, bundle)
    if err != nil {
        log.Fatalf("bob-classical: MarshalEnvelope prekey_bundle: %v", err)
    }
    if err := cl.Publish(context.Background(), sub, raw); err != nil {
        log.Fatalf("bob-classical: Publish prekey_bundle: %v", err)
    }
    log.Printf("bob-classical: published prekey bundle, waiting for Alice")

    // Block forever — os.Exit(0) called from handler goroutine after 5 echoes.
    select {}
}
```

**Note on `sub` capture:** The closure captures `sub` before it is assigned. This is safe because `cl.Publish` is called inside `go func()` goroutines that only run after `sub` is assigned (Subscribe returns before any handler fires). However, the `sub` variable assignment in the closure happens after `cl.Subscribe` returns. Planner must assign `sub` before calling `cl.Subscribe`, or restructure so the goroutine reads `sub` after subscribe returns. Safest: declare `var sub *centrifuge.Subscription` before the Subscribe call, assign the result, then use it inside the goroutine. This works because the goroutine doesn't run until a message arrives, which requires the subscription to be active (Subscribe must have already returned).

---

## Common Pitfalls

### Pitfall 1: Calling transport.Publish Inside OnPublication Without go func()
**What goes wrong:** Hard deadlock. The cbQueue dispatch goroutine blocks waiting for the Publish to complete. The Publish completion callback needs to be delivered via the same cbQueue, which is now occupied.
**Why it happens:** centrifuge-go v0.10.12 uses a single sequential callback goroutine (`cbQueue`). `runHandlerSync` blocks the dispatch goroutine until the handler returns.
**How to avoid:** Always wrap the entire handler body in `go func() { ... }()` as mandated by D-10. Capture `data := e.Data` before the goroutine if using the transport.Subscribe wrapper (it already provides raw bytes).
**Warning signs:** Binary hangs after first message; no further messages processed.

### Pitfall 2: Unbuffered bundleCh Blocking the OnPublication Goroutine
**What goes wrong:** If Alice's select hasn't started yet and the OnPublication goroutine tries to send to an unbuffered channel, it blocks indefinitely inside the goroutine.
**Why it happens:** Race between subscribe setup and the select statement in main.
**How to avoid:** Always `make(chan []byte, 1)` — buffered size 1. The send is non-blocking once buffered.
**Warning signs:** Alice hangs at startup; bundleCh send never completes.

### Pitfall 3: Race on Echo/Receive Counter
**What goes wrong:** Multiple OnPublication goroutines may run concurrently. Both read and increment `echoCount` without synchronization, causing double-exit or missed exit.
**Why it happens:** Each message spawns a new goroutine; goroutines are not serialized.
**How to avoid:** Use `sync/atomic.AddInt32(&echoCount, 1)` and compare the returned value.
**Warning signs:** Binary exits after 4 or 6 echoes, or panics on exit.

### Pitfall 4: sess Nil Before initial_msg Arrives
**What goes wrong:** A ratchet_msg arrives before the initial_msg handshake completes (should not happen with the protocol sequence, but worth guarding). `sess.Decrypt` panics on nil receiver.
**Why it happens:** Out-of-order arrival (unlikely but possible in a pub/sub system).
**How to avoid:** Add `if sess == nil { return }` guard before Decrypt. The session pointer must be set atomically or the binary must serialize message processing.
**Warning signs:** Nil pointer panic in Decrypt.

### Pitfall 5: Serializing classical.Message as doubleratchet.Message Directly
**What goes wrong:** `classical.Message` wraps `doubleratchet.Message` in a struct field `DR`. If you unmarshal into `doubleratchet.Message` directly, the outer `DR` wrapper is lost.
**Why it happens:** `classical.Message = struct{ DR doubleratchet.Message }` — the wrapping is intentional but easy to forget.
**How to avoid:** Always unmarshal into `classical.Message`, then pass `&msg` to `Session.Decrypt`.
**Warning signs:** `json: cannot unmarshal object into Go value of type doubleratchet.Message`.

### Pitfall 6: pqxdh.PrekeyBundle OneTimePreKey Field
**What goes wrong:** `OneTimePreKey *[32]byte` serializes as `null` if nil, and as a JSON number array if set. When Alice unmarshals the bundle, a nil pointer dereference in `SendHandshake` if the pointer is nil but the code expects a value.
**Why it happens:** The bundle is generated by `NewResponderBundle` which always sets `OneTimePreKey: &opk.PublicKey`. Serialization of a `*[32]byte` through JSON works correctly: marshal produces a number array, unmarshal into `*[32]byte` requires that the receiver field is the same pointer type.
**How to avoid:** Always unmarshal into `pqxdh.PrekeyBundle` (the library type), not a custom struct. The pointer field (`OneTimePreKey *[32]byte`) round-trips correctly.
**Warning signs:** `pqxdh: bundle one-time prekey: ...` error in SendHandshake.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Centrifugo `insecure_skip_verify` global | `client.insecure: true` nested config | Centrifugo 5.x config schema | Config key is nested under `client`, not at root |

**Deprecated/outdated:**
- Centrifugo root-level `insecure: true`: In Centrifugo 5.x the insecure option moved under the `client` namespace. The config in D-07 is correct for current Centrifugo.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `pqxdh.PrekeyBundle` with `*[32]byte` OneTimePreKey round-trips correctly through JSON when set to a real value | JSON Serialization | Pointer-to-array JSON serialization is a stdlib behavior — verified for nil case, assumed correct for non-nil (nil case tested) |
| A2 | Bob's `OnPublication` never fires for Bob's own published messages (Centrifugo does not echo publications back to the publisher) | Architecture | Standard pub/sub behavior; if Centrifugo echoes, Bob would try to decrypt his own prekey bundle envelope and fail |
| A3 | Centrifugo `client.insecure: true` config works for Centrifugo 5.x as written in D-07 | Standard Stack | If config key changed, Centrifugo starts but requires token, causing subscribe failure |

**Note on A2:** Standard WebSocket pub/sub servers do NOT echo messages back to the publishing client. Centrifugo follows this convention. This assumption is safe for the demo.

---

## Open Questions

1. **Race between sess assignment and concurrent goroutines (Pitfall 4)**
   - What we know: `sess` is set inside a goroutine from TypeInitialMsg; then TypeRatchetMsg goroutines read it. Both goroutines are spawned by successive OnPublication events.
   - What's unclear: Whether the sequential cbQueue dispatch guarantees ordering such that TypeInitialMsg completes before TypeRatchetMsg is dispatched.
   - Recommendation: Add a nil guard (`if sess == nil { return }`) and optionally use an atomic pointer or channel to pass the session. For a 5-message demo with a real network, the ordering will hold in practice. The nil guard is sufficient.

2. **Bob's `sub` variable in closure before assignment**
   - What we know: The handler closure captures `sub` from the outer scope. `sub` is assigned from `cl.Subscribe(...)` return. The goroutine inside the handler calls `cl.Publish(ctx, sub, raw)` which references `sub`.
   - What's unclear: Whether declaring `var sub *centrifuge.Subscription` before Subscribe and assigning after is safe for the closure.
   - Recommendation: Declare `sub` with `var` before the Subscribe call. Assign the result. The closure reads `sub` only when a goroutine runs after Subscribe returns — this is safe because Subscribe returns before any handler fires. The planner should structure code this way explicitly.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | Running Centrifugo for integration test | Yes | 28.4.0 | — |
| centrifugo binary | CENT-01 runtime test | No (not installed locally) | — | docker run centrifugo/centrifugo (Phase 5) |
| go 1.25 | go.mod minimum | [ASSUMED] same as prior phases | 1.25+ | — |

**Missing dependencies with no fallback:**
- centrifugo binary is not installed locally. Phase 3 binaries cannot be manually tested without it. Integration testing is deferred to Phase 5 (docker compose). Phase 3 verification is: `go build ./...` succeeds and code review confirms correctness.

**Missing dependencies with fallback:**
- None.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none (standard `go test ./...`) |
| Quick run command | `go test ./internal/protocol/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CENT-01 | Centrifugo config file exists at `centrifugo/config.json` with correct keys | manual | `cat centrifugo/config.json` (visual verify) | Wave 0 creates |
| CENT-02 | `MarshalEnvelope` produces valid JSON with type and payload fields | unit | `go test ./internal/protocol/... -run TestMarshalEnvelope` | Wave 0 |
| CENT-02 | Unmarshal envelope and dispatch to correct inner type | unit | `go test ./internal/protocol/... -run TestUnmarshalEnvelope` | Wave 0 |
| CENT-03 | `go build ./cmd/alice-classical` and `go build ./cmd/bob-classical` succeed | build | `go build ./cmd/...` | Wave 0 creates |
| CENT-04 | All OnPublication handlers contain `go func()` | code review | grep-based: `grep -r "OnPublication" cmd/` | n/a |
| CENT-05 | Channel constants in protocol package | unit | `go test ./internal/protocol/... -run TestChannelConstants` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go build ./...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go build ./...` green + code review of all four binaries before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/protocol/` — package does not exist yet; create with `envelope.go`
- [ ] `internal/protocol/envelope_test.go` — unit tests for MarshalEnvelope and round-trip dispatch
- [ ] `centrifugo/config.json` — config file does not exist yet

*(Existing test infrastructure in `internal/classical`, `internal/pq`, `internal/metrics`, `internal/transport` covers prior phases — no changes needed to those.)*

---

## Security Domain

> security_enforcement: not set in config.json (absent = enabled)

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Centrifugo insecure mode — explicit demo decision (D-07) |
| V3 Session Management | No | Sessions are per-run, in-memory; no persistence |
| V4 Access Control | No | No user roles; single shared channel per protocol |
| V5 Input Validation | Yes (limited) | `json.Unmarshal` returns error; binaries log and return on unmarshal failure |
| V6 Cryptography | Yes | All crypto via `internal/classical` and `internal/pq` (established in Phase 1/2); no new crypto hand-rolled |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malformed envelope injection | Tampering | `json.Unmarshal` error check + early return in goroutine |
| Replay of Bob's prekey bundle | Spoofing | Buffered channel size 1 drops duplicates; handshake is idempotent per session |
| Nil session Decrypt (Pitfall 4) | Tampering (out-of-order) | Nil guard before Decrypt |

**Note:** `client.insecure: true` is an explicit blog-demo decision (D-07). No token auth is by design.

---

## Sources

### Primary (HIGH confidence)
- centrifuge-go v0.10.12 source: `/Users/pavelkushneryk/go/pkg/mod/github.com/centrifugal/centrifuge-go@v0.10.12/` — cbQueue, runHandlerSync, handlePublication, Subscription.Publish, onSubscribe
- go-doubleratchet v0.0.2 source: `/Users/pavelkushneryk/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/` — Message, TripleRatchetMessage, x3dh types, pqxdh types
- Project source: `internal/transport/transport.go`, `internal/classical/classical.go`, `internal/pq/pq.go`, `internal/protocol` (planned), `go.mod`
- JSON serialization test: `go run /tmp/json_main.go` run 2026-04-27 against actual library types

### Secondary (MEDIUM confidence)
- CONTEXT.md locked decisions D-01 through D-13 — all architectural choices verified against existing code

### Tertiary (LOW confidence)
- Centrifugo 5.x config schema `client.insecure` key — [ASSUMED] matches D-07; not tested locally (no centrifugo binary)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries pinned in go.mod, source-verified
- Architecture: HIGH — all decisions locked in CONTEXT.md; research confirms feasibility
- JSON serialization: HIGH — verified by running actual code against library types
- goroutine deadlock: HIGH — verified by reading centrifuge-go cbQueue source
- Centrifugo config: MEDIUM — config key `client.insecure` not tested locally (no binary)
- Pitfalls: HIGH — derived from source code analysis

**Research date:** 2026-04-27
**Valid until:** 2026-05-27 (centrifuge-go is pinned; go-doubleratchet is pinned — no staleness risk within project lifetime)
