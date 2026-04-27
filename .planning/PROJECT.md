# Centrifugal Ratchet

## What This Is

Side-by-side comparison of classical (X3DH + Double Ratchet) vs post-quantum (PQXDH + Triple Ratchet) encrypted chat in a single Go monorepo. One Centrifugo backend carries both protocol pairs simultaneously; a Grafana dashboard shows the cost difference live. Built as a runnable demo to accompany a LinkedIn technical article.

## Core Value

A reader clones the repo, runs `docker compose up`, and immediately sees the real wire-size and latency difference between classical and post-quantum ratchet protocols in a working chat.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Go monorepo with shared module root and per-cmd binaries
- [ ] Single Centrifugo instance routes both classical and PQ message pairs on isolated channels
- [ ] Classical pair: X3DH key agreement + Double Ratchet (KushnerykPavel/go-doubleratchet v0.0.2)
- [ ] PQ pair: PQXDH key agreement using crypto/mlkem (Go 1.23+) + Triple Ratchet (KushnerykPavel/go-doubleratchet v0.0.2)
- [ ] Two isolated Alice–Bob pairs: classical on `ch-classical`, PQ on `ch-pq`
- [ ] Prometheus metrics exported per message: wire size (bytes), handshake latency (ns), per-message encrypt/decrypt overhead
- [ ] Grafana dashboard JSON committed to repo showing all three metric categories side by side
- [ ] Single `docker compose up` starts everything: Centrifugo, both client pairs, Prometheus, Grafana
- [ ] Code samples in README are exact against go-doubleratchet v0.0.2 API
- [ ] README explains X3DH vs PQXDH and DR vs Triple Ratchet for a technical-but-not-crypto audience

### Out of Scope

- Cross-protocol bridging (classical Alice ↔ PQ Bob) — orthogonal to the comparison goal
- Persistent message storage — demo resets on restart, complexity not needed
- Authentication / user accounts — no server-side identity, keys generated fresh each run
- Web chat UI — Grafana covers all visualization; terminal output sufficient for chat panes
- Production hardening (rate limiting, auth tokens) — blog demo, not production service

## Context

- Library: `github.com/KushnerykPavel/go-doubleratchet` v0.0.2 — author's own fork, must match its API exactly
- PQ KEM: `crypto/mlkem` (Go 1.23+ standard library) — no CGo, no external KEM dependency
- Centrifugo: websocket pub/sub server, Go client via `centrifuge-go`
- Metrics pipeline: Prometheus scrape → Grafana; both run as Docker services
- Audience: engineers who know TLS/crypto basics but haven't implemented ratchet protocols
- Deliverable is a blog companion — reproducibility and clarity matter more than throughput

## Constraints

- **Library**: `KushnerykPavel/go-doubleratchet v0.0.2` — all ratchet code must match this API exactly
- **Go version**: 1.23+ required (crypto/mlkem in stdlib)
- **Deployment**: `docker compose up` must be the single entry point — no manual steps
- **No CGo**: crypto/mlkem replaces liboqs-go to keep the build simple
- **Scope**: blog demo — code must be readable/explainable, not optimized for throughput

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| crypto/mlkem for PQ KEM | Go stdlib, no CGo, Go 1.23+ — keeps build hermetic | — Pending |
| Centrifugo as transport | Centrifugo = Centrifugo + ratchet (the project's theme); real pub/sub adds realistic wire overhead | — Pending |
| Two isolated channel pairs | Cleanest comparison — no cross-protocol noise in metrics | — Pending |
| Grafana-only UI | Removes web frontend phase; metrics tell the story better than a chat UI | — Pending |
| go-doubleratchet v0.0.2 pinned | Author's own fork — must be exact for blog code samples | — Pending |

---

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-27 after initialization*
