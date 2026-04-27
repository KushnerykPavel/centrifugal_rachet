# Stack Research

**Project:** Centrifugal Ratchet — classical vs post-quantum encrypted chat
**Researched:** 2026-04-27
**Go version in environment:** 1.26.1 (well above the 1.23+ minimum)

---

## Recommended Stack

### Transport: Centrifugo

**Server:** `centrifugal/centrifugo` — latest is **v6.7.1** (released 2026-04-23).
Use the official Docker image `centrifugo/centrifugo:v6` in `docker compose`.

**v6 breaking change to know:** Configuration is now hierarchical, not flat.
Channel namespaces live under `channel -> namespaces`. The old flat keys like
`"allowed_origins"` moved into `client -> allowed_origins`. SockJS was removed
(irrelevant for this project). The v5 → v6 migration guide covers env-var renames.

**Go client:** `github.com/centrifugal/centrifuge-go` — latest is **v0.10.12** (2026-03-07).
Compatible with Centrifugo v4/v5/v6 and Centrifuge >= 0.25.0.

Key API (HIGH confidence — verified via pkg.go.dev):

```go
import "github.com/centrifugal/centrifuge-go"

// Create client (JSON encoding)
client := centrifuge.NewJsonClient("ws://centrifugo:8000/connection/websocket", centrifuge.Config{})

// Wire up connection events
client.OnConnected(func(e centrifuge.ConnectedEvent) { ... })
client.OnDisconnected(func(e centrifuge.DisconnectedEvent) { ... })

// Subscribe to a channel
sub, err := client.NewSubscription("ch-classical")
sub.OnPublication(func(e centrifuge.PublicationEvent) {
    // e.Data is []byte — your encrypted payload
    // NEVER call blocking Client methods here; spawn a goroutine
})
if err := sub.Subscribe(); err != nil { ... }

// Connect
client.Connect()

// Publish (from goroutine, not inside event handler)
result, err := sub.Publish(ctx, encryptedPayload)
// or client.Publish(ctx, "ch-classical", encryptedPayload)
```

`PublicationEvent` embeds `Publication` with `.Data []byte`, `.Offset uint64`,
`.Tags map[string]string`.

**Deadlock rule (critical):** Event handlers block the read loop. Any call to
`Publish`, `RPC`, `History`, `Presence` from inside an `OnPublication` callback
will deadlock. Always `go func() { ... }()` those calls.

---

### Ratchet Library: go-doubleratchet v0.0.2

**Import:** `github.com/KushnerykPavel/go-doubleratchet v0.0.2`
**Go requirement:** 1.25.0+
**Dependencies:** `filippo.io/edwards25519 v1.1.0`, `golang.org/x/crypto v0.50.0`
**Status:** Pre-1.0; minor versions may have API breaks. Pin strictly to v0.0.2.

Full exported API (HIGH confidence — verified via pkg.go.dev at @v0.0.2):

#### Classical Double Ratchet (`*Session`)

```go
// Key generation
privKey, pubKey, err := doubleratchet.GenerateKeyPair() // [32]byte each

// Alice (initiator)
alice, err := doubleratchet.InitInitiator(sharedSecret []byte, bobPub [32]byte, cfg *Config)

// Bob (responder)
bobKeyPair := doubleratchet.KeyPair{PrivateKey: privKey, PublicKey: pubKey}
bob, err := doubleratchet.InitResponder(sharedSecret []byte, bobKeyPair, cfg *Config)

// Encrypt / Decrypt
msg, err := alice.Encrypt(plaintext []byte, ad []byte)  // returns Message
plain, err := bob.Decrypt(msg, ad []byte)               // returns []byte

// Cleanup
alice.Close()
```

`Message` struct: `{Header Header, Ciphertext []byte}`
`Header` struct: `{RatchetPublicKey [32]byte, PN uint32, N uint32}`

#### Post-Quantum Triple Ratchet (`*TripleRatchetSession`)

```go
// Requires an scka.Provider implementation
alice, err := doubleratchet.InitInitiatorTripleRatchet(
    sharedSecret []byte,
    bobDRPK [32]byte,       // Bob's EC ratchet public key
    provider scka.Provider,
    cfg *Config,
)
bob, err := doubleratchet.InitResponderTripleRatchet(
    sharedSecret []byte,
    bobKeyPair KeyPair,
    provider scka.Provider,
    cfg *Config,
)

trMsg, err := alice.Encrypt(plaintext, ad)  // returns TripleRatchetMessage
plain, err := bob.Decrypt(trMsg, ad)
```

`TripleRatchetMessage` struct: `{Ciphertext []byte, Header TripleRatchetHeader}`
`TripleRatchetHeader` struct: `{SCKA *SCKAHeader, EC Header}`

#### `scka.Provider` interface (must be implemented for PQ sessions)

```go
type Provider interface {
    InitInitiator(sk []byte) error
    InitResponder(sk []byte) error
    Send() (msg []byte, sendingEpoch uint32, outputKey []byte, keyEpoch uint32, err error)
    Receive(msg []byte) (receivingEpoch uint32, outputKey []byte, keyEpoch uint32, err error)
    Snapshot() any
    Restore(snapshot any)
    Close() error
}
```

`sckatest.MockSCKA` exists for testing only — **not for blog demo** (does not use
real PQ crypto). The project must implement a real `scka.Provider` that wraps
`crypto/mlkem` calls. This is the key integration point between the library and
`crypto/mlkem`.

#### `Config` struct

```go
type Config struct {
    KDFInfo           []byte   // default "WhisperRatchet"
    HEKDFInfo         []byte
    EncryptInfo       []byte
    HybridInfo        []byte   // Triple Ratchet hybrid KDF
    MaxSkip           uint32   // default 1000
    LocalIdentityKey  [32]byte
    RemoteIdentityKey [32]byte
}
```

Passing `nil` for config uses all defaults — fine for the blog demo.

**State serialization:** v0.0.2 does not expose explicit Marshal/Unmarshal methods.
Sessions are in-memory only; the demo resets on restart (matches project scope).

---

### PQ KEM: crypto/mlkem

**Package:** `crypto/mlkem` (Go 1.23+ stdlib — no import required beyond Go version)
**Recommended parameter set:** ML-KEM-768 (NIST recommendation; 128-bit PQ security)

Full API (HIGH confidence — verified via pkg.go.dev):

```go
import "crypto/mlkem"

// Constants
// SharedKeySize         = 32
// SeedSize              = 64
// CiphertextSize768     = 1088
// EncapsulationKeySize768 = 1184

// Alice: generate key pair
dk, err := mlkem.GenerateKey768()
ekBytes := dk.EncapsulationKey().Bytes()  // send to Bob (1184 bytes)

// Bob: encapsulate
ek, err := mlkem.NewEncapsulationKey768(ekBytes)
sharedSecret, ciphertext := ek.Encapsulate()  // send ciphertext to Alice (1088 bytes)

// Alice: decapsulate
sharedSecret, err := dk.Decapsulate(ciphertext)  // same 32-byte secret as Bob's

// Seed-based reconstruction (for deterministic testing)
dk, err := mlkem.NewDecapsulationKey768(seed64bytes)
seedBytes := dk.Bytes()  // 64-byte "d || z" form
```

Wire overhead compared to X25519: public key 1184 bytes vs 32 bytes; ciphertext
1088 bytes vs 32 bytes. This difference is exactly what the Grafana dashboard
should highlight.

---

### Classical Key Agreement: X3DH

**No standalone X3DH library is needed.** Build it from primitives — this is the
right approach for a blog demo because it makes the protocol steps explicit.

**Primitives to use:**

| Step | Package | Notes |
|------|---------|-------|
| X25519 DH | `crypto/ecdh` (stdlib, Go 1.20+) | Preferred over `golang.org/x/crypto/curve25519`; higher-level API |
| Ed25519 identity keys | `crypto/ed25519` (stdlib) | For IK signing in full X3DH |
| HKDF | `golang.org/x/crypto/hkdf` | KDF over concatenated DH outputs |
| ChaCha20-Poly1305 | `golang.org/x/crypto/chacha20poly1305` | Symmetric encryption after X3DH |

**X3DH DH operations:**

```go
import "crypto/ecdh"

curve := ecdh.X25519()
privKey, err := curve.GenerateKey(rand.Reader)
pubKey := privKey.PublicKey()

// DH
shared, err := privKey.ECDH(theirPubKey)  // returns []byte
```

**PQXDH** adds a fifth DH using `crypto/mlkem`: the initiator encapsulates to
the responder's ML-KEM public key; the resulting 32-byte shared secret is
concatenated with the X3DH DH outputs before HKDF. The `go-doubleratchet`
library then receives this concatenated secret as `sharedSecret []byte`.

Packages to `go get`:
```bash
go get golang.org/x/crypto
```
(`crypto/ecdh` and `crypto/ed25519` are stdlib.)

---

### Metrics: Prometheus + Grafana

#### Prometheus Go Client

**Package:** `github.com/prometheus/client_golang` — latest stable **v1.23.2** (2025-09-05).
There is no separate `v2` module path. The `prometheus.V2` struct exists as an
experimental namespace within v1.x but is not needed here — stick to v1 API.

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Define metrics at package level with promauto (auto-registers to DefaultRegisterer)
var (
    msgWireBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "chat_message_wire_bytes",
        Help:    "Encrypted message wire size in bytes",
        Buckets: []float64{100, 200, 500, 1000, 2000, 5000},
    }, []string{"protocol"}) // label: "classical" or "pq"

    handshakeLatencyNs = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "chat_handshake_latency_ns",
        Help:    "Key agreement handshake latency in nanoseconds",
        Buckets: prometheus.ExponentialBuckets(1000, 10, 8),
    }, []string{"protocol"})

    encryptLatencyNs = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "chat_encrypt_latency_ns",
        Help:    "Per-message encrypt latency in nanoseconds",
        Buckets: prometheus.ExponentialBuckets(100, 5, 8),
    }, []string{"protocol"})
)

// Expose metrics endpoint
http.Handle("/metrics", promhttp.Handler())
```

Record observations:
```go
msgWireBytes.WithLabelValues("classical").Observe(float64(len(wirePayload)))
```

**Deprecated pattern to avoid:** `prometheus.NewGoCollector()` at the call site —
use `collectors.NewGoCollector()` from `github.com/prometheus/client_golang/prometheus/collectors` instead.

#### Grafana

**Version:** Grafana 11.x (current as of 2026). Use `grafana/grafana:11` in compose.

**Provisioning via docker compose** — mount config directories:

```yaml
# docker-compose.yml
grafana:
  image: grafana/grafana:11
  volumes:
    - ./grafana/provisioning:/etc/grafana/provisioning
    - ./grafana/dashboards:/var/lib/grafana/dashboards
  environment:
    - GF_SECURITY_ADMIN_PASSWORD=admin
```

Directory layout:
```
grafana/
  provisioning/
    datasources/
      prometheus.yml     # datasource config
    dashboards/
      dashboards.yml     # dashboard provider config
  dashboards/
    chat-comparison.json # exported dashboard JSON
```

`grafana/provisioning/datasources/prometheus.yml`:
```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

`grafana/provisioning/dashboards/dashboards.yml`:
```yaml
apiVersion: 1
providers:
  - name: default
    folder: ''
    type: file
    options:
      path: /var/lib/grafana/dashboards
```

Dashboard JSON: export from Grafana UI after building panels, then commit the
JSON file at `grafana/dashboards/chat-comparison.json`. The JSON references
datasource by `uid: "${DS_PROMETHEUS}"` using a template variable so it is
portable across provisioned environments.

#### Prometheus scrape config

```yaml
# prometheus/prometheus.yml
scrape_configs:
  - job_name: chat-clients
    static_configs:
      - targets: ['alice-classical:2112', 'bob-classical:2113', 'alice-pq:2114', 'bob-pq:2115']
```

Each client binary exposes `:211x/metrics`.

---

### Monorepo Structure: go.work vs Single Module

**Recommendation: single `go.mod` at the repo root.**

Rationale for this specific project:
- Four client binaries (`alice-classical`, `bob-classical`, `alice-pq`, `bob-pq`)
  plus shared crypto/metrics packages — all are part of one cohesive demo.
- No module publishes to a separate import path; nothing is imported externally.
- `go.work` is a local-development tool for working across separately-published
  modules. The community consensus (2025/2026) is not to commit `go.work` files.
- A single module with `cmd/` subdirectories is the standard Go layout here.

**Recommended layout:**

```
centrifugal_rachet/
  go.mod                           module github.com/KushnerykPavel/centrifugal-ratchet
  go.sum
  cmd/
    alice-classical/main.go
    bob-classical/main.go
    alice-pq/main.go
    bob-pq/main.go
  internal/
    classical/                     X3DH + DR wrappers
    pq/                            PQXDH + Triple Ratchet wrappers
    transport/                     centrifuge-go wrappers
    metrics/                       prometheus metric definitions
    scka/                          scka.Provider implementation over crypto/mlkem
  centrifugo/
    config.json                    Centrifugo v6 config
  grafana/
    provisioning/...
    dashboards/...
  prometheus/
    prometheus.yml
  docker-compose.yml
  Dockerfile.client                multi-stage build for all cmd/ binaries
```

Each `cmd/*/main.go` is a separate binary; `go build ./cmd/alice-classical` works
without any workspace file.

**If you later extract `internal/scka` as a standalone publishable library:**
Add `go.work` then, and add it to `.gitignore`. For this demo, don't bother.

---

## Confidence Levels

| Component | Confidence | Source |
|-----------|------------|--------|
| centrifuge-go v0.10.12 API | HIGH | pkg.go.dev verified |
| Centrifugo v6 config format | HIGH | official blog post |
| go-doubleratchet v0.0.2 API | HIGH | pkg.go.dev @v0.0.2 |
| scka.Provider interface | HIGH | pkg.go.dev @v0.0.2 |
| crypto/mlkem API | HIGH | pkg.go.dev official stdlib docs |
| crypto/ecdh for X3DH DH steps | HIGH | Go stdlib docs |
| prometheus/client_golang v1.23.2 | HIGH | pkg.go.dev |
| Grafana provisioning pattern | MEDIUM | official docs + community |
| Single module recommendation | HIGH | Go official tutorial + 2025/2026 articles |

---

## What NOT to Use

| Avoid | Why |
|-------|-----|
| `liboqs-go` / CGo KEM libraries | CGo breaks `docker build` hermeticity; `crypto/mlkem` stdlib is sufficient and simpler |
| `github.com/tiabc/doubleratchet` or `status-im/doubleratchet` | Wrong library — project requires KushnerykPavel fork exactly |
| `sckatest.MockSCKA` in production demo code | Does not use real PQ crypto; would make the PQ comparison meaningless |
| `golang.org/x/crypto/curve25519` directly | Superseded by `crypto/ecdh` (stdlib Go 1.20+); higher-level, safer API |
| `filippo.io/edwards25519` directly for DH | It is an Edwards curve library (for Ed25519 signatures), not X25519 DH; `crypto/ecdh.X25519()` is the right tool |
| `prometheus.V2` experimental API | Unnecessary complexity; v1 API covers all needs here |
| Committing `go.work` | Community consensus is to gitignore it; single module makes it irrelevant anyway |
| SockJS transport for Centrifugo | Removed in Centrifugo v6 |
| `centrifuge-go v0.8.x` | v0.8.x is for Centrifugo v2/v3; use v0.10.x for v6 |

---

## Sources

- [centrifuge-go pkg.go.dev](https://pkg.go.dev/github.com/centrifugal/centrifuge-go)
- [Centrifugo v6 release blog](https://centrifugal.dev/blog/2025/01/16/centrifugo-v6-released)
- [Centrifugo releases](https://github.com/centrifugal/centrifugo/releases)
- [go-doubleratchet pkg.go.dev @v0.0.2](https://pkg.go.dev/github.com/KushnerykPavel/go-doubleratchet@v0.0.2)
- [crypto/mlkem pkg.go.dev](https://pkg.go.dev/crypto/mlkem)
- [prometheus/client_golang pkg.go.dev](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus)
- [Grafana provisioning docs](https://grafana.com/docs/grafana/latest/administration/provisioning/)
- [Go workspaces tutorial](https://go.dev/doc/tutorial/workspaces)
- [Go workspaces for monorepos (2026)](https://oneuptime.com/blog/post/2026-02-01-go-workspaces-monorepos/view)
