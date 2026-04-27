# Roadmap: Centrifugal Ratchet

## Overview

Six phases deliver a runnable side-by-side comparison of classical and post-quantum ratchet protocols. The monorepo foundation and classical session land first so all downstream work has a proven crypto core. The PQ session follows in isolation — before Centrifugo adds another failure mode. Centrifugo integration wires both protocol pairs over real pub/sub channels. Prometheus instrumentation makes the size and latency differences measurable. Docker Compose and Grafana make the numbers visible in a single `docker compose up`. The README and blog content close the loop with a reader-facing narrative anchored to verified numbers.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation + Classical Session** - Monorepo layout, shared helpers, and a working X3DH + Double Ratchet session with unit tests
- [ ] **Phase 2: PQ Session** - PQXDH key agreement and Triple Ratchet session with unit tests confirming PQ wire overhead
- [ ] **Phase 3: Centrifugo Integration** - Both protocol pairs running over live Centrifugo pub/sub channels with in-band key exchange
- [ ] **Phase 4: Prometheus Metrics** - All four binaries instrumented with wire-size, handshake, and encrypt/decrypt histograms
- [ ] **Phase 5: Docker Compose + Grafana** - Full seven-service compose stack with healthcheck ordering and Grafana dashboard JSON
- [ ] **Phase 6: README + Blog Content** - Quick-start, comparison table with exact byte numbers, ASCII diagrams, and code pointers

## Phase Details

### Phase 1: Foundation + Classical Session
**Goal**: A working classical encrypted chat session exists, verified by unit tests, with shared helpers ready for all downstream code
**Depends on**: Nothing (first phase)
**Requirements**: FOUND-01, FOUND-02, FOUND-03, FOUND-04, CLASS-01, CLASS-02, CLASS-03
**Success Criteria** (what must be TRUE):
  1. `go build ./...` succeeds from the repo root with a single `go.mod` and four binaries under `cmd/`
  2. `internal/keys.toKey32()` rejects incorrect-length input and converts valid input without padding or truncation
  3. `internal/metrics` exposes a `/metrics` endpoint serving a Prometheus registry with pre-registered histograms
  4. `internal/transport` connects, subscribes, publishes, and disconnects via `centrifuge-go` without error
  5. Unit test passes: `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` and `alice.RootKey == bob.RootKey` after X3DH handshake
**Plans**: 4 plans
Plans:
- [x] 01-01-PLAN.md — Monorepo scaffold: go.mod, go.sum, four cmd/ stub binaries
- [ ] 01-02-PLAN.md — internal/keys (ToKey32) and internal/metrics (Prometheus registry + histograms)
- [ ] 01-03-PLAN.md — internal/transport (centrifuge-go wrapper + integration test stub)
- [ ] 01-04-PLAN.md — internal/classical (X3DH + DR session wrapper + CLASS-03 unit test)

### Phase 2: PQ Session
**Goal**: A working post-quantum encrypted chat session exists using PQXDH + Triple Ratchet, verified by unit tests confirming PQ wire overhead is captured
**Depends on**: Phase 1
**Requirements**: PQ-01, PQ-02, PQ-03, PQ-04
**Success Criteria** (what must be TRUE):
  1. PQXDH handshake produces a shared root key combining ML-KEM-768 and X25519 outputs via HKDF in Signal-spec KDF input order
  2. `MLKEMProvider.Snapshot()` performs a deep copy — mutating the original does not affect the snapshot
  3. Unit test passes: `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` over a `TripleRatchetSession`
  4. Unit test confirms `len(serialized TripleRatchetMessage) > len(serialized *Message)` for identical plaintext, proving PQ overhead is captured
**Plans**: 4 plans
Plans:
- [ ] 02-01-PLAN.md — MLKEMProvider (scka.Provider), Snapshot/Restore, Close
- [ ] 02-02-PLAN.md — PQXDH handshake facade (NewResponderBundle, InitiatorHandshake, ResponderHandshake)
- [ ] 02-03-PLAN.md — Session Encrypt/Decrypt, PQ-01/PQ-03/PQ-04 acceptance tests
- [x] 02-04-PLAN.md — Gap closure: correct two-round ML-KEM-768 protocol (Decapsulate), remove drPriv/drPub, add KEM equality test

### Phase 3: Centrifugo Integration
**Goal**: Both classical and PQ Alice-Bob pairs exchange encrypted messages over live Centrifugo channels using in-band key exchange, with no goroutine deadlocks
**Depends on**: Phase 2
**Requirements**: CENT-01, CENT-02, CENT-03, CENT-04, CENT-05
**Success Criteria** (what must be TRUE):
  1. Centrifugo starts with `client.insecure: true` and `health: true`; `GET /health` returns 200
  2. Bob roles publish a typed `prekey_bundle` JSON envelope as their first message; Alice roles receive it within 30 s and complete the handshake
  3. Multiple ratchet messages flow on `ch-classical` and `ch-pq` independently — neither channel carries messages from the other protocol
  4. All `OnPublication` callbacks dispatch work to `go func()` — no blocking call inside the handler loop
**Plans**: TBD

### Phase 4: Prometheus Metrics
**Goal**: All four binaries emit wire-size, handshake, and encrypt/decrypt metrics; Prometheus scrape config attaches protocol and role labels without the binaries emitting them
**Depends on**: Phase 3
**Requirements**: OBS-01, OBS-02, OBS-03, OBS-04
**Success Criteria** (what must be TRUE):
  1. `ratchet_message_wire_bytes` histogram appears in each binary's `/metrics` output and reflects the full serialized struct size, not just ciphertext length
  2. `ratchet_handshake_duration_seconds`, `ratchet_encrypt_duration_seconds`, and `ratchet_decrypt_duration_seconds` histograms are present on all four `/metrics` endpoints
  3. `prometheus.yml` attaches `protocol=classical|pq` and `role=alice|bob` via `static_configs.labels`; the Go binaries emit no protocol or role label
  4. Total active series count stays flat as messages accumulate (no high-cardinality per-message labels)
**Plans**: TBD

### Phase 5: Docker Compose + Grafana
**Goal**: `docker compose up` starts all seven services in the correct order and the Grafana dashboard panels populate with live metrics within seconds of startup
**Depends on**: Phase 4
**Requirements**: OBS-05, DEPL-01, DEPL-02, DEPL-03
**Success Criteria** (what must be TRUE):
  1. `docker compose up` (from a clean clone, no pre-built images) starts centrifugo, four client binaries, prometheus, and grafana without manual steps
  2. Service startup order is enforced via `depends_on` with `condition: service_healthy`; Alice roles never start before their paired Bob is healthy
  3. Grafana dashboard shows two Stat panels (handshake initial message size classical vs PQ), one wire-size Time Series (both protocols overlaid), and two latency panels (encrypt and decrypt)
  4. Dashboard JSON is committed to the repo and provisioned automatically — no manual Grafana UI import required
**Plans**: TBD

### Phase 6: README + Blog Content
**Goal**: A reader can clone the repo, follow the README, and immediately understand both what to run and why the numbers matter
**Depends on**: Phase 5
**Requirements**: DEPL-04
**Success Criteria** (what must be TRUE):
  1. README quick-start section (`git clone` + `docker compose up`) is sufficient to get a running system on a clean machine
  2. Comparison table in README contains exact byte numbers (120 vs 1208 bytes handshake, 40 vs 1128 bytes ratchet step) sourced from a running instance
  3. ASCII sequence diagrams accurately represent X3DH and PQXDH key exchange steps
  4. Code sample references match `go-doubleratchet v0.0.2` API exactly — no invented method names or signatures
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation + Classical Session | 4/4 | Complete | 2026-04-27 |
| 2. PQ Session | 4/4 | Complete | 2026-04-27 |
| 3. Centrifugo Integration | 0/? | Not started | - |
| 4. Prometheus Metrics | 0/? | Not started | - |
| 5. Docker Compose + Grafana | 0/? | Not started | - |
| 6. README + Blog Content | 0/? | Not started | - |
