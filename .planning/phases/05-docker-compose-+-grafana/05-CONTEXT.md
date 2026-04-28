# Phase 5: Docker Compose + Grafana - Context

**Gathered:** 2026-04-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Dockerfiles for all four client binaries, `docker-compose.yml` defining all seven services with healthcheck-ordered startup, Grafana provisioning files (datasource + dashboard provider), and the Grafana dashboard JSON. Running `docker compose up` from a clean clone must start everything without manual steps and populate the dashboard with live metrics.

</domain>

<decisions>
## Implementation Decisions

### Go Builder Image (DEPL-03)

- **D-01:** Builder stage: `golang:1.25-alpine` (NOT `golang:1.23-alpine` as originally in DEPL-03 — go.mod requires go 1.25.0 which go-doubleratchet forced in Phase 1). Final stage: `alpine:latest` (minimal image).
- **D-02:** Multi-stage Dockerfile pattern:
  ```dockerfile
  FROM golang:1.25-alpine AS builder
  WORKDIR /src
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  RUN CGO_ENABLED=0 go build -o /app/binary-name ./cmd/binary-name

  FROM alpine:latest
  RUN apk add --no-cache ca-certificates
  COPY --from=builder /app/binary-name /app/binary-name
  EXPOSE PORT
  HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:PORT/metrics || exit 1
  CMD ["/app/binary-name"]
  ```
- **D-03:** One Dockerfile per binary in each `cmd/binary-name/` directory. Build context is repo root (docker-compose.yml sets `context: .`, `dockerfile: cmd/binary-name/Dockerfile`). This keeps go.mod accessible.
- **D-04:** Ports per binary (HEALTHCHECK URL matches metrics port from Phase 4):
  | Binary | Metrics port | EXPOSE |
  |--------|-------------|--------|
  | alice-classical | 9091 | 9091 |
  | bob-classical | 9092 | 9092 |
  | alice-pq | 9093 | 9093 |
  | bob-pq | 9094 | 9094 |

### docker-compose.yml (DEPL-01, DEPL-02)

- **D-05:** Seven services: `centrifugo`, `bob-classical`, `alice-classical`, `bob-pq`, `alice-pq`, `prometheus`, `grafana`.
- **D-06:** Startup ordering via `depends_on` with `condition: service_healthy`:
  - `bob-classical`, `bob-pq`: depend on `centrifugo` (service_healthy)
  - `alice-classical`, `alice-pq`: depend on `centrifugo` (service_healthy) AND their paired Bob (service_healthy)
  - `prometheus`, `grafana`: depend on `centrifugo` (service_healthy)
- **D-07:** Centrifugo healthcheck: `wget -qO- http://localhost:8000/health || exit 1` (health endpoint enabled in centrifugo/config.json)
- **D-08:** Environment variables in docker-compose.yml:
  ```yaml
  alice-classical:
    environment:
      - CENTRIFUGO_URL=ws://centrifugo:8000/connection/websocket
      - METRICS_PORT=9091
  ```
  Same pattern for all four binaries with their respective ports.
- **D-09:** Volume mounts:
  - centrifugo: `./centrifugo:/centrifugo`
  - prometheus: `./prometheus:/etc/prometheus`
  - grafana: `./grafana/provisioning:/etc/grafana/provisioning` AND `./grafana/dashboards:/var/lib/grafana/dashboards`
- **D-10:** Grafana service: use `grafana/grafana:latest`. Expose port 3000. No auth for demo (`GF_AUTH_ANONYMOUS_ENABLED=true`, `GF_AUTH_ANONYMOUS_ORG_ROLE=Admin`).

### Grafana Provisioning Layout (OBS-05)

- **D-11:** Directory structure:
  ```
  grafana/
  ├── provisioning/
  │   ├── datasources/
  │   │   └── prometheus.yaml    # Prometheus datasource
  │   └── dashboards/
  │       └── dashboard.yaml     # Dashboard provider config
  └── dashboards/
      └── ratchet.json           # Dashboard JSON
  ```
- **D-12:** Datasource file (`grafana/provisioning/datasources/prometheus.yaml`):
  ```yaml
  apiVersion: 1
  datasources:
    - name: Prometheus
      type: prometheus
      access: proxy
      url: http://prometheus:9090
      isDefault: true
  ```
- **D-13:** Dashboard provider (`grafana/provisioning/dashboards/dashboard.yaml`):
  ```yaml
  apiVersion: 1
  providers:
    - name: default
      folder: ''
      type: file
      options:
        path: /var/lib/grafana/dashboards
  ```

### Grafana Dashboard JSON (OBS-05)

- **D-14:** Dashboard title: "Centrifugal Ratchet — Classical vs PQ"
- **D-15:** Five panels per OBS-05 spec:
  1. **Stat: Classical handshake latency** — `rate(ratchet_handshake_duration_seconds_sum{job=~"alice-classical|bob-classical"}[1m]) / rate(ratchet_handshake_duration_seconds_count{job=~"alice-classical|bob-classical"}[1m])` — unit: seconds, title: "Classical Handshake"
  2. **Stat: PQ handshake latency** — same query with `job=~"alice-pq|bob-pq"` — title: "PQ Handshake"
  3. **Time series: Wire size over time** — `rate(ratchet_message_wire_bytes_sum[30s]) / rate(ratchet_message_wire_bytes_count[30s])` for each job — title: "Message Wire Size (bytes)", both classical and PQ overlaid
  4. **Time series: Encrypt latency** — `rate(ratchet_encrypt_duration_seconds_sum[30s]) / rate(ratchet_encrypt_duration_seconds_count[30s])` — title: "Encrypt Latency"
  5. **Time series: Decrypt latency** — same for decrypt — title: "Decrypt Latency"
- **D-16:** Dashboard UID: `"centrifugal-ratchet"`. Dashboard JSON committed to `grafana/dashboards/ratchet.json`.

### Claude's Discretion

- Exact dashboard JSON panel grid positions (row, column, width, height)
- Whether to include `restart: on-failure` or `restart: unless-stopped` on client binaries
- Docker network name (default bridge or named `ratchet`)
- Exact Grafana panel colors and thresholds

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Config Files
- `centrifugo/config.json` — health endpoint enabled on port 8000; Centrifugo service name in docker-compose must match Prometheus scrape target
- `prometheus/prometheus.yml` — targets use Docker service names (alice-classical:9091 etc.)
- `internal/metrics/metrics.go` — histogram names: `ratchet_message_wire_bytes`, `ratchet_handshake_duration_seconds`, `ratchet_encrypt_duration_seconds`, `ratchet_decrypt_duration_seconds`

### Requirements
- `.planning/REQUIREMENTS.md` §Deployment (DEPL-01 through DEPL-03) and §Observability (OBS-05)
- `.planning/PROJECT.md` §Constraints — single `docker compose up` entry point; no CGo

### Prior Phase Decisions
- Phase 4 CONTEXT.md D-01 through D-04 — METRICS_PORT defaults and ports per binary
- Phase 4 prometheus.yml — `scrape_interval: 1s`; Docker service names as targets

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `centrifugo/config.json` — complete, mount to `/centrifugo`
- `prometheus/prometheus.yml` — complete with 4 jobs + labels
- All 4 `cmd/*/main.go` — read `CENTRIFUGO_URL` and `METRICS_PORT` from env; /metrics on respective ports

### Established Patterns
- `centrifugo/` directory pattern → `prometheus/` → `grafana/` (consistent repo layout)
- Alpine base for minimal images (Go binary is CGO_ENABLED=0, no libc needed)

### Integration Points
- Centrifugo service name `centrifugo` matches Prometheus target `centrifugo:8000` and all binary `CENTRIFUGO_URL=ws://centrifugo:8000/...`
- Prometheus service name `prometheus` matches Grafana datasource `url: http://prometheus:9090`
- Binary service names match Prometheus scrape targets exactly

</code_context>

<specifics>
## Specific Ideas

- CGO_ENABLED=0 in Dockerfile build command — ensures static binary, no glibc dependency in alpine final stage
- `apk add --no-cache ca-certificates` in final stage — needed for HTTPS if centrifuge-go uses TLS handshake internally
- `GF_AUTH_ANONYMOUS_ENABLED=true` + `GF_AUTH_ANONYMOUS_ORG_ROLE=Admin` — no login required for blog demo
- Grafana dashboard `"refresh": "5s"` — auto-refresh every 5 seconds to capture short demo runs
- Dashboard `"time": {"from": "now-2m", "to": "now"}` — 2-minute window shows full demo run

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 05-docker-compose-+-grafana*
*Context gathered: 2026-04-28*
