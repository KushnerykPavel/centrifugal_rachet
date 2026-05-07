# Centrifugal Ratchet

A side-by-side comparison of classical (X3DH + Double Ratchet) and post-quantum (PQXDH + Triple Ratchet) encrypted chat over Centrifugo pub/sub, with live Prometheus metrics and a Grafana dashboard.

---

## Prerequisites

- **Docker Engine 20.10+** — `docker compose` is a first-class plugin since 20.10 (no hyphen variant needed)
- **docker compose v2+** — bundled with Docker Desktop; or install the Compose plugin separately

No local Go installation required. All four Go binaries are built inside Docker containers during the first `docker compose up`.

---

## Quick Start

```bash
git clone https://github.com/KushnerykPavel/centrifugal-ratchet.git
cd centrifugal-ratchet
docker compose up
```

First run builds four Go binaries inside Docker — allow 1–2 minutes for the initial image build.

---

## What You'll See

Startup order is enforced via `depends_on` health checks:

1. **centrifugo** starts first and becomes healthy (pub/sub hub on `:8000`)
2. **bob-classical** and **bob-pq** start concurrently — each publishes a prekey bundle to its channel
3. **alice-classical** and **alice-pq** start after their respective Bob is healthy — each completes the X3DH / PQXDH handshake
4. Both Alice–Bob pairs exchange encrypted ratchet messages (one message + one echo)
5. Client binaries exit after the echo exchange (`restart: on-failure` — this is expected behaviour)
6. **prometheus** and **grafana** keep running

Open **http://localhost:3000** — no login required (anonymous Viewer). The Grafana dashboard shows:

- Wire size histograms: classical ratchet messages (~40 bytes) vs PQ ratchet messages (~1128 bytes)
- Handshake latency: X3DH vs PQXDH round-trip time
- Per-message encrypt/decrypt overhead for both protocol pairs

---

## Comparison Table

| Metric | Classical (X3DH + DR) | Post-Quantum (PQXDH + TR) |
|--------|----------------------|---------------------------|
| Handshake initial message | ~120 bytes† | ~1208 bytes† |
| Ratchet step message | ~40 bytes† | ~1128 bytes† |
| ML-KEM-768 encap key | — | 1184 bytes |
| ML-KEM-768 ciphertext | — | 1088 bytes |

† planning-phase estimates (REQUIREMENTS.md); not yet confirmed from a live run — TODO: run `docker compose up` and verify via Grafana `ratchet_message_wire_bytes` histogram at http://localhost:3000

The ML-KEM-768 key sizes (1184 / 1088 bytes) are confirmed by unit-test assertions in `internal/pq/pq_test.go` (TestMLKEMProviderKEMProtocol and TestMLKEMProviderSnapshot). The full-message wire sizes (~120/~1208/~40/~1128) require a live stack run to confirm.

---

## Architecture Overview

| Service | Host Port | Purpose | Restart Policy |
|---------|-----------|---------|----------------|
| centrifugo | 8000 | WebSocket pub/sub hub; channels `ch-classical` and `ch-pq` | unless-stopped |
| bob-classical | 9092 | Publishes X3DH prekey bundle; echoes Double Ratchet messages | on-failure |
| alice-classical | 9091 | Initiates X3DH handshake; sends Double Ratchet messages | on-failure |
| bob-pq | 9094 | Publishes PQXDH prekey bundle; echoes Triple Ratchet messages | on-failure |
| alice-pq | 9093 | Initiates PQXDH handshake; sends Triple Ratchet messages | on-failure |
| prometheus | 9090 | Scrapes `/metrics` from all four client binaries | unless-stopped |
| grafana | 3000 | Dashboard UI; anonymous Viewer access; shows wire bytes + latency | unless-stopped |

Startup order: centrifugo → {bob-classical, bob-pq} → {alice-classical, alice-pq}; prometheus and grafana wait for centrifugo healthy.

---

## Message Flow Diagrams

### X3DH + Double Ratchet (Classical)

```
Alice                       Centrifugo (ch-classical)         Bob
  |                                   |                         |
  |                                   |<-- prekey_bundle -------|  (Bob publishes bundle on startup)
  |<-------- prekey_bundle -----------|                         |
  |                                   |                         |
  |-- initial_msg (X3DH init) ------->|                         |
  |                                   |-- initial_msg --------->|  (Bob derives RootKey)
  |                                   |                         |
  |-- ratchet_msg (DR encrypted) ---->|                         |
  |                                   |-- ratchet_msg --------->|
  |                                   |                         |
  |<-- ratchet_msg (echo, DR enc.) ---|<-- ratchet_msg ---------|
```

### PQXDH + Triple Ratchet (Post-Quantum)

```
Alice                       Centrifugo (ch-pq)              Bob
  |                                   |                         |
  |                                   |<-- prekey_bundle -------|  (includes ML-KEM-768 encap key)
  |<-------- prekey_bundle -----------|                         |
  |                                   |                         |
  |-- initial_msg (PQXDH init) ------>|                         |
  |   (contains ML-KEM ciphertext)    |-- initial_msg --------->|  (Bob derives RootKey + PQRKey)
  |                                   |                         |
  |-- ratchet_msg (TR encrypted) ---->|                         |
  |   (~1128 bytes vs ~40 bytes)      |-- ratchet_msg --------->|
  |                                   |                         |
  |<-- ratchet_msg (echo, TR enc.) ---|<-- ratchet_msg ---------|
```

Both flows are structurally identical — only the key material and wire cost differ. The `initial_msg` in the PQ flow carries a 1088-byte ML-KEM-768 ciphertext; every subsequent Triple Ratchet message carries a fresh 1184-byte encapsulation key for the next KEM epoch.

---

## Prometheus Metrics

All four client binaries expose `/metrics` on their respective ports. Prometheus scrapes them and attaches `protocol` and `role` labels at scrape time — the binaries themselves emit no label dimensions.

| Metric Name | Type | Description |
|-------------|------|-------------|
| `ratchet_message_wire_bytes` | Histogram | Serialized message size per send/receive (bytes) |
| `ratchet_handshake_duration_seconds` | Histogram | Wall time of full key exchange + session init |
| `ratchet_encrypt_duration_seconds` | Histogram | Wall time of `session.Encrypt` call |
| `ratchet_decrypt_duration_seconds` | Histogram | Wall time of `session.Decrypt` call |

Labels attached at scrape: `protocol=classical|pq`, `role=alice|bob`. All histograms use `prometheus.DefBuckets`.

Direct Prometheus query interface: http://localhost:9090

---

## Code Pointers

No inline snippets — links only; the code is the authoritative source.

- Key exchange (classical): [internal/classical/classical.go](internal/classical/classical.go)
- Key exchange (PQ): [internal/pq/pq.go](internal/pq/pq.go)
- ML-KEM provider: [internal/pq/provider.go](internal/pq/provider.go)
- Prometheus metrics: [internal/metrics/metrics.go](internal/metrics/metrics.go)
- Message envelope: [internal/protocol/envelope.go](internal/protocol/envelope.go)
- Scrape config (protocol/role labels): [prometheus/prometheus.yml](prometheus/prometheus.yml)
- Service topology: [docker-compose.yml](docker-compose.yml)


