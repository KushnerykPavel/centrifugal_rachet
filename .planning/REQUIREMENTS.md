# Requirements: Centrifugal Ratchet

**Defined:** 2026-04-27
**Core Value:** A reader clones the repo, runs `docker compose up`, and immediately sees the real wire-size and latency difference between classical and post-quantum ratchet protocols in a working chat.

## v1 Requirements

### Foundation

- [x] **FOUND-01**: Repository uses a single `go.mod` at the root with `cmd/` for four binaries and `internal/` for shared packages — no `go.work` committed
- [ ] **FOUND-02**: `internal/keys` provides a `toKey32([]byte) ([32]byte, error)` helper used everywhere key bytes cross the `[]byte`/`[32]byte` boundary
- [ ] **FOUND-03**: `internal/metrics` exposes a Prometheus registry with pre-registered histograms and a `/metrics` HTTP handler reusable by all four binaries
- [ ] **FOUND-04**: `internal/transport` wraps `centrifuge-go` client lifecycle (connect, subscribe, publish, disconnect) used by all four binaries

### Classical Protocol

- [ ] **CLASS-01**: `internal/classical` implements X3DH key agreement from `crypto/ecdh` (X25519) and `golang.org/x/crypto/hkdf` — no standalone X3DH library
- [ ] **CLASS-02**: `internal/classical` initialises a `go-doubleratchet v0.0.2` `Session` from the X3DH `RootKey` output and exposes `Encrypt(plaintext []byte) (*Message, error)` and `Decrypt(msg *Message) ([]byte, error)`
- [ ] **CLASS-03**: Unit test confirms `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` and that `alice.RootKey == bob.RootKey` after handshake

### PQ Protocol

- [x] **PQ-01
**: `internal/pq` implements PQXDH key agreement combining ML-KEM-768 (`crypto/mlkem`) encapsulation with an X25519 DH, joined via `hkdf.New` over the concatenation `SS_mlkem || SS_x25519` — matching the Signal PQXDH spec KDF input order
- [x] **PQ-02
**: `internal/pq` provides a `MLKEMProvider` struct implementing `scka.Provider` from `go-doubleratchet v0.0.2`, wrapping `crypto/mlkem`; `Snapshot()` performs a deep copy of all key material
- [x] **PQ-03
**: `internal/pq` initialises a `go-doubleratchet v0.0.2` `TripleRatchetSession` from the PQXDH `RootKey` + `PQRKey` outputs and exposes `Encrypt(plaintext []byte) (*TripleRatchetMessage, error)` and `Decrypt(msg *TripleRatchetMessage) ([]byte, error)`
- [x] **PQ-04
**: Unit test confirms `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` and that `len(serialized TripleRatchetMessage) > len(serialized *Message)` for the same plaintext

### Centrifugo Integration

- [x] **CENT-01
**: Centrifugo config enables `client.insecure: true` and `health: true`; channels `ch-classical` and `ch-pq` require no token
- [x] **CENT-02
**: All four binaries use a message envelope struct with a `Type` field (`prekey_bundle | initial_msg | ratchet_msg`) on JSON publications to Centrifugo channels
- [x] **CENT-03
**: Bob roles publish their prekey bundle as the first unencrypted JSON message on startup; Alice roles retry subscription up to 30 s waiting for the bundle before performing the handshake
- [x] **CENT-04
**: All `OnPublication` callbacks dispatch to `go func()` — no blocking calls inside the handler
- [x] **CENT-05
**: Classical pair uses `ch-classical` exclusively; PQ pair uses `ch-pq` exclusively — no shared channel state

### Observability

- [x] **OBS-01
**: All four binaries observe `ratchet_message_wire_bytes` histogram (bytes of serialized message per send/receive) — measuring the full struct, not just ciphertext
- [x] **OBS-02
**: All four binaries observe `ratchet_handshake_duration_seconds` histogram (wall time of full key exchange + session init)
- [x] **OBS-03
**: All four binaries observe `ratchet_encrypt_duration_seconds` and `ratchet_decrypt_duration_seconds` histograms
- [ ] **OBS-04**: `prometheus.yml` attaches `protocol=classical|pq` and `role=alice|bob` labels via `static_configs.labels` at scrape level — binaries emit no protocol/role labels themselves
- [ ] **OBS-05**: Grafana dashboard JSON provisioned at startup includes: two Stat panels showing handshake initial message size (classical vs PQ), one Time Series panel of wire size over time (both protocols overlaid), two latency panels (encrypt and decrypt)

### Deployment

- [ ] **DEPL-01**: `docker-compose.yml` defines all seven services: `centrifugo`, `alice-classical`, `bob-classical`, `alice-pq`, `bob-pq`, `prometheus`, `grafana`
- [ ] **DEPL-02**: Startup order enforced via `depends_on` with `condition: service_healthy`: centrifugo first, then Bob roles, then Alice roles; Prometheus and Grafana wait for centrifugo healthy
- [ ] **DEPL-03**: Each client binary has a multi-stage Dockerfile (`golang:1.23-alpine` builder, `alpine` final); `/metrics` endpoint doubles as the Docker healthcheck URL
- [ ] **DEPL-04**: `README.md` includes quick-start (`git clone` + `docker compose up`), comparison table with exact byte numbers from research (120 vs 1208 bytes handshake, 40 vs 1128 bytes ratchet step), ASCII sequence diagrams for X3DH and PQXDH, and pointers to key code sections

## v2 Requirements

### Extended Demo Features

- **V2-01**: Out-of-order message delivery demonstration (hold one message, deliver after next, verify ratchet handles skipped key)
- **V2-02**: SPQR epoch boundary visualization (annotate Grafana panel when Triple Ratchet completes a KEM epoch)
- **V2-03**: CLI flag to choose ML-KEM-1024 vs ML-KEM-768 and observe key/ciphertext size change live
- **V2-04**: Centrifugo presence API showing active client count per channel

## Out of Scope

| Feature | Reason |
|---------|--------|
| Web chat UI (browser) | Grafana covers all visualization; adds frontend complexity without insight |
| Persistent message storage | Demo resets on restart — simplicity beats durability for a blog demo |
| User authentication / JWT tokens | Keys generated fresh each run; server-side identity orthogonal to protocol comparison |
| Cross-protocol bridging (classical ↔ PQ) | Would require a protocol adapter; obscures the comparison, not illustrates it |
| liboqs-go / CGo KEM | crypto/mlkem stdlib keeps build hermetic; CGo adds complexity with no insight gain |
| sckatest.MockSCKA in production paths | MockSCKA uses fake randomness — the PQ wire-size punchline evaporates without a real KEM |
| go.work in committed files | Per community consensus 2025; cross-module workspace not needed for a single-module repo |
| Prometheus push gateway | Pull model is simpler and correct for this use case |
| Grafana alerting | Blog demo — panels are sufficient |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUND-01 | Phase 1 | Complete |
| FOUND-02 | Phase 1 | Pending |
| FOUND-03 | Phase 1 | Pending |
| FOUND-04 | Phase 1 | Pending |
| CLASS-01 | Phase 1 | Pending |
| CLASS-02 | Phase 1 | Pending |
| CLASS-03 | Phase 1 | Pending |
| PQ-01 | Phase 2 | Pending |
| PQ-02 | Phase 2 | Pending |
| PQ-03 | Phase 2 | Pending |
| PQ-04 | Phase 2 | Pending |
| CENT-01 | Phase 3 | Pending |
| CENT-02 | Phase 3 | Pending |
| CENT-03 | Phase 3 | Pending |
| CENT-04 | Phase 3 | Pending |
| CENT-05 | Phase 3 | Pending |
| OBS-01 | Phase 4 | Pending |
| OBS-02 | Phase 4 | Pending |
| OBS-03 | Phase 4 | Pending |
| OBS-04 | Phase 4 | Pending |
| OBS-05 | Phase 5 | Pending |
| DEPL-01 | Phase 5 | Pending |
| DEPL-02 | Phase 5 | Pending |
| DEPL-03 | Phase 5 | Pending |
| DEPL-04 | Phase 6 | Pending |

**Coverage:**
- v1 requirements: 25 total
- Mapped to phases: 25
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-27*
*Last updated: 2026-04-27 after roadmap creation (6 phases mapped)*
