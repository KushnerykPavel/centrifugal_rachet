# Feature Landscape

**Domain:** Cryptographic protocol comparison demo (classical vs post-quantum ratchet)
**Project:** Centrifugal Ratchet
**Researched:** 2026-04-27
**Confidence:** HIGH (protocol specs from signal.org + NIST FIPS 203 + Go stdlib docs)

---

## Concrete Size Reference (verified)

These numbers are the foundation of every metric and panel decision.

### Key sizes (all confirmed from official sources)

| Key/Value | Classical (X25519) | PQ (ML-KEM-768) |
|-----------|-------------------|-----------------|
| Public / encapsulation key | 32 bytes | 1184 bytes (37x) |
| Private / decapsulation key | 32 bytes | 2400 bytes |
| KEM ciphertext | n/a | 1088 bytes |
| Shared secret output | 32 bytes | 32 bytes (same) |

Source: [pkg.go.dev/crypto/mlkem](https://pkg.go.dev/crypto/mlkem), [Curve25519 Wikipedia](https://en.wikipedia.org/wiki/Curve25519)

### X3DH vs PQXDH handshake wire cost

X3DH initial message Alice → Bob:
- IK_A (identity key): 32 bytes
- EK_A (ephemeral key): 32 bytes
- Bob's prekey identifiers: ~8 bytes
- Initial AEAD ciphertext: ~48 bytes (empty plaintext + 16-byte tag)
- **Total: ~120 bytes**

PQXDH initial message Alice → Bob (adds to X3DH):
- All of the above: ~120 bytes
- ML-KEM-768 ciphertext (Alice encapsulates to Bob's PQ prekey): 1088 bytes
- **Total: ~1208 bytes (~10x X3DH)**

Bob's prekey bundle published to server:
- Classical: signed prekey (32 bytes) + one-time prekey (32 bytes) = ~64 bytes
- PQ: adds ML-KEM-768 encapsulation key: 1184 bytes = ~1248 bytes total

Both are single round-trip handshakes (asynchronous, one initial message). No additional handshake messages needed.

Source: [Signal X3DH spec](https://signal.org/docs/specifications/x3dh/), [Signal PQXDH spec](https://signal.org/docs/specifications/pqxdh/)

### Double Ratchet vs Triple Ratchet per-message overhead

Double Ratchet message header (confirmed from Signal spec):
- DH ratchet public key: 32 bytes (sent on ratchet step only, ~every other message)
- Message number (N): 4 bytes
- Previous chain length (PN): 4 bytes
- **Header overhead: ~40 bytes per message**

Triple Ratchet per-message overhead:
- The PQ component (SPQR/SCKA) uses ML-KEM-768 ciphertext: 1088 bytes
- This ciphertext is NOT sent on every message — it is chunked and amortized across multiple messages using erasure codes
- Signal's stated design target: ~40 bytes overhead per message after amortization
- Without chunking (naive): 1088 bytes per ratchet step (every ~2 messages = ~544 bytes/msg average)
- With Katana KEM optimization: ~40% smaller than ML-KEM-768 (future; not in current stdlib)
- **For this demo (no chunking implemented): expect 1088-byte KEM ciphertext injected on each DH ratchet step**

The `go-doubleratchet` `TripleRatchetSession` runs base Double Ratchet and SPQR in parallel, combining keys via hybrid KDF. The `SCKAHeader` in each encrypted message carries opaque SCKA data of variable size.

Source: [Triple Ratchet paper eprint.iacr.org/2025/078](https://eprint.iacr.org/2025/078.pdf), [PQShield analysis](https://pqshield.com/diving-into-signals-new-pq-protocol/), [Signal SPQR blog](https://signal.org/blog/spqr/)

---

## Table Stakes

Features the demo cannot lack — without these, a reader learns nothing useful.

| Feature | Why Expected | Complexity | Concrete Notes |
|---------|--------------|------------|----------------|
| Working classical session (X3DH + Double Ratchet) | Baseline readers know or can look up — must be correct | Low | Use `go-doubleratchet` `Session` + `x3dh` subpackage |
| Working PQ session (PQXDH + Triple Ratchet) | The whole point of the comparison | Medium | Use `TripleRatchetSession` + `pqxdh` subpackage + `crypto/mlkem` |
| Wire-size metric per message | The single most legible number for readers | Low | Prometheus `Histogram`, label `protocol={classical,pq}` |
| Handshake latency metric | Quantifies the setup cost difference | Low | Measure time from start of X3DH/PQXDH to shared-secret established |
| Per-message encrypt/decrypt latency | Shows ongoing cost, not just setup | Low | Histogram with sub-millisecond buckets (1µs, 10µs, 100µs, 1ms) |
| Grafana dashboard committed to repo | Reader sees numbers immediately without instrumentation work | Medium | At minimum: wire size over time, handshake latency stat, message overhead bar |
| `docker compose up` single entry point | Without this, 80% of readers will not run the demo | Low | Centrifugo + 2 client pairs + Prometheus + Grafana |
| README explaining X3DH vs PQXDH and DR vs TR | Audience knows TLS but not ratchets — without context, numbers are meaningless | Medium | See "Blog Reader Needs" section below |
| Both sessions sending/receiving messages continuously | Static one-shot is not useful for time-series charts | Low | Each pair sends on an interval (e.g., every 2 seconds) |
| Message content visible in terminal | Confirms encryption round-trip works, aids debugging | Low | fmt.Printf or log.Printf on successful decrypt |

## Differentiators

Features that make this demo stand out vs other ratchet write-ups.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Live side-by-side Grafana panels (classical left, PQ right) | Readers see the delta in real time, not in a static table | Low-Med | Two identical panel sets, one per protocol, same Y axis scale |
| Exact byte-count annotations in README | "The PQ handshake is 1208 bytes vs 120 bytes" is far more memorable than "PQ is larger" | Low | Derived from the confirmed numbers above |
| Code comments that name the protocol step | `// PQXDH step 3: Alice encapsulates to Bob's ML-KEM-768 signed prekey` — turns code into the article | Low | High value for the blog audience; zero runtime cost |
| State size panel (ratchet state serialized bytes) | Skipped message key cache grows differently between DR and TR | Medium | Serialize state to JSON/protobuf, measure bytes, expose as Gauge |
| Handshake key bundle size panel | Separate from per-message overhead — shows the server storage implication | Low | Gauge showing prekey bundle sizes for both protocols |
| Out-of-order message demonstration | Shows that both implementations correctly handle message reordering — differentiates from naive crypto demos | Medium | See "Out-of-Order Handling" section below |

## Anti-Features

Explicitly avoid — complexity cost exceeds blog demo value.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Web chat UI | Adds a full frontend phase; Grafana tells the story better | Terminal stdout for message content; Grafana for metrics |
| Real user accounts / authentication | Not relevant to the protocol comparison; adds auth complexity | Generate fresh keypairs on each `docker compose up` |
| Persistent message storage | Requires a database; demo resets cleanly on restart | In-memory only; state lives in process |
| Cross-protocol bridging (classical Alice ↔ PQ Bob) | Orthogonal to the comparison goal; confuses the demo narrative | Keep two strictly isolated pairs |
| Chunked/amortized SCKA messages | Production Triple Ratchet sends KEM ciphertext in chunks; implementing this hides the overhead difference you want to show | Send full KEM ciphertext per ratchet step — this makes the size difference MORE visible, which is what you want |
| Key rotation UI or controls | Not needed for a fixed-interval demo loop | Rotation happens automatically on ratchet step |
| Production rate limiting / auth tokens on Centrifugo | Blog demo, not production service | Use Centrifugo's anonymous/insecure mode |
| Throughput benchmarking | Messages/sec is not the comparison axis — bytes and latency are | Keep message rate low (every 2s) to keep charts readable |
| Cryptographic agility (configurable cipher suites) | Adds complexity without adding clarity | Hard-code ML-KEM-768 + X25519 as the specific pair being compared |

---

## Feature Dependencies

```
X3DH handshake → Classical DR session → Classical per-message metrics
PQXDH handshake → Triple Ratchet session → PQ per-message metrics
Both sessions running → Grafana comparison panels → Blog narrative
Prometheus scraping → Grafana datasource → All panels
docker-compose.yml → All of the above
README (X3DH/PQXDH explanation) → Blog article makes sense
```

---

## Grafana Dashboard: Recommended Panels

Minimum viable dashboard for the blog demo.

### Row 1: Handshake (one-time cost)

| Panel | Type | Metric | Notes |
|-------|------|--------|-------|
| Handshake latency — classical vs PQ | Stat (2 cells) | `histogram_quantile(0.99, handshake_latency_seconds_bucket)` | Two stat cells side by side, same threshold coloring |
| Handshake wire bytes — classical vs PQ | Stat (2 cells) | `handshake_bytes_total` Gauge | Show exact numbers: ~120 bytes vs ~1208 bytes |
| Prekey bundle size — classical vs PQ | Stat (2 cells) | `prekey_bundle_bytes` Gauge | ~64 bytes vs ~1248 bytes |

### Row 2: Per-message overhead (ongoing cost)

| Panel | Type | Metric | Notes |
|-------|------|--------|-------|
| Message wire size over time | Time series | `message_wire_bytes` histogram sum/count, labeled by `protocol` | Two overlapping series; PQ line should visibly ride higher |
| Message wire size distribution | Bar gauge or histogram panel | `message_wire_bytes_bucket` | Show the distribution shape — PQ has a bimodal distribution (ratchet-step messages vs non-ratchet messages) |
| Encrypt latency p99 | Time series | `message_encrypt_duration_seconds` p99 by protocol | Sub-millisecond range expected |
| Decrypt latency p99 | Time series | `message_decrypt_duration_seconds` p99 by protocol | Sub-millisecond range expected |

### Row 3: State

| Panel | Type | Metric | Notes |
|-------|------|--------|-------|
| Ratchet state size | Time series | `ratchet_state_bytes` Gauge by protocol | Grows as skipped keys accumulate; different rate for DR vs TR |
| Messages sent/received counter | Stat | `messages_total` counter by protocol and direction | Confirms both sessions are active |

Chart type rationale: Time series for ongoing trends (shows the cost difference persists, not just at startup). Stat panels for the handshake row because those are one-time numbers — a chart with a single dot is misleading. Bar gauge for distribution because bimodal shape of PQ wire size is a key insight.

---

## Metrics: Prometheus Labels and Naming

```
# Handshake metrics (recorded once per session init)
handshake_duration_seconds{protocol="classical|pq"} histogram
handshake_wire_bytes{protocol="classical|pq"} gauge
prekey_bundle_bytes{protocol="classical|pq"} gauge

# Per-message metrics (recorded per message)
message_wire_bytes{protocol="classical|pq", direction="send|recv"} histogram
message_encrypt_duration_seconds{protocol="classical|pq"} histogram
message_decrypt_duration_seconds{protocol="classical|pq"} histogram

# State metrics (polled or recorded after each message)
ratchet_state_bytes{protocol="classical|pq"} gauge
messages_total{protocol="classical|pq", direction="send|recv"} counter
```

Histogram bucket recommendations:
- `message_wire_bytes`: [32, 64, 128, 256, 512, 1024, 1200, 1400, 2048, 4096]  — chosen to bracket classical (~40-byte header + payload) and PQ (~1088+header) messages
- `handshake_duration_seconds`: [0.0001, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5] — µs to 500ms range covers both classical and PQ mobile/desktop
- `message_encrypt_duration_seconds`: [0.000001, 0.00001, 0.0001, 0.001, 0.01] — sub-ms expected

---

## Out-of-Order Message Handling

The Double Ratchet specification handles out-of-order messages via a skipped message key cache (`MKSKIPPED`), indexed by (ratchet_public_key, message_number). The `MaxSkip` constant limits cache size to prevent DoS.

For a blog demo, the recommended approach is:

**Demonstrate, do not stress-test.** Send a deliberate out-of-order message pair (send msg 3 before msg 2, deliver msg 2 after msg 3) and log successful decryption of both. This proves correctness without requiring complex network simulation.

Implementation pattern:
1. Alice sends messages 1, 2, 3 with an artificial hold on message 2
2. Bob receives 1, then 3 (triggering skipped-key storage), then 2 (decrypted from cache)
3. Log the event: `"decrypted out-of-order message 2 (arrived after 3) — skipped key cache worked"`

Do NOT implement: network partitions, reordering at the Centrifugo layer, or replay attack detection. These are production concerns, not blog demo concerns.

The `go-doubleratchet` library handles the skipped-key logic internally. The demo only needs to deliver messages in non-sequential order and confirm decryption succeeds.

---

## Blog Reader Needs: README Structure

Target audience: engineers who understand TLS and symmetric encryption but have not implemented ratchet protocols.

### Recommended README sections (in order)

1. **What you will see** — One sentence: "Run this and watch a Grafana dashboard show the wire-size cost of post-quantum encryption in a live chat." Link to a screenshot of the dashboard.

2. **Quick start** — `git clone` + `docker compose up` + URL to Grafana. Nothing else. Do not explain the code here.

3. **The comparison** — Two-column table:

   | | Classical (X3DH + Double Ratchet) | Post-Quantum (PQXDH + Triple Ratchet) |
   |-|----------------------------------|---------------------------------------|
   | Handshake wire | ~120 bytes | ~1208 bytes |
   | Per-message header | ~40 bytes | ~40–1128 bytes (ratchet step) |
   | Key agreement algorithm | X25519 DH | X25519 DH + ML-KEM-768 KEM |
   | PQ security | No (harvest-now-decrypt-later vulnerable) | Yes (NIST FIPS 203) |
   | Go package | `golang.org/x/crypto/curve25519` | `crypto/mlkem` (Go 1.23+ stdlib) |

4. **How X3DH works** — 200 words max, diagram (ASCII or SVG). Prekey bundle, initial message, shared secret derivation.

5. **How PQXDH differs** — 150 words. One extra step: Alice encapsulates to Bob's ML-KEM key. That ciphertext is the size difference you see in Grafana.

6. **How the Double Ratchet works** — 200 words. KDF chain + DH ratchet. Forward secrecy + break-in recovery.

7. **How the Triple Ratchet differs** — 150 words. Parallel SPQR ratchet. When it fires (DH ratchet step), it adds ~1088 bytes. Combined via hybrid KDF.

8. **Code walkthrough** — Point to specific files. "The handshake is in `internal/classical/handshake.go`, annotated line by line." Do not reproduce the entire file in the README.

9. **Metrics and Grafana** — Explain each dashboard panel in one sentence. Link the JSON.

10. **Architecture** — ASCII diagram of Centrifugo + two channel pairs + Prometheus scrape path.

### Diagram needs (confirmed necessary for the audience)

- Prekey bundle publish / initial message flow (X3DH): sequence diagram, 4 actors (Alice, Server, Bob, time)
- PQXDH diff: same diagram with the KEM ciphertext box highlighted in a different color
- Double Ratchet state machine: two chains (sending, receiving) + DH ratchet arrow
- Docker Compose service graph: which containers communicate with which

Format: ASCII diagrams committed to repo (no external tooling dependency). SVG acceptable if generated and committed.

---

## MVP Recommendation

Prioritize in this order:

1. Classical session end-to-end (X3DH + DR), messages flowing, Prometheus metrics emitted
2. PQ session end-to-end (PQXDH + Triple Ratchet), same metrics
3. Grafana dashboard with Row 1 (handshake stat panels) and Row 2 (wire size time series) — enough to see the difference
4. `docker compose up` brings everything up — this is the deliverable
5. README with "What you will see" + quick start + comparison table

Defer until foundation is solid:
- State size panel (Row 3) — useful but not the primary story
- Out-of-order message demo — adds depth but blocks nothing
- Diagram generation — ASCII is sufficient for MVP, polish later
- Encrypt/decrypt latency panels — secondary to wire size for this audience

---

## Sources

- [Signal X3DH specification](https://signal.org/docs/specifications/x3dh/) — HIGH confidence
- [Signal PQXDH specification](https://signal.org/docs/specifications/pqxdh/) — HIGH confidence
- [Signal Double Ratchet specification](https://signal.org/docs/specifications/doubleratchet/) — HIGH confidence
- [Go crypto/mlkem package docs](https://pkg.go.dev/crypto/mlkem) — HIGH confidence (stdlib, NIST FIPS 203)
- [Triple Ratchet paper — eprint.iacr.org/2025/078](https://eprint.iacr.org/2025/078.pdf) — HIGH confidence (IACR ePrint 2025)
- [Signal SPQR blog post](https://signal.org/blog/spqr/) — HIGH confidence
- [PQShield: Diving into Signal's PQ protocol](https://pqshield.com/diving-into-signals-new-pq-protocol/) — MEDIUM confidence (secondary analysis)
- [Cryspen PQXDH analysis](https://cryspen.com/post/pqxdh/) — MEDIUM confidence (independent security analysis firm)
- [go-doubleratchet GitHub (KushnerykPavel)](https://github.com/KushnerykPavel/go-doubleratchet) — HIGH confidence (the pinned library)
- [Grafana visualization docs](https://grafana.com/docs/grafana/latest/visualizations/panels-visualizations/) — HIGH confidence (official docs)
