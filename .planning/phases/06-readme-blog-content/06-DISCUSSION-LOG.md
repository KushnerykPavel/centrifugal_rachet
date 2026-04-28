# Phase 6: README + Blog Content - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-28
**Phase:** 06-readme-blog-content
**Areas discussed:** Blog artifact location, README depth split, ASCII diagram fidelity, Code sample style

---

## Blog Artifact Location

| Option | Description | Selected |
|--------|-------------|----------|
| Separate blog-draft.md at repo root | Standalone file alongside README, linked from README | ✓ |
| All inline in README | One fat README covering both run-it and understand-it | |
| Separate docs/ directory | docs/blog-post.md with tidier structure | |

**User's choice:** Separate blog-draft.md at repo root, README links to it with a one-liner.

**Notes:** Keeps README lean. blog-draft.md is the article draft ready to paste into LinkedIn/Medium.

---

## README Depth Split

| Option | Description | Selected |
|--------|-------------|----------|
| Run-it + numbers, minimal narrative | Quick-start, table, diagrams, code pointers only | ✓ |
| Full companion document | README includes protocol narrative (~400-600 lines) | |
| Two-section split | Separate "How to run" and "How it works" sections | |

**User's choice:** Run-it + numbers, minimal narrative (~200 lines target).

**Additional sections selected:** Prerequisites, What you'll see, Architecture overview.

**Notes:** Protocol explanation belongs in blog-draft.md. Audience knows TLS basics — no hand-holding on crypto in README.

---

## ASCII Diagram Fidelity

| Option | Description | Selected |
|--------|-------------|----------|
| Message flow only | Alice/Bob actors, arrows for messages sent/received | |
| With key derivation steps | Crypto internals (X25519 DH, KEM, hkdf) shown inline | ✓ |
| Separate diagrams per protocol | Individual diagrams for X3DH and PQXDH at full depth | |

**User's choice:** Detailed key-derivation diagrams — but split: README gets message-flow-only, blog-draft.md gets the full detailed diagrams.

**Notes:** README stays fast to scan. blog-draft.md is where the "why" lives, including the full X3DH and PQXDH key derivation flows.

---

## Code Sample Style

| Option | Description | Selected |
|--------|-------------|----------|
| File links in README, snippets in blog-draft.md | No copy-paste in README, full snippets in blog article | ✓ |
| Inline snippets in both files | Both files have Go code blocks | |
| Links only in both files | No code copy-pasted anywhere | |

**User's choice:** File links in README, Go snippets in blog-draft.md.

**Snippets to include in blog-draft.md:**
- X3DH handshake (InitiatorHandshake / ResponderHandshake)
- PQXDH handshake (MLKEMProvider + TripleRatchetSession init) — the PQ punchline
- Encrypt/Decrypt round-trip — shows identical API surface despite wire cost difference
- Metrics registration — prometheus.NewHistogram from internal/metrics

**Notes:** All snippets must match go-doubleratchet v0.0.2 API exactly. No invented method names.

---

## Claude's Discretion

- blog-draft.md narrative structure and section ordering
- Exact wording of comparison table column headers
- README architecture overview format (table vs ASCII diagram)
- blog-draft.md voice and tone

## Deferred Ideas

None.
