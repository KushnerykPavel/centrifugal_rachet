# Architecture Patterns

**Domain:** Go monorepo — classical vs post-quantum encrypted chat demo
**Researched:** 2026-04-27
**Confidence:** HIGH (official docs + verified patterns)

---

## Recommended Monorepo Directory Layout

```
centrifugal-ratchet/
├── go.work                        # LOCAL ONLY — add to .gitignore
├── go.work.sum
│
├── cmd/
│   ├── alice-classical/
│   │   ├── go.mod                 # module: github.com/you/centrifugal-ratchet/cmd/alice-classical
│   │   ├── main.go
│   │   └── Dockerfile
│   ├── bob-classical/
│   │   ├── go.mod
│   │   ├── main.go
│   │   └── Dockerfile
│   ├── alice-pq/
│   │   ├── go.mod
│   │   ├── main.go
│   │   └── Dockerfile
│   └── bob-pq/
│       ├── go.mod
│       ├── main.go
│       └── Dockerfile
│
├── internal/
│   ├── classical/                 # X3DH + Double Ratchet session logic
│   │   ├── go.mod                 # module: github.com/you/centrifugal-ratchet/internal/classical
│   │   ├── x3dh.go                # prekey bundle gen, key agreement
│   │   └── session.go             # DR session init, encrypt/decrypt wrappers
│   ├── pq/                        # PQXDH + Triple Ratchet session logic
│   │   ├── go.mod                 # module: github.com/you/centrifugal-ratchet/internal/pq
│   │   ├── pqxdh.go               # ML-KEM encapsulation, key agreement
│   │   └── session.go             # Triple Ratchet session init, encrypt/decrypt
│   ├── transport/                 # centrifuge-go wrappers (shared)
│   │   ├── go.mod                 # module: github.com/you/centrifugal-ratchet/internal/transport
│   │   ├── client.go              # NewClient, Subscribe, Publish helpers
│   │   └── channel.go             # channel name constants
│   └── metrics/                   # prometheus instrumentation (shared)
│       ├── go.mod                 # module: github.com/you/centrifugal-ratchet/internal/metrics
│       └── metrics.go             # counters, histograms, HTTP /metrics server
│
├── deploy/
│   ├── centrifugo/
│   │   └── config.json            # Centrifugo server config
│   ├── prometheus/
│   │   └── prometheus.yml         # scrape targets for all 4 processes
│   └── grafana/
│       ├── provisioning/
│       │   ├── datasources/
│       │   │   └── prometheus.yml # Prometheus datasource
│       │   └── dashboards/
│       │       └── dashboards.yml # dashboard provider config
│       └── dashboards/
│           └── ratchet-comparison.json  # committed dashboard JSON
│
└── docker-compose.yml
```

**Key layout decisions:**

- Each binary (`cmd/X`) is its own Go module with its own `go.mod`. This keeps Dockerfiles hermetic and lets `go build` work per-service without the workspace.
- Shared code lives in `internal/` as separate modules. The workspace's `use` directives wire them together locally. In Docker builds, each `cmd/X/go.mod` has a `replace` directive pointing to the local path (or use `GOFLAGS=-mod=mod` with the workspace copied in).
- `deploy/` holds all non-Go config (Centrifugo, Prometheus, Grafana). Keeps infrastructure config co-located with the code that needs it.
- `go.work` is added to `.gitignore` per community consensus — it is a local development convenience, not a build artifact. CI builds each module independently via its `go.mod`.

**Alternative:** single-module layout (`go.mod` at root, `cmd/`, `internal/` as packages, not modules). Simpler `go.work` story (none needed) but all four binaries share one dependency graph. Acceptable for this demo since all four processes use the same library versions. The multi-module layout above is preferred only if you want strict per-binary dependency isolation in CI.

For a blog demo, a **single-module layout** may be more readable:

```
centrifugal-ratchet/
├── go.mod                          # single module root
├── go.sum
├── cmd/
│   ├── alice-classical/main.go
│   ├── bob-classical/main.go
│   ├── alice-pq/main.go
│   └── bob-pq/main.go
├── internal/
│   ├── classical/
│   │   ├── x3dh.go
│   │   └── session.go
│   ├── pq/
│   │   ├── pqxdh.go
│   │   └── session.go
│   ├── transport/
│   │   ├── client.go
│   │   └── channel.go
│   └── metrics/
│       └── metrics.go
└── deploy/
    └── ...                         (same as above)
```

**Recommendation: single-module layout.** The blog audience needs to `go build ./cmd/...` and understand the structure in one pass. The multi-module layout adds go.work complexity that the demo does not need.

---

## Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `internal/classical` | X3DH handshake, DR session encrypt/decrypt | `cmd/alice-classical`, `cmd/bob-classical` |
| `internal/pq` | PQXDH handshake (ML-KEM), Triple Ratchet session | `cmd/alice-pq`, `cmd/bob-pq` |
| `internal/transport` | centrifuge-go client lifecycle, subscribe/publish helpers | All four `cmd/` binaries |
| `internal/metrics` | Prometheus registry, histograms for wire size + latency, `/metrics` HTTP handler | All four `cmd/` binaries |
| `cmd/alice-classical` | Alice role, classical protocol: initiate X3DH, send DR-encrypted messages | Centrifugo via `ch-classical`, exposes `:9101/metrics` |
| `cmd/bob-classical` | Bob role, classical protocol: respond X3DH, receive/decrypt DR messages | Centrifugo via `ch-classical`, exposes `:9102/metrics` |
| `cmd/alice-pq` | Alice role, PQ protocol: initiate PQXDH, send Triple Ratchet messages | Centrifugo via `ch-pq`, exposes `:9103/metrics` |
| `cmd/bob-pq` | Bob role, PQ protocol: respond PQXDH, receive/decrypt Triple Ratchet messages | Centrifugo via `ch-pq`, exposes `:9104/metrics` |
| Centrifugo | WebSocket pub/sub, two isolated channels | All four client binaries |
| Prometheus | Scrapes four `/metrics` endpoints every 5s | Four client binaries |
| Grafana | Renders dashboard from Prometheus data | Prometheus |

---

## Data Flow Diagram

```
                        CENTRIFUGO  :8000
                        ┌──────────────────────────────────────┐
                        │                                      │
           ch-classical │  pub: alice→bob, bob→alice           │
           ─────────────┤  (ciphertext bytes only, no keys)    │
                        │                                      │
           ch-pq        │  pub: alice-pq→bob-pq, bob-pq→alice  │
           ─────────────┤  (ciphertext bytes only, no keys)    │
                        └──────────────────────────────────────┘
                              ▲               ▲
                 subscribe/pub│               │subscribe/pub
          ┌───────────────────┘               └─────────────────┐
          │                                                     │
┌─────────┴──────────────┐                         ┌───────────┴────────────┐
│  alice-classical :9101 │                         │  bob-classical  :9102  │
│  ─────────────────     │  KEY EXCHANGE           │  ─────────────────     │
│  X3DH initiator        │◄────────────────────────│  publishes prekey      │
│  DR session (send)     │  (via ch-classical,     │  bundle at startup     │
│  metrics: wire size,   │   first message is      │  DR session (recv)     │
│  handshake latency     │   InitialMessage)        │  metrics: decrypt      │
└────────────────────────┘                         └────────────────────────┘

┌─────────────────────────┐                        ┌────────────────────────┐
│  alice-pq       :9103   │                        │  bob-pq         :9104  │
│  ─────────────────      │  KEY EXCHANGE          │  ─────────────────     │
│  PQXDH initiator        │◄───────────────────────│  publishes PQ prekey   │
│  Triple Ratchet (send)  │  (via ch-pq,           │  bundle at startup     │
│  metrics: wire size,    │   first message is      │  Triple Ratchet (recv) │
│  handshake latency,     │   PQInitialMessage)     │  metrics: decrypt      │
│  KEM overhead           │                         │                        │
└─────────────────────────┘                        └────────────────────────┘
          │                                                     │
          └─────────────┬───────────────────────────────────────┘
                        │  /metrics scrape (HTTP)
                        ▼
                  PROMETHEUS  :9090
                  ┌──────────────────────────────────────┐
                  │  job: ratchet-clients                 │
                  │  targets:                             │
                  │    alice-classical:9101 {protocol=classical, role=alice} │
                  │    bob-classical:9102   {protocol=classical, role=bob}   │
                  │    alice-pq:9103        {protocol=pq,        role=alice} │
                  │    bob-pq:9104          {protocol=pq,        role=bob}   │
                  └──────────────────────────────────────┘
                        │
                        ▼
                  GRAFANA  :3000
                  ┌──────────────────────────────────────┐
                  │  ratchet-comparison dashboard         │
                  │  panels:                              │
                  │    • wire size: classical vs pq       │
                  │    • handshake latency: classical vs pq│
                  │    • per-msg encrypt/decrypt overhead  │
                  └──────────────────────────────────────┘
```

---

## Key Exchange Sequence

The demo has no key exchange server. Keys are exchanged **in-band over Centrifugo channels** as the first unencrypted message at startup. This is the correct approach for a demo: simple, no extra service, self-contained.

### Classical (X3DH) Key Exchange

```
Bob starts first, publishes prekey bundle to ch-classical (unencrypted JSON):

Bob                                  ch-classical channel          Alice
 │                                          │                        │
 │  1. Generate keys:                       │                        │
 │     IK_B (identity keypair)              │                        │
 │     SPK_B (signed prekey)                │                        │
 │     OPK_B (one-time prekeys)             │                        │
 │                                          │                        │
 │  2. Publish PrekeyBundle{               │                        │
 │       IK_B_pub, SPK_B_pub,              │──────────────────────►│
 │       SPK_B_sig, OPK_B_pub[0]           │                        │
 │     } as msg_type="prekey_bundle"        │                        │
 │                                          │                        │
 │                                          │  3. Alice receives     │
 │                                          │     bundle, validates  │
 │                                          │     SPK signature      │
 │                                          │                        │
 │                                          │  4. Alice computes X3DH:
 │                                          │     IK_A, EK_A (ephemeral)
 │                                          │     SK = KDF(DH(IK_A, SPK_B)
 │                                          │          || DH(EK_A, IK_B)
 │                                          │          || DH(EK_A, SPK_B)
 │                                          │          || DH(EK_A, OPK_B))
 │                                          │                        │
 │                                          │  5. Alice inits DR     │
 │                                          │     session as initiator│
 │                                          │     with SK, SPK_B_pub │
 │                                          │                        │
 │                                          │  6. Alice publishes    │
 │◄─────────────────────────────────────────│     InitialMessage{    │
 │                                          │       IK_A_pub,        │
 │                                          │       EK_A_pub,        │
 │                                          │       OPK_ID_used,     │
 │                                          │       ciphertext[0]    │ (first DR msg)
 │                                          │     }                  │
 │  7. Bob receives InitialMessage,         │                        │
 │     computes same SK from DH params,     │                        │
 │     inits DR session as responder        │                        │
 │     with SK, own SPK keypair             │                        │
 │                                          │                        │
 │  8. Bob decrypts ciphertext[0] ──────────┘                        │
 │     (session established, normal DR                               │
 │      messages follow)                                             │
```

### Post-Quantum (PQXDH) Key Exchange

```
Bob-PQ publishes a PQXDH prekey bundle to ch-pq.
The bundle adds ML-KEM keys on top of the EC keys.

Bob-PQ                               ch-pq channel           Alice-PQ
 │                                          │                        │
 │  1. Generate keys:                       │                        │
 │     IK_B (EC identity)                   │                        │
 │     SPK_B (EC signed prekey)             │                        │
 │     PQSPK_B (ML-KEM-768 keypair,         │                        │
 │              last-resort KEM key)        │                        │
 │     PQOPK_B[0] (one-time KEM prekey)     │                        │
 │                                          │                        │
 │  2. Publish PQPrekeyBundle{             │                        │
 │       IK_B_pub, SPK_B_pub, SPK_B_sig,   │──────────────────────►│
 │       PQSPK_B_pub, PQSPK_B_sig,         │                        │
 │       PQOPK_B_pub[0], PQOPK_B_sig[0]   │                        │
 │     }                                    │                        │
 │                                          │                        │
 │                                          │  3. Alice validates all│
 │                                          │     signatures         │
 │                                          │                        │
 │                                          │  4. Alice computes PQXDH:
 │                                          │     IK_A, EK_A (EC ephemeral)
 │                                          │     (ct, ss) = KEM.Encapsulate(PQOPK_B_pub)
 │                                          │     SK = KDF(DH(IK_A,SPK_B)
 │                                          │          || DH(EK_A,IK_B)
 │                                          │          || DH(EK_A,SPK_B)
 │                                          │          || DH(EK_A,OPK_B) [if used]
 │                                          │          || ss)  ← PQ contribution
 │                                          │                        │
 │                                          │  5. Alice inits Triple │
 │                                          │     Ratchet as initiator│
 │                                          │                        │
 │                                          │  6. Alice publishes    │
 │◄─────────────────────────────────────────│     PQInitialMessage{  │
 │                                          │       IK_A_pub,        │
 │                                          │       EK_A_pub,        │
 │                                          │       PQOPK_ID_used,   │
 │                                          │       kem_ciphertext,  │ ← Alice's KEM ct
 │                                          │       ciphertext[0]    │ (first Triple Ratchet msg)
 │                                          │     }                  │
 │  7. Bob receives PQInitialMessage,       │                        │
 │     ss = KEM.Decapsulate(PQOPK_B_priv,  │                        │
 │          kem_ciphertext)                 │                        │
 │     Computes same SK from DH + ss        │                        │
 │     Inits Triple Ratchet as responder    │                        │
 │                                          │                        │
 │  8. Bob decrypts ciphertext[0]           │                        │
```

**Why in-band channel key exchange for a demo:**
- No key server process to manage or sequence in docker-compose
- The prekey bundle message is tiny and human-readable in logs
- Makes the key exchange visible and auditable — blog readers can see it happen
- Single drawback: Alice must wait for Bob's bundle before starting. Solved by Bob subscribing and publishing at startup with a retry loop, Alice polling with a timeout.

**In-memory storage:** Both sides hold their keys in process memory (`sync.Mutex`-protected struct). No persistence — demo resets cleanly on restart.

---

## Docker Compose Service Dependency Graph

```
                     STARTUP ORDER
                     ─────────────

centrifugo ──────────────────────────────────────────────┐
  healthcheck: GET /health → 200 OK                      │
  (health endpoint enabled via config)                   │
                                                         │ condition: service_healthy
                              ┌──────────────────────────▼──────────────┐
                              │           bob-classical                  │
                              │           bob-pq                         │
                              │           (Bob roles start first —       │
                              │            they publish prekey bundles)  │
                              └──────────────┬───────────────────────────┘
                                             │
                                bob-classical healthy: /metrics 200 OK
                                bob-pq healthy:        /metrics 200 OK
                                             │ condition: service_healthy
                              ┌──────────────▼────────────────────────┐
                              │           alice-classical              │
                              │           alice-pq                     │
                              │           (Alice roles start after Bob │
                              │            prekey bundles are ready)   │
                              └───────────────────────────────────────┘

prometheus ──────────────────────────────────────────────┐
  depends_on: centrifugo (service_healthy)               │
  (scrapes centrifugo metrics too if desired)            │
                                                         │
grafana ─────────────────────────────────────────────────┘
  depends_on: prometheus (service_started)
  (provisioned with datasource + dashboard at startup)
```

**Full docker-compose.yml service list:**

```
services:
  centrifugo       # pub/sub server; healthcheck: GET http://localhost:8000/health
  bob-classical    # waits for centrifugo healthy; healthcheck: GET http://localhost:9102/metrics
  bob-pq           # waits for centrifugo healthy; healthcheck: GET http://localhost:9104/metrics
  alice-classical  # waits for bob-classical healthy (ensures prekey bundle published)
  alice-pq         # waits for bob-pq healthy (ensures PQ prekey bundle published)
  prometheus       # waits for centrifugo healthy
  grafana          # waits for prometheus started
```

**Healthcheck pattern for Go client processes:**

```yaml
healthcheck:
  test: ["CMD-SHELL", "wget -qO- http://localhost:9102/metrics || exit 1"]
  interval: 5s
  timeout: 3s
  retries: 10
  start_period: 10s
```

The `/metrics` endpoint doubles as a readiness probe. Once it responds 200, the process has initialized its Prometheus registry, connected to Centrifugo, and (for Bob roles) published its prekey bundle.

**Alice startup race condition:** Alice waits for Bob's `/metrics` to be healthy (meaning Bob is connected to Centrifugo), but Bob may not have published its prekey bundle in the 5s healthcheck window. Alice should implement a retry loop with a 30s timeout when waiting for Bob's prekey bundle message on the channel. This is simpler than adding a second healthcheck endpoint.

---

## Centrifugo Configuration

```json
{
  "client": {
    "insecure": true,
    "allow_anonymous_connect_without_token": true
  },
  "channel": {
    "without_namespace": {
      "allow_subscribe_for_anonymous": true,
      "allow_publish_for_anonymous": true
    }
  },
  "health": true
}
```

- `client.insecure: true` disables all JWT checks — appropriate for a demo with no authentication requirement.
- `allow_subscribe_for_anonymous` + `allow_publish_for_anonymous` allow anonymous clients (empty user ID) to use channels.
- `health: true` enables `GET /health` → 200 OK for Docker healthchecks.
- No namespace config needed — `ch-classical` and `ch-pq` are flat channel names using the without_namespace defaults.

---

## Prometheus Configuration

```yaml
# deploy/prometheus/prometheus.yml
global:
  scrape_interval: 5s

scrape_configs:
  - job_name: ratchet-clients
    static_configs:
      - targets: ["alice-classical:9101"]
        labels:
          protocol: classical
          role: alice
      - targets: ["bob-classical:9102"]
        labels:
          protocol: classical
          role: bob
      - targets: ["alice-pq:9103"]
        labels:
          protocol: pq
          role: alice
      - targets: ["bob-pq:9104"]
        labels:
          protocol: pq
          role: bob
  - job_name: centrifugo
    static_configs:
      - targets: ["centrifugo:8000"]
```

**Port allocation:**

| Service | Metrics Port |
|---------|-------------|
| alice-classical | 9101 |
| bob-classical | 9102 |
| alice-pq | 9103 |
| bob-pq | 9104 |
| Centrifugo | 8000 (built-in) |
| Prometheus | 9090 |
| Grafana | 3000 |

**Label strategy:** `protocol=classical|pq` and `role=alice|bob` are attached at the Prometheus scrape level via `static_configs.labels`, not emitted by the processes themselves. This keeps the Go processes protocol-agnostic at the metrics layer — the same `internal/metrics` package serves all four. PromQL queries use `{protocol="pq"}` to compare wire sizes.

---

## Grafana Provisioning Layout

```
deploy/grafana/
├── provisioning/
│   ├── datasources/
│   │   └── prometheus.yml          # apiVersion: 1, datasources list
│   └── dashboards/
│       └── dashboards.yml          # type: file, options.path: /var/lib/grafana/dashboards
└── dashboards/
    └── ratchet-comparison.json     # committed dashboard JSON
```

**datasources/prometheus.yml:**

```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

**dashboards/dashboards.yml:**

```yaml
apiVersion: 1
providers:
  - name: default
    type: file
    updateIntervalSeconds: 30
    options:
      path: /var/lib/grafana/dashboards
```

**docker-compose.yml volume mounts for grafana:**

```yaml
grafana:
  image: grafana/grafana:latest
  volumes:
    - ./deploy/grafana/provisioning:/etc/grafana/provisioning
    - ./deploy/grafana/dashboards:/var/lib/grafana/dashboards
```

---

## Prometheus Metrics to Expose

Each process exposes metrics via `internal/metrics`. Recommended metric names:

```
# Wire size per message (bytes)
ratchet_message_wire_bytes_total{direction="send|recv"} counter
ratchet_message_wire_bytes histogram (for distribution)

# Handshake latency (nanoseconds, one observation per session start)
ratchet_handshake_duration_seconds histogram

# Per-message encrypt/decrypt overhead
ratchet_encrypt_duration_seconds histogram
ratchet_decrypt_duration_seconds histogram
```

The `protocol` and `role` labels come from Prometheus scrape config, not the process. Grafana panels use:

```promql
# Wire size comparison (classical vs pq)
histogram_quantile(0.99, sum by (le, protocol) (
  rate(ratchet_message_wire_bytes_bucket[1m])
))

# Handshake latency comparison
histogram_quantile(0.99, sum by (le, protocol) (
  rate(ratchet_handshake_duration_seconds_bucket[1m])
))
```

---

## Build Order (What Must Exist Before What)

```
1. go build ./...
   └── No dependencies between binaries at build time.
       All compile independently once go.mod deps are resolved.

2. Docker images built:
   centrifugo (official image, no build)
   bob-classical, bob-pq (FROM golang:1.23-alpine, go build cmd/bob-classical)
   alice-classical, alice-pq (same)
   prometheus (official image)
   grafana (official image)

3. Runtime startup order (enforced by depends_on + healthchecks):
   centrifugo         → must be healthy first (WebSocket endpoint ready)
   bob-classical      → connects to Centrifugo, publishes prekey bundle
   bob-pq             → connects to Centrifugo, publishes PQ prekey bundle
   alice-classical    → waits for bob-classical healthy, fetches bundle, initiates X3DH
   alice-pq           → waits for bob-pq healthy, fetches bundle, initiates PQXDH
   prometheus         → starts scraping (can start before clients are ready)
   grafana            → renders dashboard (needs prometheus up)

4. In-process initialization order (within each binary):
   a. Register Prometheus metrics
   b. Start /metrics HTTP server (so healthcheck passes immediately)
   c. Connect to Centrifugo
   d. [Bob] Generate keys → publish prekey bundle
      [Alice] Subscribe to channel → wait for prekey bundle (retry loop, 30s timeout)
   e. Perform key agreement, initialize ratchet session
   f. Begin message loop
```

---

## Patterns to Follow

### Pattern 1: Message Type Envelope

All Centrifugo publications carry a JSON envelope with a `type` field. This allows key exchange messages and ratchet messages to coexist on the same channel without an additional control channel.

```go
type Envelope struct {
    Type    string          `json:"type"` // "prekey_bundle" | "initial_msg" | "ratchet_msg"
    Payload json.RawMessage `json:"payload"`
}
```

### Pattern 2: Non-blocking Event Handler

centrifuge-go OnPublication handlers must not block. Move all crypto and Centrifugo publish calls into a goroutine:

```go
sub.OnPublication(func(e centrifuge.PublicationEvent) {
    go func() {
        // decrypt, process, optionally publish response
    }()
})
```

### Pattern 3: Metrics Before Connect

Start the `/metrics` HTTP listener before connecting to Centrifugo. This ensures Docker's healthcheck can pass even if Centrifugo is briefly unavailable, and the process exits cleanly rather than blocking health.

### Pattern 4: Separate Prometheus Registry Per Binary

Use `prometheus.NewRegistry()` instead of the default global registry. This prevents accidental metric leakage if the `internal/metrics` package is tested in isolation.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Key Exchange Out-of-Band (HTTP Server)
**What goes wrong:** Adding a fifth HTTP service for prekey bundle exchange complicates docker-compose startup ordering and adds a service with no blog narrative value.
**Instead:** Exchange prekey bundles in-band as the first Centrifugo message on the channel.

### Anti-Pattern 2: Blocking in centrifuge-go Event Handlers
**What goes wrong:** centrifuge-go runs OnPublication synchronously on the read loop. Any blocking call (Publish, RPC, crypto) deadlocks the connection.
**Instead:** Always `go func() { ... }()` inside event handlers for any non-trivial work.

### Anti-Pattern 3: Global Prometheus Registry
**What goes wrong:** If tests instantiate the metrics package multiple times, duplicate metric registration panics.
**Instead:** Use `prometheus.NewRegistry()` and inject it; avoid `prometheus.MustRegister` on the default registry in library code.

### Anti-Pattern 4: Alice Starts Before Bob
**What goes wrong:** Alice subscribes to the channel, Bob hasn't published its prekey bundle yet, Alice times out.
**Instead:** The depends_on graph ensures Bob is healthy (and has published its bundle) before Alice starts. Alice still implements a retry loop as defense-in-depth.

### Anti-Pattern 5: Committing go.work
**What goes wrong:** go.work contains local path directives that break other developers' setups. CI tools may not respect workspace files, causing inconsistent builds.
**Instead:** Add `go.work` and `go.work.sum` to `.gitignore`. Each module's `go.mod` must be self-contained for Docker builds.

---

## Scalability Considerations

This is a blog demo — scalability is not a requirement. For reference:

| Concern | At demo scale (4 processes) | If it were production |
|---------|-----------------------------|-----------------------|
| Centrifugo connections | 4 persistent WebSocket connections — trivial | Horizontal scale with Redis engine |
| Prometheus scrape | 4 targets, 5s interval — trivial | Use remote_write to Thanos/Cortex |
| Key storage | In-memory, one session per process | Persistent sealed vault |
| Ratchet state | In-memory struct | Encrypted DB with atomic updates |

---

## Sources

- [Centrifugo Channel Namespaces](https://centrifugal.dev/docs/server/channels) — channel isolation, namespace options (HIGH confidence)
- [centrifuge-go pkg.go.dev](https://pkg.go.dev/github.com/centrifugal/centrifuge-go) — client API: NewClient, NewSubscription, Publish, OnPublication (HIGH confidence)
- [Go Workspaces Tutorial](https://go.dev/doc/tutorial/workspaces) — go.work format, use directive (HIGH confidence)
- [Go Workspaces for Monorepos](https://oneuptime.com/blog/post/2026-02-01-go-workspaces-monorepos/view) — monorepo layout patterns (MEDIUM confidence)
- [PQXDH Specification](https://signal.org/docs/specifications/pqxdh/) — prekey bundle structure, Alice's initial message (HIGH confidence)
- [X3DH Specification](https://signal.org/docs/specifications/x3dh/) — prekey bundle, key agreement sequence (HIGH confidence)
- [Docker Compose Startup Order](https://docs.docker.com/compose/how-tos/startup-order/) — depends_on, condition: service_healthy (HIGH confidence)
- [Prometheus Jobs and Instances](https://prometheus.io/docs/concepts/jobs_instances/) — label strategy (HIGH confidence)
- [Grafana Provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/) — datasource.yml, dashboard.yml format (HIGH confidence)
- [Centrifugo Health Endpoint](https://github.com/centrifugal/centrifugo/issues/252) — health: true config option (MEDIUM confidence, GitHub issue)
- [Prometheus client_golang](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus) — NewRegistry, histogram, counter (HIGH confidence)
