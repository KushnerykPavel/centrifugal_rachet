# Phase 2: PQ Session - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-27
**Phase:** 02-pq-session
**Areas discussed:** OPK usage, MLKEMProvider rotation, TripleRatchetMessage encoding, internal/pq API surface

---

## OPK usage

| Option | Description | Selected |
|--------|-------------|----------|
| Skip OPKs | 3-DH only (DH1+DH2+DH3 + KEM). Simpler bundle, cleaner code. | |
| Include OPKs | Full Signal PQXDH: DH4 + PQ OPK. Matches libsignal more exactly. | ✓ |

**User's choice:** Include OPKs

| Option | Description | Selected |
|--------|-------------|----------|
| 1 OPK per session | 1 EC OPK + 1 PQ OPK generated on startup, used once by Alice. | ✓ |
| 0 — skip after all | Reconsider given no-persistence constraint. | |

**User's choice:** 1 OPK per session

**Notes:** Demo resets on restart so no OPK rotation needed. User wants full Signal PQXDH spec compliance for blog accuracy.

---

## MLKEMProvider rotation

| Option | Description | Selected |
|--------|-------------|----------|
| Every message | Maximum KEM overhead visible in every metric sample. Best for blog. | ✓ |
| Every N messages | Reduces PQ overhead; harder to demonstrate cost in Grafana. | |

**User's choice:** Every message

| Option | Description | Selected |
|--------|-------------|----------|
| Receiver generates, sender encapsulates | Bob generates KEM keypair, sends encap key in SCKAHeader.Msg. Alice encapsulates on next send. Matches SPQR spec. | ✓ |
| Fixed role: Alice always encapsulates | Alice uses Bob's static KEM key forever. Breaks forward secrecy. | |

**User's choice:** Receiver generates, sender encapsulates

**Notes:** Both decisions optimize for Phase 4 blog impact — every Grafana sample shows the real per-message KEM cost.

---

## TripleRatchetMessage encoding

| Option | Description | Selected |
|--------|-------------|----------|
| encoding/json for both | Consistent with Phase 3 Centrifugo envelope. Base64-inflates bytes but KEM ciphertext dominates. | ✓ |
| encoding/json for test, raw bytes Phase 4 | Two size definitions — risks blog confusion. | |

**User's choice:** encoding/json for both

**Notes:** Same format for PQ-04 test assertion and Phase 4 histogram observation. Apples-to-apples Grafana comparison.

---

## internal/pq API surface

| Option | Description | Selected |
|--------|-------------|----------|
| Mirror classical signature | Encrypt(plaintext) / Decrypt(*Msg), AD threaded internally. Phase 3 reads identically for both pairs. | ✓ |
| Expose TR native signature | Encrypt(plaintext, ad []byte) / Decrypt(msg, ad []byte). Diverges from classical. | |

**User's choice:** Mirror classical signature

**Notes:** `internal/pq.Session` stores `ad` from `HandshakeResult.AD`, same as classical stores X3DH AD. Phase 3 wiring is symmetric.

---

## Claude's Discretion

- Prekey bundle struct layout (use library `pqxdh.PrekeyBundle` directly vs wrap)
- `MLKEMProvider` internal field names and epoch counter type
- Test helper structure for PQ-04
- PQXDH AD composition (use `HandshakeResult.AD` as-is)

## Deferred Ideas

None.
