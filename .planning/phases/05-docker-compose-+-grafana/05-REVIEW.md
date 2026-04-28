---
phase: 05-docker-compose-+-grafana
reviewed: 2026-04-28T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - cmd/alice-classical/Dockerfile
  - cmd/bob-classical/Dockerfile
  - cmd/alice-pq/Dockerfile
  - cmd/bob-pq/Dockerfile
  - docker-compose.yml
  - grafana/provisioning/datasources/prometheus.yaml
  - grafana/provisioning/dashboards/dashboard.yaml
  - grafana/dashboards/ratchet.json
findings:
  critical: 2
  warning: 4
  info: 3
  total: 9
status: issues_found
---

# Phase 05: Code Review Report

**Reviewed:** 2026-04-28
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Eight infrastructure files were reviewed: four multi-stage Dockerfiles (one per service), the docker-compose stack, two Grafana provisioning YAML files, and the dashboard JSON. The files are well-structured and demonstrate good patterns (multi-stage builds, CGO disabled, healthchecks, depends_on with `condition: service_healthy`). Two critical issues were found: a non-existent Go base image tag that will cause build failures, and a Grafana admin-open security misconfiguration. Four warnings cover unpinned image tags across all services and a fragile datasource-UID reference pattern in the dashboard. Three informational items round out the review.

---

## Critical Issues

### CR-01: Non-Existent Go Base Image Tag Causes Build Failure

**File:** `cmd/alice-classical/Dockerfile:1`, `cmd/bob-classical/Dockerfile:1`, `cmd/alice-pq/Dockerfile:1`, `cmd/bob-pq/Dockerfile:1`

**Issue:** All four Dockerfiles use `golang:1.25-alpine` as the builder base image. Go 1.25 does not exist — the latest stable release line as of April 2026 is 1.24.x. Docker will fail to pull this image tag, making `docker compose build` fail immediately for every service.

**Fix:** Pin to a real, existing release. Use the latest patch release of the current stable line:

```dockerfile
FROM golang:1.24-alpine AS builder
```

Or pin to a specific patch version for full reproducibility:

```dockerfile
FROM golang:1.24.2-alpine AS builder
```

---

### CR-02: Grafana Anonymous Admin Access

**File:** `docker-compose.yml:94-96`

**Issue:** `GF_AUTH_ANONYMOUS_ENABLED=true` combined with `GF_AUTH_ANONYMOUS_ORG_ROLE=Admin` grants full Grafana Admin privileges (including datasource management, dashboard deletion, user management, and plugin installation) to any unauthenticated HTTP client that can reach port 3000. If the host machine is on a shared network or port 3000 is forwarded, this is an authorization bypass.

**Fix:** For a local demo it is acceptable, but add a comment making the intent explicit and reduce the role to `Viewer` unless write access is actually needed:

```yaml
environment:
  - GF_AUTH_ANONYMOUS_ENABLED=true
  # Viewer is sufficient for read-only dashboard access in a local demo
  - GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer
```

If dashboard editing is needed during development, `Editor` is safer than `Admin`.

---

## Warnings

### WR-01: Unpinned `alpine:latest` in Runtime Stage

**File:** `cmd/alice-classical/Dockerfile:8`, `cmd/bob-classical/Dockerfile:8`, `cmd/alice-pq/Dockerfile:8`, `cmd/bob-pq/Dockerfile:8`

**Issue:** The runtime stage uses `alpine:latest`. The `latest` tag resolves to a different digest on each pull, making builds non-reproducible. A future Alpine release could change library versions or remove packages (`ca-certificates`, `wget`) silently.

**Fix:** Pin to a specific Alpine minor version (patch is managed by Docker Hub for security updates):

```dockerfile
FROM alpine:3.21
```

---

### WR-02: Unpinned Third-Party Image Tags in docker-compose.yml

**File:** `docker-compose.yml:5`, `docker-compose.yml:80`, `docker-compose.yml:92`

**Issue:** Three service images use the `latest` tag:
- `centrifugo/centrifugo:latest` (line 5)
- `prom/prometheus:latest` (line 80)
- `grafana/grafana:latest` (line 92)

Unpinned tags produce non-reproducible environments and can silently introduce breaking API or config changes between `docker compose pull` runs.

**Fix:** Pin to a specific version for each:

```yaml
centrifugo/centrifugo:v5.4.8
prom/prometheus:v3.2.1
grafana/grafana:11.5.2
```

(Verify latest stable versions at the time of pinning.)

---

### WR-03: Dashboard Datasource UID Uses Magic String `"-- Default --"`

**File:** `grafana/dashboards/ratchet.json:13`, `:44`, `:54`, `:85`, `:116`, `:144`, `:165`, `:193`, `:214`

**Issue:** Every panel target and datasource field references `"uid": "-- Default --"`. This is a Grafana internal sentinel that means "use whatever is currently set as the default datasource." If provisioning order changes, if the Prometheus datasource is not yet the default when the dashboard loads, or if a second datasource is later added as default, all panels will silently display no data with no visible error.

**Fix:** Add a stable `uid` to the provisioned datasource and reference it explicitly. In `grafana/provisioning/datasources/prometheus.yaml`:

```yaml
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    uid: prometheus-ds
    isDefault: true
```

Then in `ratchet.json`, replace every `"uid": "-- Default --"` occurrence with:

```json
"datasource": { "type": "prometheus", "uid": "prometheus-ds" }
```

---

### WR-04: Grafana Does Not Declare Dependency on Prometheus

**File:** `docker-compose.yml:102-104`

**Issue:** The `grafana` service declares `depends_on: centrifugo: condition: service_healthy` but does not depend on `prometheus`. Grafana will start and attempt to query the datasource before Prometheus is ready. While Grafana retries datasource connections gracefully, the dashboard may show transient "datasource not found" errors during startup, and the dashboard time window (`now-2m`) is short enough that these errors may persist past the window.

**Fix:**

```yaml
grafana:
  depends_on:
    centrifugo:
      condition: service_healthy
    prometheus:
      condition: service_started
```

`service_started` is sufficient since Prometheus has no healthcheck defined in the compose file. If you add a Prometheus healthcheck (`/-/healthy` endpoint), use `service_healthy` instead.

---

## Info

### IN-01: Containers Run as Root

**File:** `cmd/alice-classical/Dockerfile`, `cmd/bob-classical/Dockerfile`, `cmd/alice-pq/Dockerfile`, `cmd/bob-pq/Dockerfile`

**Issue:** No `USER` directive is set in the runtime stage, so the binary executes as `root` (uid 0) inside the container. This is a container hardening concern; a compromised process would have root-level access to the container filesystem.

**Fix:** Add a non-root user in each runtime stage:

```dockerfile
FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget \
    && addgroup -S app && adduser -S app -G app
COPY --from=builder /app/alice-classical /app/alice-classical
USER app
EXPOSE 9091
```

---

### IN-02: Dashboard Time Window Is Very Narrow for a Demo

**File:** `grafana/dashboards/ratchet.json:246`

**Issue:** `"time": { "from": "now-2m", "to": "now" }` means the dashboard only displays the last 2 minutes of data. With a typical 15s Prometheus scrape interval, there are at most ~8 data points visible. On first stack startup, services may not have produced enough data points to fill this window, leaving panels empty or showing a single `lastNotNull` value.

**Fix:** Widen the default time range to give more context:

```json
"time": { "from": "now-15m", "to": "now" }
```

---

### IN-03: Build Context Copies Entire Repository

**File:** `cmd/alice-classical/Dockerfile:5`, and equivalents in the other three Dockerfiles

**Issue:** `COPY . .` in the builder stage copies the entire repository root (since `context: .` in docker-compose.yml). This includes `.planning/`, test fixtures, and any local secrets in `.env` files if they exist. The final binary-only image is not affected (multi-stage build strips all of this), but it slows builds and risks accidentally including sensitive files in the build layer cache.

**Fix:** Add a `.dockerignore` at the repository root:

```
.planning/
.git/
*.md
grafana/
prometheus/
centrifugo/
```

---

_Reviewed: 2026-04-28_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
