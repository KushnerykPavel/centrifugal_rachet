---
phase: 01
slug: foundation-classical-session
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-27
---

# Phase 01 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| developer machine → Go module proxy | go.sum entries verify download integrity | Go module source code (public) |
| internal/metrics HTTP handler → Prometheus scraper | /metrics endpoint exposes observability data | Timing + wire-size histograms (no secrets) |
| internal/transport → Centrifugo WebSocket | Outbound authenticated WebSocket | Encrypted message payloads |
| OnPublication callback → handler func | handler is caller-supplied code | Raw publication bytes |
| x3dh handshake → doubleratchet session | SharedSecret crosses from X3DH into DR | Shared secret ([32]byte) |
| internal/classical.Session.ad → DR Encrypt/Decrypt | AD binds identity keys to every ciphertext | Public key bytes |
| PrekeyBundle → wire | Identity and pre-key public bytes | Public keys only |
| InitialMessage → wire | Ephemeral key + encrypted X3DH output | Encrypted payload |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-01-01 | Tampering | go.mod/go.sum | mitigate | Pin exact versions; commit go.sum; `go mod verify` confirms checksums | closed |
| T-01-02 | Spoofing | go module proxy | accept | proxy.golang.org + sum.golang.org sufficient for blog demo | closed |
| T-02-01 | Information Disclosure | /metrics endpoint | accept | Timing/wire-size only; no auth needed for blog demo | closed |
| T-02-02 | Denial of Service | prometheus.DefBuckets | accept | Fixed cardinality; no per-message labels | closed |
| T-02-03 | Tampering | ToKey32 input validation | mitigate | Rejects wrong-length slices with error (keys.go:8) | closed |
| T-03-01 | Denial of Service | OnPublication callback blocking | mitigate | Godoc warns handler MUST dispatch via go func() (transport.go:41-43) | closed |
| T-03-02 | Spoofing | Centrifugo insecure mode | accept | insecure:true required by Phase 3 design; not production | closed |
| T-03-03 | Tampering | Publication data integrity | accept | Transport treats data as opaque; Phase 3 adds JSON validation | closed |
| T-03-04 | Elevation of Privilege | Publish without Subscribe | accept | Single-client model; no multi-tenant isolation needed | closed |
| T-04-01 | Spoofing | x3dh PrekeyBundle SPKSignature | mitigate | XEdDSAVerify checks signature; error propagated (classical.go:109-112) | closed |
| T-04-02 | Tampering | doubleratchet.Message in transit | mitigate | AEAD auth failure returns error; wrapper propagates (classical.go:148-151) | closed |
| T-04-03 | Repudiation | Missing associated data | mitigate | AD stored in Session.ad, threaded through every Encrypt/Decrypt (classical.go:46) | closed |
| T-04-04 | Information Disclosure | Session.RootKey field | accept | Test assertion only; would be zeroed in production | closed |
| T-04-05 | Denial of Service | Nil message to Decrypt | mitigate | Explicit nil check returns error (classical.go:145-147) | closed |
| T-04-06 | Elevation of Privilege | DR session reuse across identities | accept | Fresh Session per handshake; no global state | closed |

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01 | T-01-02 | Module proxy + sum database sufficient for blog demo scope | gsd-security-auditor | 2026-04-27 |
| AR-02 | T-02-01 | Metrics contain timing/wire-size only; no auth acceptable for blog demo | gsd-security-auditor | 2026-04-27 |
| AR-03 | T-02-02 | Default buckets produce fixed cardinality; no explosion risk | gsd-security-auditor | 2026-04-27 |
| AR-04 | T-03-02 | insecure:true required by design; not a production service | gsd-security-auditor | 2026-04-27 |
| AR-05 | T-03-03 | Transport is opaque layer; Phase 3 adds validation | gsd-security-auditor | 2026-04-27 |
| AR-06 | T-03-04 | Single-client model; no multi-tenant isolation needed | gsd-security-auditor | 2026-04-27 |
| AR-07 | T-04-04 | RootKey for test transparency; would be zeroed in production | gsd-security-auditor | 2026-04-27 |
| AR-08 | T-04-06 | Fresh Session per handshake; no sharing across identities | gsd-security-auditor | 2026-04-27 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-27 | 15 | 15 | 0 | gsd-security-auditor |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-27
