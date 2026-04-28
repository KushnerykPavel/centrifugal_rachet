# Post-Quantum Ratchets in Go: Same API, 10x the Wire

*A hands-on walkthrough of building X3DH + Double Ratchet and PQXDH + Triple Ratchet side by side — and why the application code looks almost identical.*

---

## The Problem: Forward Secrecy Is Not Enough Anymore

If you've shipped anything that uses TLS 1.3 or Signal-style end-to-end encryption, you already understand forward secrecy: even if an attacker captures today's ciphertext and steals tomorrow's long-term key, they can't decrypt old sessions. Ratchet protocols — specifically X3DH key agreement combined with the Double Ratchet Algorithm — are the machinery behind Signal, WhatsApp, and Matrix that deliver this property.

The catch is harvest-now-decrypt-later. Nation-state actors (and well-funded others) are capturing encrypted traffic today, on the assumption that sufficiently powerful quantum computers will arrive in the next decade and let them decrypt the archive. If your messages have a shelf life longer than "before the first cryptographically-relevant quantum computer," classical X25519-based ratchets are not sufficient.

NIST finalized its first post-quantum standards in 2024. The one that matters for key encapsulation is ML-KEM (Module Lattice Key Encapsulation Mechanism, formerly CRYSTALS-Kyber). The Signal Foundation published a PQXDH specification that layers ML-KEM on top of the existing X3DH handshake. And starting with Go 1.23, `crypto/mlkem` is in the standard library — no CGo, no external KEM dependency.

I built a small Go demo that runs both protocols side by side over the same pub/sub hub, with live Prometheus metrics showing the wire-size difference. This article walks through the implementation, the architecture, and the key insight: **the application-layer API for PQXDH is nearly identical to X3DH**. The 10x wire overhead is real and measurable — but it does not leak into your application logic.

---

## Architecture in One Paragraph

The demo has seven Docker services: a **Centrifugo** WebSocket pub/sub server, four client binaries (alice-classical, bob-classical, alice-pq, bob-pq), **Prometheus**, and **Grafana**. Alice roles initiate the handshake; Bob roles respond. The two protocol pairs run on completely isolated channels — `ch-classical` and `ch-pq` — so there is no cross-protocol noise in the metrics. All four binaries expose a `/metrics` endpoint; Prometheus scrapes them and attaches `protocol` and `role` labels at scrape time rather than inside the binary code. Grafana visualizes wire size, handshake latency, encrypt/decrypt overhead.

```
centrifugo (:8000)
 ├── ch-classical: bob-classical (:9092) ↔ alice-classical (:9091)
 └── ch-pq:        bob-pq (:9094)        ↔ alice-pq (:9093)
                               ↓ /metrics
              prometheus (:9090) → grafana (:3000)
```

Each Alice–Bob pair does exactly one full session: Bob publishes a prekey bundle, Alice initiates the handshake and sends a few encrypted messages, Bob echoes them back. The session exits cleanly. Infrastructure services keep running so you can inspect the Grafana dashboard at `http://localhost:3000` after the exchange completes.

---

## X3DH: Classical Key Agreement

X3DH (Extended Triple Diffie-Hellman) is the key agreement protocol underneath Signal. The idea is elegant: Bob pre-publishes a bundle of public keys signed with his identity key. Alice fetches that bundle and performs three (or four) Diffie-Hellman operations to derive a shared secret — without Bob being online. Bob can later derive the same secret from Alice's initial message. The result is an authenticated, forward-secret session key with no interactive round-trip required for setup.

### Key Derivation: X3DH Detail

```
Alice                                                          Bob
  |                                                              |
  | Generate IK_A (X25519 identity key)                         |
  |                                                              | Generate IK_B, SPK_B (signed pre-key)
  |                                                              | Sig = Sign(IK_B, SPK_B)
  |                        <-- prekey_bundle -------------------|
  |     {IK_B_pub, SPK_B_pub, SPKID, Sig}                       |
  |                                                              |
  | EK_A = ephemeral X25519 key pair                            |
  | DH1 = DH(IK_A,  SPK_B_pub)  // identity auth               |
  | DH2 = DH(EK_A,  IK_B_pub)   // forward secrecy             |
  | DH3 = DH(EK_A,  SPK_B_pub)  // binding to pre-key          |
  | SharedSecret = HKDF(DH1 || DH2 || DH3)                      |
  |                                                              |
  |-- initial_msg {IK_A_pub, EK_A_pub} -----------------------> |
  |                              DH1 = DH(SPK_B,  IK_A_pub)    |
  |                              DH2 = DH(IK_B,   EK_A_pub)    |
  |                              DH3 = DH(SPK_B,  EK_A_pub)    |
  |                              SharedSecret = HKDF(DH1||DH2||DH3)
  |                              doubleratchet.InitBob(SharedSecret, SPK_B_KeyPair)
  |                                                              |
  | doubleratchet.InitAlice(SharedSecret, SPK_B_pub)            |
  |                                                              |
  |              === Double Ratchet session established ===      |
```

### Go Code: X3DH Handshake

```go
import "github.com/KushnerykPavel/centrifugal-ratchet/internal/classical"

// Bob: generate and publish prekey bundle
bundle, priv, err := classical.NewResponderBundle()
// bundle (*classical.PrekeyBundle) — published to Centrifugo ch-classical as JSON
// priv  (*classical.ResponderKeys) — kept secret by Bob

// Alice: initiate X3DH
aliceSess, initMsg, err := classical.InitiatorHandshake(bundle)
// aliceSess.RootKey is set; initMsg is sent to Bob

// Bob: complete handshake
bobSess, err := classical.ResponderHandshake(priv, initMsg)
// bobSess.RootKey == aliceSess.RootKey
```

Three calls. `NewResponderBundle` / `InitiatorHandshake` / `ResponderHandshake`. That's the entire key agreement. The `doubleratchet.InitAlice` / `InitBob` calls happen inside those functions — callers never touch the DR internals directly.

After the handshake, both sides have a `*classical.Session` with a live Double Ratchet session and a `RootKey [32]byte` they can compare in tests (`alice.RootKey == bob.RootKey`).

---

## PQXDH: Post-Quantum Key Agreement

PQXDH is the Signal Foundation's extension to X3DH. It layers ML-KEM-768 encapsulation on top of the X25519 DH operations. The intuition: even if a future quantum computer breaks X25519, the ML-KEM shared secret is computationally infeasible to break with classical *or* quantum algorithms (under lattice hardness assumptions). The combined root key is secure as long as *either* assumption holds.

The concrete additions over X3DH:
- Bob's prekey bundle includes a **1184-byte ML-KEM-768 encapsulation key** (`KEM_SPK_B`).
- Alice performs the same X25519 DH operations (DH1..DH4) *and* runs `ML-KEM-768.Encapsulate(KEM_SPK_B)` to produce a KEM ciphertext and a KEM shared secret.
- The root key is derived via `HKDF(DH1 || DH2 || DH3 || DH4 || SS_kem)` — both X25519 and ML-KEM contribute.
- Alice's initial message carries the **1088-byte KEM ciphertext** alongside the X25519 public keys.
- After the handshake, a Triple Ratchet session runs — a KEM ratchet epoch updates ML-KEM keys periodically, adding a `SCKAHeader` to every encrypted message.

### Key Derivation: PQXDH Detail

```
Alice                                                          Bob
  |                                                              |
  |                  <-- prekey_bundle --------------------------|
  | {IK_B, SPK_B, OPK_B, KEM_SPK_B (ML-KEM-768 encap key, 1184 bytes)}
  |                                                              |
  | Generate IK_A, EK_A (X25519)                                |
  | DH1 = DH(IK_A,  SPK_B)   (identity auth)                   |
  | DH2 = DH(EK_A,  IK_B)    (forward secrecy)                 |
  | DH3 = DH(EK_A,  SPK_B)   (pre-key binding)                 |
  | DH4 = DH(EK_A,  OPK_B)   (one-time pre-key)                |
  | (ct, SS_kem) = ML-KEM-768.Encapsulate(KEM_SPK_B.EncapKey)  |
  | RootKey = HKDF(DH1 || DH2 || DH3 || DH4 || SS_kem)         |
  |   (Signal PQXDH spec KDF input order: X25519 first, KEM last)
  |                                                              |
  |-- initial_msg {IK_A, EK_A, ct (1088-byte KEM ciphertext)} ->|
  |                       DH1..DH4 = matching X25519 operations |
  |                       SS_kem = ML-KEM-768.Decapsulate(ct)   |
  |                       RootKey = HKDF(DH1||DH2||DH3||DH4||SS_kem)
  |                       doubleratchet.InitBobTripleRatchet(    |
  |                           RootKey, SPK_B_KeyPair, MLKEMProvider)
  |                                                              |
  | doubleratchet.InitAliceTripleRatchet(                       |
  |     RootKey, SPK_B_pub, MLKEMProvider)                      |
  |                                                              |
  |        === Triple Ratchet session established ===            |
  | (ML-KEM epoch ratchet: 1184-byte encap key +               |
  |  1088-byte ciphertext carried in SCKAHeader per message)   |
```

### Go Code: PQXDH Handshake

```go
import "github.com/KushnerykPavel/centrifugal-ratchet/internal/pq"

// Bob: generate and publish PQXDH prekey bundle (includes ML-KEM-768 encap key)
bundle, priv, err := pq.NewResponderBundle()
// bundle (*pqxdh.PrekeyBundle) — 1184-byte ML-KEM-768 encap key included
// priv  (*pq.ResponderKeys)    — kept secret by Bob

// Alice: initiate PQXDH — caller API is identical to X3DH
aliceSess, initMsg, err := pq.InitiatorHandshake(bundle)
// Under the hood: ML-KEM-768.Encapsulate runs inside InitiatorHandshake
// initMsg carries the 1088-byte KEM ciphertext

// Bob: complete handshake
bobSess, err := pq.ResponderHandshake(priv, initMsg)
// bobSess.RootKey == aliceSess.RootKey (derived from X25519 + ML-KEM shared secrets via HKDF)
```

Notice anything? The three-call structure is identical:

| Classical | PQ |
|-----------|-----|
| `classical.NewResponderBundle()` | `pq.NewResponderBundle()` |
| `classical.InitiatorHandshake(bundle)` | `pq.InitiatorHandshake(bundle)` |
| `classical.ResponderHandshake(priv, initMsg)` | `pq.ResponderHandshake(priv, initMsg)` |

**This is the engineering insight.** The caller sees the same API surface. The 10x wire overhead lives entirely inside the library.

> **Under the hood:** `InitiatorHandshake` and `ResponderHandshake` each instantiate an `MLKEMProvider` (implementing the `scka.Provider` interface from go-doubleratchet). The caller never touches it — that's the point.

---

## Encrypt/Decrypt: The API Parity Punchline

Once you have a session from either protocol, encryption and decryption are syntactically identical:

```go
// Classical: returns *classical.Message
msg, err := aliceSess.Encrypt([]byte("hello"))
plain, err := bobSess.Decrypt(msg)

// PQ — identical call surface, different wire cost
// PQ: returns *pq.TripleRatchetMessage
msg, err := aliceSess.Encrypt([]byte("hello"))
plain, err := bobSess.Decrypt(msg)
```

The return types differ — `*classical.Message` vs `*pq.TripleRatchetMessage` — but both carry their protocol-specific wire encoding internally. The call site is identical: `sess.Encrypt(plaintext)` → `sess.Decrypt(msg)`. Application logic that sends and receives messages does not change between protocols.

One minor asymmetry worth noting for cleanup code: `classical.Session.Close()` returns nothing (`func (s *Session) Close()`), while `pq.Session.Close()` returns `error` (`func (s *Session) Close() error`). This reflects the underlying library's TripleRatchetSession having a fallible cleanup path for KEM key material.

---

## The Numbers: What Grafana Shows

After `docker compose up`, visit `http://localhost:3000`. No login required — anonymous Viewer access is enabled. The dashboard shows wire bytes per message, handshake latency, and per-message encrypt/decrypt overhead for both protocol pairs side by side.

### Wire-Size Comparison

| Metric | Classical (X3DH + DR) | PQ (PQXDH + Triple Ratchet) |
|--------|----------------------|------------------------------|
| Handshake initial message (bytes) | ~120 | ~1208 |
| Per-message ratchet step (bytes) | ~40 | ~1128 |

*Numbers measured from a live `docker compose up` run via Grafana `ratchet_message_wire_bytes` histogram.*

What drives the PQ size:

- **1184 bytes** — ML-KEM-768 encapsulation key in the prekey bundle (Bob's public KEM key)
- **1088 bytes** — ML-KEM-768 ciphertext in every initial message (Alice's encapsulated shared secret)
- **SCKAHeader** — carried in each Triple Ratchet message for the KEM epoch ratchet; contains the next ML-KEM public key for Bob and the encapsulated KEM shared secret for the current epoch

The X25519 DH contributions are 32 bytes each. The ML-KEM-768 contributions dwarf them by an order of magnitude. That's why the wire size jumps from ~40 bytes to ~1128 bytes per ratchet step.

---

## Metrics Registration: Keeping Binaries Label-Agnostic

Here is the metrics package that all four binaries import:

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var Reg *prometheus.Registry
var MessageWireBytes prometheus.Histogram

func init() {
    Reg = prometheus.NewRegistry()
    MessageWireBytes = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "ratchet_message_wire_bytes",
        Help:    "Wire size of serialized ratchet messages in bytes.",
        Buckets: prometheus.DefBuckets,
    })
    Reg.MustRegister(MessageWireBytes)
    // HandshakeDurationSeconds, EncryptDurationSeconds, DecryptDurationSeconds follow same pattern
}
```

A key design choice: `protocol` (classical/pq) and `role` (alice/bob) labels are **not** emitted by the binaries. They are attached at Prometheus scrape time via `static_configs.labels` in `prometheus/prometheus.yml`:

```yaml
scrape_configs:
  - job_name: alice-classical
    static_configs:
      - targets: ["alice-classical:9091"]
        labels:
          protocol: classical
          role: alice
```

This keeps the binaries label-agnostic. The `internal/metrics` package is identical across all four binaries — no conditional compilation, no per-binary initialization, no label parameters. The Grafana dashboard queries `ratchet_message_wire_bytes{protocol="pq"}` to filter. Labels come from topology, not from code.

---

## Takeaway: Crypto Agility Is Real

The demo shows that crypto agility is achievable in practice, not just in principle.

The application-level code — the part that calls `Encrypt` and `Decrypt`, that decides what to send, that handles errors — is **identical** between classical and PQ sessions. Not "similar." Identical. You could write a generic chat loop parameterized on a session interface and it would work for both protocols without modification.

The 10x wire overhead is real and measurable. It will matter in constrained environments: IoT devices on cellular links, high-frequency messaging systems where bandwidth costs are significant, latency-sensitive applications on slow networks. You should measure your own use case against a live Grafana dashboard, not against these demo numbers.

But the overhead does **not leak into your application logic**. The `SCKAHeader`, the ML-KEM ciphertext, the epoch ratchet updates — all of that is inside `pq.Session`. The caller sees `Encrypt(plaintext []byte) (*TripleRatchetMessage, error)` and `Decrypt(msg *TripleRatchetMessage) ([]byte, error)`. Period.

Go's stdlib `crypto/mlkem` makes this hermetic: no CGo, no external KEM library, no liboqs, no build complexity beyond `go mod tidy`. The ML-KEM implementation was added in Go 1.23 and follows the same stdlib quality bar as `crypto/ecdh`. When your team's security policy requires post-quantum key agreement, the migration path is: swap the session type, update the import, update the wire-type references. The business logic does not change.

When NIST finalizes PQ standards and clients demand it, swapping the ratchet layer is a small change.

---

## Links

- **Quick-start:** See [README.md](README.md) for `docker compose up` instructions and the comparison table.
- **Repository:** `github.com/KushnerykPavel/centrifugal-ratchet`
- **Classical session source:** [`internal/classical/classical.go`](internal/classical/classical.go)
- **PQ session source:** [`internal/pq/pq.go`](internal/pq/pq.go) and [`internal/pq/provider.go`](internal/pq/provider.go)
- **Metrics source:** [`internal/metrics/metrics.go`](internal/metrics/metrics.go)
- **go-doubleratchet library:** `github.com/KushnerykPavel/go-doubleratchet v0.0.2`
