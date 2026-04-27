# Phase 1: Foundation + Classical Session - Research

**Researched:** 2026-04-27
**Domain:** Go monorepo scaffolding, X3DH + Double Ratchet (classical), Prometheus metrics, centrifuge-go transport
**Confidence:** HIGH (all critical APIs verified against v0.0.2 source and official docs)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** All four `cmd/` binaries (`alice-classical`, `bob-classical`, `alice-pq`, `bob-pq`) are stub `main()` functions only — `package main` + blank `func main() {}`. Real wiring happens in Phase 3. Only requirement: `go build ./...` succeeds.
- **D-02:** `internal/transport` test uses `//go:build integration` guard. Verifies connect/subscribe/publish/disconnect against a real Centrifugo instance but is excluded from the default `go test ./...` run.
- **D-03:** `internal/metrics` registers ALL final Phase 4 histogram names upfront: `ratchet_message_wire_bytes`, `ratchet_handshake_duration_seconds`, `ratchet_encrypt_duration_seconds`, `ratchet_decrypt_duration_seconds`. Phase 4 only calls `.Observe()`.
- **D-04 (CORRECTED — see API Discrepancy below):** Session constructor is `doubleratchet.InitAlice` / `doubleratchet.InitBob`, NOT `dr.New`.
- **D-05 (CORRECTED):** `Session.Encrypt(plaintext, ad []byte) (Message, error)` — returns value type `Message` (not pointer), and requires associated data parameter.
- **D-06 (CORRECTED):** `Session.Decrypt(msg Message, ad []byte) ([]byte, error)` — accepts value type `Message` (not pointer), requires associated data.
- **D-07:** Crypto provider is the default (nil `*Config` is accepted — passes `nil` for cfg).
- **D-08:** Library is `github.com/KushnerykPavel/go-doubleratchet v0.0.2` — pinned exact.

### Claude's Discretion

- X3DH prekey bundle struct field layout (IK, SPK, SPK_sig presence/absence) — minimal bundle sufficient for CLASS-03 unit test.
- Test helper setup for the integration-tagged transport test — Claude chooses config format.
- Exact histogram bucket boundaries — use Prometheus defaults unless constrained by Phase 4.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FOUND-01 | Single `go.mod` at root, `cmd/` for four binaries, `internal/` for shared packages, no `go.work` | Standard Go monorepo layout; verified against Go toolchain conventions |
| FOUND-02 | `internal/keys` provides `toKey32([]byte) ([32]byte, error)` helper | Simple slice-to-array conversion; length check pattern documented below |
| FOUND-03 | `internal/metrics` exposes Prometheus registry with pre-registered histograms + `/metrics` HTTP handler | `prometheus.NewRegistry()` + `promhttp.HandlerFor()` pattern verified |
| FOUND-04 | `internal/transport` wraps `centrifuge-go` client lifecycle | `centrifuge.NewJsonClient`, `Connect`, `NewSubscription`, `Subscribe`, `Publish`, `Disconnect` verified from example source |
| CLASS-01 | `internal/classical` implements X3DH using `x3dh.SendHandshake`/`x3dh.ReceiveHandshake` from the library's own subpackage | Full x3dh subpackage API verified from v0.0.2 source |
| CLASS-02 | `internal/classical` initializes DR session from X3DH shared secret; exposes `Encrypt`/`Decrypt` | `InitAlice`/`InitBob` + `Encrypt(pt, ad)`/`Decrypt(msg, ad)` verified from v0.0.2 example_test.go and x3dh_test.go |
| CLASS-03 | Unit test confirms `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` and `alice.RootKey == bob.RootKey` | RootKey is UNEXPORTED — test must compare `HandshakeResult.SharedSecret` from x3dh, not a Session field; see Critical Finding #2 |
</phase_requirements>

---

## Summary

Phase 1 creates the Go monorepo skeleton and two non-trivial packages: `internal/classical` (X3DH + Double Ratchet) and `internal/transport` (centrifuge-go wrapper). The remaining packages (`internal/keys`, `internal/metrics`) are straightforward helpers with one sharp edge each.

The single most important finding: **the locked CONTEXT.md API decisions (D-04 through D-06) do not match v0.0.2 source**. The constructor is `doubleratchet.InitAlice`/`doubleratchet.InitBob` (not `dr.New`), `Encrypt` returns a value type `Message` (not `*dr.Message`), and both `Encrypt` and `Decrypt` require an associated-data `[]byte` argument. This is verified directly from `example_test.go` and `x3dh_test.go` at tag `v0.0.2`. [VERIFIED: github.com/KushnerykPavel/go-doubleratchet v0.0.2 source]

The library ships its own `x3dh/` subpackage with `SendHandshake`/`ReceiveHandshake`. The CLASS-01 requirement says "no standalone X3DH library" — the intent is to avoid a separate third-party X3DH library, not to forbid using the subpackage bundled inside go-doubleratchet itself. Using `x3dh.SendHandshake`/`x3dh.ReceiveHandshake` satisfies CLASS-01 and is the pattern used in the library's own `TestX3DH_IntegrationWithDoubleRatchet`.

`Session.RootKey` is **not exported**. The CLASS-03 criterion "alice.RootKey == bob.RootKey" must be interpreted as comparing `HandshakeResult.SharedSecret` from the x3dh handshake (before session initialization), not a field on the Session object.

**Primary recommendation:** Use `github.com/KushnerykPavel/go-doubleratchet/x3dh` for X3DH, `doubleratchet.InitAlice`/`doubleratchet.InitBob` for DR session init, `Encrypt(pt, ad)`/`Decrypt(msg, ad)` for message exchange, and expose `HandshakeResult.SharedSecret` from `internal/classical` as `Session.RootKey` (a `[32]byte` field on the wrapper struct, not the underlying `doubleratchet.Session`).

---

## CRITICAL FINDING: API Discrepancy

**CONTEXT.md decisions D-04 through D-06 are incorrect for v0.0.2.**

| CONTEXT.md says | v0.0.2 reality | Source |
|-----------------|----------------|--------|
| `dr.New(sharedKey, bobDHPublicKey, dr.DefaultCrypto())` | `doubleratchet.InitAlice(sharedSecret []byte, bobRatchetPK [32]byte, cfg *Config) (*Session, error)` | [VERIFIED: v0.0.2 keys.go + example_test.go] |
| `dr.New` for both roles | `doubleratchet.InitAlice` (initiator) / `doubleratchet.InitBob` (responder) | [VERIFIED: v0.0.2 example_test.go] |
| `Session.Encrypt(plaintext []byte) (*dr.Message, error)` | `(s *Session) Encrypt(plaintext, ad []byte) (Message, error)` — value not pointer, requires ad | [VERIFIED: v0.0.2 session.go] |
| `Session.Decrypt(msg *dr.Message) ([]byte, error)` | `(s *Session) Decrypt(msg Message, ad []byte) ([]byte, error)` — value not pointer, requires ad | [VERIFIED: v0.0.2 session.go] |
| `dr.DefaultCrypto()` | No such function; pass `nil` for cfg to accept defaults | [VERIFIED: v0.0.2 source — no DefaultCrypto exported] |
| `Session.RootKey` accessible | `rk` is unexported; no `RootKey` field on `*Session` | [VERIFIED: v0.0.2 session_test.go — tests access private `rk`] |

**Impact on planning:** All tasks touching `internal/classical` must use the corrected API. The `internal/classical` wrapper struct should expose `RootKey [32]byte` (copied from `HandshakeResult.SharedSecret`) so CLASS-03 can compare them.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| X3DH key exchange | `internal/classical` | — | Pure crypto, no I/O |
| DR session lifecycle (Encrypt/Decrypt) | `internal/classical` | — | Wraps `doubleratchet.Session` |
| Key byte conversion | `internal/keys` | — | Shared utility, no deps |
| Prometheus registry + handler | `internal/metrics` | — | Shared by all four binaries |
| Centrifugo websocket lifecycle | `internal/transport` | — | Wraps `centrifuge-go` client |
| Binary entrypoints | `cmd/*` | — | Stub only in Phase 1 |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/KushnerykPavel/go-doubleratchet` | v0.0.2 (pinned) | Double Ratchet + x3dh subpackage | Author's fork, blog requirement |
| `github.com/centrifugal/centrifuge-go` | v0.10.12 | Centrifugo WebSocket client | Latest stable as of 2026-03-07 |
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metrics registry + HTTP handler | Latest stable as of research date |
| `golang.org/x/crypto` | (pulled by go-doubleratchet) | hkdf, x25519 — already a transitive dep | Go official extended crypto |

[VERIFIED: Go module proxy — `proxy.golang.org` confirms v0.0.2, centrifuge-go v0.10.12, prometheus v1.23.2]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `filippo.io/edwards25519` | v1.1.0 | XEdDSA in x3dh subpackage — pulled transitively | Do not import directly |
| `github.com/stretchr/testify` | v1.11.1 | Test assertions | Unit tests for classical session |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| x3dh subpackage | raw `crypto/ecdh` + custom HKDF | More code, same result; subpackage is authoritative and tested |
| `promhttp.HandlerFor(reg, ...)` | Default global registry | Custom registry avoids global state; required for blog demo isolation |

**Installation:**
```bash
go get github.com/KushnerykPavel/go-doubleratchet@v0.0.2
go get github.com/centrifugal/centrifuge-go@v0.10.12
go get github.com/prometheus/client_golang@v1.23.2
```

---

## Architecture Patterns

### System Architecture Diagram

```
[x3dh.SendHandshake / x3dh.ReceiveHandshake]
        |
        | HandshakeResult{SharedSecret [32]byte, AD []byte}
        v
[doubleratchet.InitAlice / InitBob]  ← sharedSecret[:], bobSPK.PublicKey, nil cfg
        |
        | *doubleratchet.Session
        v
[internal/classical.Session wrapper]
        |── .RootKey  [32]byte  (copy of HandshakeResult.SharedSecret)
        |── .Encrypt(plaintext []byte) (*Message, error)
        |── .Decrypt(msg *Message) ([]byte, error)
        |
        | internal/classical.Message{DR Message + AD}
        v
[internal/transport.Client]  (Phase 3 wires this; stub in Phase 1)
        |
        | centrifuge-go Publish/Subscribe → Centrifugo WebSocket
        v
[wire]
```

### Recommended Project Structure

```
.
├── go.mod                          # single module root
├── go.sum
├── cmd/
│   ├── alice-classical/main.go     # stub: package main; func main() {}
│   ├── bob-classical/main.go       # stub
│   ├── alice-pq/main.go            # stub
│   └── bob-pq/main.go              # stub
└── internal/
    ├── keys/
    │   ├── keys.go                 # toKey32()
    │   └── keys_test.go
    ├── metrics/
    │   ├── metrics.go              # NewRegistry(), handler, histograms
    │   └── metrics_test.go
    ├── transport/
    │   ├── transport.go            # Client struct wrapping centrifuge.Client
    │   └── transport_integration_test.go  # //go:build integration
    └── classical/
        ├── classical.go            # Session wrapper, Handshake func
        └── classical_test.go       # CLASS-03 unit test
```

### Pattern 1: X3DH Handshake with x3dh subpackage

**What:** Use `x3dh.SendHandshake` (Alice) and `x3dh.ReceiveHandshake` (Bob) to derive a shared secret, then initialize DR sessions.

**When to use:** CLASS-01, CLASS-02 — always use this pattern, not raw `crypto/ecdh`.

```go
// Source: github.com/KushnerykPavel/go-doubleratchet/x3dh/x3dh_test.go (TestX3DH_IntegrationWithDoubleRatchet)

// Bob's keys (generated once, published as prekey bundle)
bobIK, _ := x3dh.GenerateIdentityKey()
bobSPK, _ := x3dh.GenerateSPK(bobIK, 1)   // signs SPK with IK via XEdDSA

bundle := &x3dh.PrekeyBundle{
    IdentityKey:  bobIK.PublicKey,
    SignedPreKey: bobSPK.PublicKey,
    SPKID:        bobSPK.KeyID,
    SPKSignature: bobSPK.Signature,
    // OneTimePreKey: optional — omit for minimal bundle
}

// Alice side
aliceIK, _ := x3dh.GenerateIdentityKey()
aliceResult, initMsg, _ := x3dh.SendHandshake(aliceIK, bundle)
// aliceResult.SharedSecret [32]byte — Alice's DR root key input
// aliceResult.AD []byte — IKA_pub ‖ IKB_pub — use as associated data

// Bob side (after receiving initMsg from Alice)
bobResult, _ := x3dh.ReceiveHandshake(bobIK, &bobSPK, nil, initMsg)
// bobResult.SharedSecret == aliceResult.SharedSecret  ✓

// Bob's SPK serves as his initial DR ratchet key pair (Signal convention)
bobRatchetKP := doubleratchet.KeyPair{
    PrivateKey: bobSPK.PrivateKey,
    PublicKey:  bobSPK.PublicKey,
}

// Initialize DR sessions
aliceSess, _ := doubleratchet.InitAlice(aliceResult.SharedSecret[:], bobSPK.PublicKey, nil)
defer aliceSess.Close()
bobSess, _ := doubleratchet.InitBob(bobResult.SharedSecret[:], bobRatchetKP, nil)
defer bobSess.Close()
```

### Pattern 2: DR Encrypt/Decrypt

**What:** Use associated data from `HandshakeResult.AD` for every Encrypt/Decrypt call.

```go
// Source: github.com/KushnerykPavel/go-doubleratchet/x3dh/x3dh_test.go

ad := aliceResult.AD  // []byte — must be SAME on both sides

// Alice encrypts
msg, err := aliceSess.Encrypt([]byte("hello"), ad)
// msg is doubleratchet.Message (value type, not pointer)

// Bob decrypts
plaintext, err := bobSess.Decrypt(msg, ad)
```

### Pattern 3: internal/classical Wrapper Design

**What:** Wrap the library types behind a simpler facade that exposes `RootKey` and hides the associated-data threading.

```go
// internal/classical/classical.go

type Session struct {
    RootKey  [32]byte             // copied from HandshakeResult.SharedSecret
    ad       []byte               // stored from HandshakeResult.AD
    dr       *doubleratchet.Session
}

// Message is the wire type internal/classical exposes.
// Wraps doubleratchet.Message for JSON serialization in Phase 3.
type Message struct {
    DR doubleratchet.Message
}

func (s *Session) Encrypt(plaintext []byte) (*Message, error) {
    msg, err := s.dr.Encrypt(plaintext, s.ad)
    if err != nil {
        return nil, err
    }
    return &Message{DR: msg}, nil
}

func (s *Session) Decrypt(msg *Message) ([]byte, error) {
    return s.dr.Decrypt(msg.DR, s.ad)
}
```

This gives CLASS-03 a `Session.RootKey` field to compare, even though `doubleratchet.Session.rk` is unexported.

### Pattern 4: Prometheus Custom Registry

```go
// Source: pkg.go.dev/github.com/prometheus/client_golang/prometheus

package metrics

import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    Reg *prometheus.Registry

    MessageWireBytes         *prometheus.Histogram
    HandshakeDurationSeconds *prometheus.Histogram
    EncryptDurationSeconds   *prometheus.Histogram
    DecryptDurationSeconds   *prometheus.Histogram
)

func init() {
    Reg = prometheus.NewRegistry()

    wireBytes := prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "ratchet_message_wire_bytes",
        Help:    "Wire size of serialized ratchet messages in bytes.",
        Buckets: prometheus.DefBuckets,
    })
    handshakeDur := prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "ratchet_handshake_duration_seconds",
        Help:    "Wall time of full key exchange and session init.",
        Buckets: prometheus.DefBuckets,
    })
    encryptDur := prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "ratchet_encrypt_duration_seconds",
        Help:    "Wall time of ratchet Encrypt call.",
        Buckets: prometheus.DefBuckets,
    })
    decryptDur := prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "ratchet_decrypt_duration_seconds",
        Help:    "Wall time of ratchet Decrypt call.",
        Buckets: prometheus.DefBuckets,
    })

    Reg.MustRegister(wireBytes, handshakeDur, encryptDur, decryptDur)
    MessageWireBytes = &wireBytes
    HandshakeDurationSeconds = &handshakeDur
    EncryptDurationSeconds = &encryptDur
    DecryptDurationSeconds = &decryptDur
}

func Handler() http.Handler {
    return promhttp.HandlerFor(Reg, promhttp.HandlerOpts{})
}
```

### Pattern 5: centrifuge-go Client Lifecycle

```go
// Source: github.com/centrifugal/centrifuge-go examples/chat/main.go (verified)

client := centrifuge.NewJsonClient(
    "ws://localhost:8000/connection/websocket",
    centrifuge.Config{},  // no token for insecure mode
)
defer client.Close()

client.OnConnected(func(e centrifuge.ConnectedEvent) { /* ... */ })
client.OnDisconnected(func(e centrifuge.DisconnectedEvent) { /* ... */ })
client.OnError(func(e centrifuge.ErrorEvent) { /* ... */ })

if err := client.Connect(); err != nil { /* handle */ }

sub, err := client.NewSubscription("ch-classical", centrifuge.SubscriptionConfig{})
sub.OnPublication(func(e centrifuge.PublicationEvent) {
    // CRITICAL: must spawn goroutine for any blocking work
    go func() { /* process e.Data */ }()
})
if err := sub.Subscribe(); err != nil { /* handle */ }

// Publish
_, err = sub.Publish(context.Background(), data)

// Teardown
sub.Unsubscribe()
client.Disconnect()
```

### Pattern 6: toKey32 Helper

```go
// internal/keys/keys.go
func toKey32(b []byte) ([32]byte, error) {
    if len(b) != 32 {
        return [32]byte{}, fmt.Errorf("keys: expected 32 bytes, got %d", len(b))
    }
    var k [32]byte
    copy(k[:], b)
    return k, nil
}
```

### Anti-Patterns to Avoid

- **Calling `Encrypt`/`Decrypt` without associated data:** The v0.0.2 API requires `ad []byte` on every call. Passing `nil` is valid Go but will silently diverge from the canonical Signal construction that binds identity keys to messages. Store `HandshakeResult.AD` in the Session wrapper.
- **Treating `doubleratchet.Message` as a pointer:** It is a value type. Assigning `var m *doubleratchet.Message` and passing `m` to Decrypt will panic.
- **Blocking inside `OnPublication`:** centrifuge-go processes all handler callbacks on the read loop goroutine. Any call to `sub.Publish` or `client.Disconnect` inside a handler without `go func()` will deadlock.
- **Registering histograms in `init()` of cmd/ packages:** Histograms must live in `internal/metrics` registered against the custom registry — not the global Prometheus default registry — so `cmd/` binaries only need to call `metrics.Handler()`.
- **Using `go.work`:** Explicitly prohibited. Single `go.mod` at root.
- **Using CGo:** No CGo. `crypto/ecdh`, `golang.org/x/crypto`, and `crypto/mlkem` (stdlib) cover all crypto.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| X3DH key exchange | Custom DH + HKDF composition | `x3dh.SendHandshake` / `x3dh.ReceiveHandshake` | XEdDSA SPK signature verification is non-trivial; library has test coverage |
| XEdDSA signing of SPK | Custom XEdDSA | `x3dh.GenerateSPK(ik, keyID)` | Correct XEdDSA over X25519 requires nonce clamping; library handles it |
| Prometheus HTTP handler | Custom text encoder | `promhttp.HandlerFor(reg, promhttp.HandlerOpts{})` | Content negotiation, OpenMetrics support, compression |
| WebSocket reconnection | Custom retry loop | `centrifuge-go` built-in reconnect | Exponential backoff, connection state machine |
| Double Ratchet message skipping | Custom skipped-key cache | `doubleratchet.Session` internal | Out-of-order message handling has subtle DoS vectors (MaxSkip) |

**Key insight:** The x3dh subpackage already implements the full Signal X3DH spec including XEdDSA signature verification; hand-rolling from raw `crypto/ecdh` would replicate 200+ lines of tested code and add signature-format bugs.

---

## Common Pitfalls

### Pitfall 1: Using Wrong Constructor Names
**What goes wrong:** Code fails to compile or links to wrong API. CONTEXT.md named `dr.New` and `dr.DefaultCrypto()` — neither exist in v0.0.2.
**Why it happens:** API documentation was drafted from memory before verifying the actual tag.
**How to avoid:** Use `doubleratchet.InitAlice` / `doubleratchet.InitBob`. Pass `nil` for `*Config` to accept defaults.
**Warning signs:** `undefined: doubleratchet.New`, `undefined: doubleratchet.DefaultCrypto`

### Pitfall 2: Missing Associated Data Argument
**What goes wrong:** `alice.Encrypt(plaintext)` — compile error: too few arguments. `Encrypt` requires `(plaintext, ad []byte)`.
**Why it happens:** CONTEXT.md described the signature without the `ad` parameter.
**How to avoid:** Store `HandshakeResult.AD` in the `internal/classical.Session` struct and pass it on every call.
**Warning signs:** Compile error at Encrypt/Decrypt call sites.

### Pitfall 3: Accessing Session.RootKey Directly
**What goes wrong:** `alice.RootKey` — compile error: `doubleratchet.Session` has no exported field `RootKey`.
**Why it happens:** CLASS-03 success criterion describes it as a Session field.
**How to avoid:** The `internal/classical.Session` wrapper struct must carry `RootKey [32]byte` (populated from `HandshakeResult.SharedSecret` during handshake). The test compares `classicalSession.RootKey`, not `doubleratchetSession.rk`.
**Warning signs:** `session.RootKey undefined (type *doubleratchet.Session has no field or method RootKey)`

### Pitfall 4: Message as Pointer Where Value Expected
**What goes wrong:** `var msg *doubleratchet.Message; msg, _ = alice.Encrypt(...)` — Encrypt returns `doubleratchet.Message` (value), not `*doubleratchet.Message`.
**Why it happens:** CONTEXT.md said `(*dr.Message, error)`.
**How to avoid:** `msg, err := aliceSess.Encrypt(plaintext, ad)` — infer the type, do not pre-declare as pointer.
**Warning signs:** `cannot use Message as *Message` or nil dereference.

### Pitfall 5: Blocking in centrifuge-go Callbacks
**What goes wrong:** Calling `sub.Publish(...)` inside `sub.OnPublication(...)` handler deadlocks the read loop.
**Why it happens:** centrifuge-go v0.10.12 processes handler callbacks synchronously on the connection read goroutine.
**How to avoid:** Wrap all blocking operations in `go func() { ... }()` inside any `OnPublication` / `OnMessage` callback.
**Warning signs:** Integration test hangs; no output after first publication.

### Pitfall 6: Registering Against Default Prometheus Registry
**What goes wrong:** Using `prometheus.MustRegister(h)` registers against the global default registry, not the custom `Reg`. The `/metrics` handler returns only Go runtime metrics, not ratchet histograms.
**How to avoid:** Always call `Reg.MustRegister(h)` where `Reg` is the `*prometheus.Registry` returned by `prometheus.NewRegistry()`.
**Warning signs:** `/metrics` scrape shows only `go_*` and `process_*` metrics.

### Pitfall 7: go.mod Go Version vs go-doubleratchet go.mod
**What goes wrong:** go-doubleratchet's own `go.mod` declares `go 1.25.0`. This means importing it from a module declaring `go 1.23` may work (toolchain is backwards-compatible) but could surface toolchain warnings.
**How to avoid:** Declare `go 1.23` in the project's `go.mod` (minimum required for `crypto/mlkem` used in Phase 2). Go toolchain 1.26 is installed on this machine — no issue at runtime.
**Warning signs:** `note: module requires Go 1.25` toolchain warnings during `go build`.

---

## Code Examples

### Complete X3DH + DR session establishment (verified pattern)

```go
// Source: github.com/KushnerykPavel/go-doubleratchet/x3dh/x3dh_test.go TestX3DH_IntegrationWithDoubleRatchet

import (
    doubleratchet "github.com/KushnerykPavel/go-doubleratchet"
    "github.com/KushnerykPavel/go-doubleratchet/x3dh"
)

// --- Bob setup (run once; publish bundle) ---
bobIK, _ := x3dh.GenerateIdentityKey()
bobSPK, _ := x3dh.GenerateSPK(bobIK, 1)
bundle := &x3dh.PrekeyBundle{
    IdentityKey:  bobIK.PublicKey,
    SignedPreKey: bobSPK.PublicKey,
    SPKID:        bobSPK.KeyID,
    SPKSignature: bobSPK.Signature,
}

// --- Alice setup (after receiving bundle) ---
aliceIK, _ := x3dh.GenerateIdentityKey()
aliceResult, initMsg, _ := x3dh.SendHandshake(aliceIK, bundle)

// --- Bob completes handshake (after receiving initMsg) ---
bobResult, _ := x3dh.ReceiveHandshake(bobIK, &bobSPK, nil, initMsg)
// aliceResult.SharedSecret == bobResult.SharedSecret  ← compare these for CLASS-03

// --- DR session init ---
aliceSess, _ := doubleratchet.InitAlice(aliceResult.SharedSecret[:], bobSPK.PublicKey, nil)
defer aliceSess.Close()

bobRatchetKP := doubleratchet.KeyPair{PrivateKey: bobSPK.PrivateKey, PublicKey: bobSPK.PublicKey}
bobSess, _ := doubleratchet.InitBob(bobResult.SharedSecret[:], bobRatchetKP, nil)
defer bobSess.Close()

// --- Encrypt / Decrypt ---
ad := aliceResult.AD  // must match on both sides
msg, _ := aliceSess.Encrypt([]byte("hello"), ad)   // msg is doubleratchet.Message (value)
plain, _ := bobSess.Decrypt(msg, ad)                // plain == []byte("hello")
```

### internal/keys — toKey32

```go
// internal/keys/keys.go
package keys

import "fmt"

// ToKey32 converts a byte slice to a [32]byte array.
// Returns an error if len(b) != 32.
func ToKey32(b []byte) ([32]byte, error) {
    if len(b) != 32 {
        return [32]byte{}, fmt.Errorf("keys: expected 32 bytes, got %d", len(b))
    }
    var k [32]byte
    copy(k[:], b)
    return k, nil
}
```

### integration test guard

```go
// internal/transport/transport_integration_test.go
//go:build integration

package transport_test

import (
    "testing"
    // ...
)

func TestTransport_ConnectSubscribePublishDisconnect(t *testing.T) {
    // requires real Centrifugo at ws://localhost:8000/connection/websocket
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `doubleratchet.New(...)` (pre-release API) | `InitAlice` / `InitBob` | v0.0.2 (2026-04-26) | All session init code must use named constructors |
| Manual X3DH from `crypto/ecdh` | `x3dh.SendHandshake` / `x3dh.ReceiveHandshake` | v0.0.2 (adds x3dh subpackage) | X3DH is now provided by the library |
| Global Prometheus registry | `prometheus.NewRegistry()` + `promhttp.HandlerFor` | client_golang v1.x | Required for multi-binary metrics isolation |

**Deprecated/outdated:**
- `dr.DefaultCrypto()`: Does not exist in v0.0.2. The nil `*Config` is the default.
- `*dr.Message` (pointer): `doubleratchet.Message` is a value type in v0.0.2.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All compilation | ✓ | go1.26.1 darwin/arm64 | — |
| Docker | `internal/transport` integration test (opt-in) | ✓ | 28.4.0 | Skip with `//go:build integration` |
| Centrifugo | Integration test only | ✗ | — | `//go:build integration` guard (D-02) |

[VERIFIED: `go version`, `docker --version` on target machine]

**Missing dependencies with no fallback:** None that block Phase 1. Centrifugo is required only for the opt-in integration test.

**Missing dependencies with fallback:** Centrifugo — guarded by `//go:build integration`; default `go test ./...` passes without it.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package (go1.26.1) |
| Config file | none — `go test` built-in |
| Quick run command | `go test ./internal/...` |
| Full suite command | `go test ./... -count=1 -race` |
| Integration run | `go test ./internal/transport/... -tags=integration -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-01 | `go build ./...` succeeds, no `go.work` | build smoke | `go build ./...` | ❌ Wave 0: create go.mod |
| FOUND-02 | `toKey32` rejects wrong length, converts valid | unit | `go test ./internal/keys/... -run TestToKey32` | ❌ Wave 0 |
| FOUND-03 | `/metrics` endpoint serves histograms | unit | `go test ./internal/metrics/... -run TestMetricsHandler` | ❌ Wave 0 |
| FOUND-04 | transport connects/subscribes/publishes/disconnects | integration | `go test ./internal/transport/... -tags=integration` | ❌ Wave 0 |
| CLASS-01 | X3DH shared secrets match | unit (inside CLASS-03 test) | `go test ./internal/classical/... -run TestClassicalSession` | ❌ Wave 0 |
| CLASS-02 | Session exposes Encrypt/Decrypt with correct types | unit (compile + CLASS-03) | `go test ./internal/classical/... -run TestClassicalSession` | ❌ Wave 0 |
| CLASS-03 | `bob.Decrypt(alice.Encrypt(pt)) == pt` AND `alice.RootKey == bob.RootKey` | unit | `go test ./internal/classical/... -run TestClassicalSession` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go build ./...`
- **Per wave merge:** `go test ./internal/keys/... ./internal/metrics/... ./internal/classical/... -count=1 -race`
- **Phase gate:** Full suite green (`go test ./... -count=1 -race`) before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `go.mod` — create with `module github.com/KushnerykPavel/centrifugal-ratchet` (or chosen name), `go 1.23`
- [ ] `internal/keys/keys_test.go` — covers FOUND-02
- [ ] `internal/metrics/metrics_test.go` — covers FOUND-03
- [ ] `internal/transport/transport_integration_test.go` — covers FOUND-04 (build-tagged)
- [ ] `internal/classical/classical_test.go` — covers CLASS-01, CLASS-02, CLASS-03

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `nil` `*Config` is accepted by `InitAlice`/`InitBob` and applies safe defaults | Pattern 1 code example | Build error or panic at session init — verify by reading config.go zero-value behavior |
| A2 | `x3dh.SendHandshake` without OPK (no OneTimePreKey in bundle) is valid for minimal blog demo | Pattern 1 | Handshake may enforce OPK presence; if so, generate OPK and include in bundle |
| A3 | Module name for the new repo is author's choice — research used `github.com/KushnerykPavel/centrifugal-ratchet` as placeholder | Project structure | Any valid module path works; update go.mod accordingly |
| A4 | `doubleratchet.KeyPair` has fields `PrivateKey [32]byte` and `PublicKey [32]byte` | Pattern 1 code | Struct field name mismatch → compile error |

[A4 is LOW risk: verified from `type KeyPair = crypto.KeyPair` in keys.go and usage in x3dh_test.go shows `doubleratchet.KeyPair{PrivateKey: ..., PublicKey: ...}`]
[A1 is LOW risk: nil Config is accepted in example_test.go `ExampleInitAlice` — `InitAlice(sharedSecret, bobPub, nil)` confirmed]
[A2 is LOW risk: x3dh_test.go `TestX3DH_RoundTrip_WithoutOPK` confirms OPK-less handshake works]

---

## Open Questions (RESOLVED)

1. **Module name for the new repository**
   - What we know: No `go.mod` exists yet; PROJECT.md does not specify the module path.
   - What's unclear: Should it be `github.com/KushnerykPavel/centrifugal-ratchet` or another path?
   - Recommendation: Planner should include a task to confirm module name with author before `go mod init`; default to kebab-case of repo name.
   - **RESOLVED:** Plan 01-01 Task 1 uses `github.com/KushnerykPavel/centrifugal-ratchet` as the module path (kebab-case of repo name, matching PROJECT.md naming convention).

2. **Config.LocalIdentityKey / RemoteIdentityKey population**
   - What we know: `Config` has `LocalIdentityKey [32]byte` and `RemoteIdentityKey [32]byte` fields; session_test.go notes "identity keys either both set or both zero."
   - What's unclear: If set, they enable identity binding in AD construction. The blog demo may benefit from setting them for correctness.
   - Recommendation: Populate from handshake identity keys for correctness; pass via `&doubleratchet.Config{LocalIdentityKey: ..., RemoteIdentityKey: ...}` instead of nil.
   - **RESOLVED:** Plan 01-04 passes `nil` for `*Config` per CONTEXT.md D-07 (Claude's discretion). Blog demo simplicity takes precedence over identity binding; both sides use zero-value identity keys (both zero = valid per library convention).

---

## Sources

### Primary (HIGH confidence)

- `github.com/KushnerykPavel/go-doubleratchet` v0.0.2 tag — `session.go`, `keys.go`, `message.go`, `config.go`, `example_test.go`, `x3dh/x3dh.go`, `x3dh/keys.go`, `x3dh/x3dh_test.go` — all fetched via `raw.githubusercontent.com`
- Go module proxy `proxy.golang.org` — confirmed v0.0.2 exists; confirmed centrifuge-go v0.10.12 and prometheus/client_golang v1.23.2 as latest stable
- `github.com/centrifugal/centrifuge-go` examples/chat/main.go — verified connect/subscribe/publish/disconnect API
- `pkg.go.dev/github.com/prometheus/client_golang/prometheus` — verified `NewRegistry`, `MustRegister`, `promhttp.HandlerFor`

### Secondary (MEDIUM confidence)

- centrifuge-go README (github.com/centrifugal/centrifuge-go) — confirmed v0.10.12 latest, Centrifugo v4/v5/v6 compatibility, callback blocking warning

### Tertiary (LOW confidence)

- None

---

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — all versions verified via Go proxy; API verified from source
- Architecture: HIGH — derived directly from library test patterns and official examples
- Pitfalls: HIGH — all pitfalls derived from concrete API mismatches found during research, not speculation
- API discrepancy: HIGH — directly read v0.0.2 source; discrepancy is unambiguous

**Research date:** 2026-04-27
**Valid until:** 2026-05-27 (stable library, pinned version — changes only if v0.0.2 is yanked, which is extremely unlikely)
