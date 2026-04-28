# Phase 6: README + Blog Content - Context

**Gathered:** 2026-04-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Deliver two artifacts: `README.md` (operator guide — run it, see the numbers) and `blog-draft.md` (narrative companion — understand why the numbers matter). README stays lean and scannable. blog-draft.md is the article draft, ready to paste into LinkedIn/Medium.

What does NOT go in this phase: new code, new metrics, new services. Documentation only.

</domain>

<decisions>
## Implementation Decisions

### Blog Artifact (plan 06-02)

- **D-01:** Blog content lives in a separate `blog-draft.md` at the **repo root** (not `docs/`, not inline in README). This keeps README lean while giving the article its own file.
- **D-02:** README includes a one-liner link: `📝 Blog post draft: [blog-draft.md](blog-draft.md)` — discoverable but not intrusive.

### README Structure and Depth

- **D-03:** README tone: **run-it + numbers, minimal narrative**. Protocol explanation belongs in blog-draft.md. Target ~200 lines. Audience already knows TLS basics — no hand-holding on crypto fundamentals in the README.
- **D-04:** README sections (in order):
  1. Project name + one-line description
  2. **Prerequisites** — Docker + docker compose version requirements (avoids #1 clone-fail)
  3. **Quick-start** — `git clone` + `docker compose up` (DEPL-04)
  4. **What you'll see** — brief description of terminal output + Grafana URL (`http://localhost:3000`)
  5. **Comparison table** — exact byte numbers: 120 vs 1208 bytes handshake, 40 vs 1128 bytes ratchet step (DEPL-04)
  6. **Architecture overview** — one-paragraph or ASCII diagram of the 7-service topology
  7. **ASCII sequence diagrams** — message-flow-only (fast scan; detailed key-derivation diagrams live in blog-draft.md)
  8. **Code pointers** — links to key source files (no inline snippets in README)
  9. 📝 Blog post draft link

### ASCII Sequence Diagrams

- **D-05:** README gets **message-flow-only** diagrams for X3DH and PQXDH — Alice/Bob actors, arrows showing what gets sent when (prekey bundle, initial_msg, ratchet_msg). No crypto internals. Fast to scan.
- **D-06:** `blog-draft.md` gets **detailed diagrams with key derivation steps** — showing X25519 DH, ML-KEM encapsulate/decapsulate, hkdf calls, RootKey derivation inline as diagram notes. These are the diagrams that explain the WHY.

### Code Samples

- **D-07:** README contains **file links only** — e.g., `Key exchange: [internal/classical/session.go](internal/classical/session.go)`. No copy-pasted snippets in README (zero staleness risk).
- **D-08:** `blog-draft.md` contains **Go code snippets** for these four sections (all must match go-doubleratchet v0.0.2 API exactly):
  1. **X3DH handshake** — `InitiatorHandshake` / `ResponderHandshake` API calls
  2. **PQXDH handshake** — `MLKEMProvider` construction + `TripleRatchetSession` init (the PQ-specific punchline)
  3. **Encrypt/Decrypt round-trip** — `session.Encrypt` / `session.Decrypt` showing identical API surface despite different wire cost
  4. **Metrics registration** — `prometheus.NewHistogram` call from `internal/metrics` showing how wire-size is measured

### Claude's Discretion

- blog-draft.md narrative structure (intro, body sections, conclusion ordering) — Claude decides
- Exact wording of comparison table column headers
- Whether README uses a markdown table or ASCII art for the 7-service architecture overview
- Tone of blog-draft.md (first-person engineer voice vs neutral technical writeup)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — DEPL-04 specifies exact README content requirements (quick-start, comparison table with exact numbers, ASCII diagrams, code pointers)
- `.planning/PROJECT.md` — audience definition (engineers who know TLS/crypto basics), blog-companion context, core value statement

### Protocol Implementation (for code snippets accuracy)
- `internal/classical/session.go` — X3DH + Double Ratchet session; source of truth for API calls in blog-draft.md
- `internal/pq/session.go` — PQXDH + Triple Ratchet session; source of truth for PQ API calls
- `internal/metrics/metrics.go` — Prometheus histogram registrations; source of truth for metrics snippet
- `go.mod` — confirms go-doubleratchet v0.0.2 and Go 1.25.0 — code samples must match exactly

### Prior Phase Context (for exact numbers and service topology)
- `.planning/phases/05-docker-compose-+-grafana/05-02-SUMMARY.md` — 7 services, ports, depends_on ordering (for architecture overview)
- `.planning/phases/05-docker-compose-+-grafana/05-04-SUMMARY.md` — final Grafana dashboard config, Grafana URL

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/classical/session.go` — X3DH + DR session; contains the actual go-doubleratchet v0.0.2 API calls to quote
- `internal/pq/session.go` — PQXDH + Triple Ratchet; contains MLKEMProvider and TripleRatchetSession
- `internal/metrics/metrics.go` — Prometheus histogram definitions
- `docker-compose.yml` — 7-service topology for architecture overview

### Established Patterns
- No existing README.md — creating from scratch
- Go code already written and verified — snippets must quote actual function names, not invent them

### Integration Points
- README.md at repo root (standard location)
- blog-draft.md at repo root (D-01)
- No new code files — documentation only

</code_context>

<specifics>
## Specific Ideas

- Comparison table exact numbers from DEPL-04: **120 vs 1208 bytes** (handshake), **40 vs 1128 bytes** (ratchet step) — these are the research numbers, must match a running instance per ROADMAP SC2
- Grafana dashboard URL: `http://localhost:3000`
- blog-draft.md should show that despite the 10x wire overhead, the PQXDH API surface is nearly identical to X3DH — this is the engineering insight the article is built around

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 06-readme-blog-content*
*Context gathered: 2026-04-28*
