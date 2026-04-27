# Phase 2: PQ Session - Context

**Gathered:** 2026-04-27
**Status:** Ready for planning

<domain>
## Phase Boundary

`internal/pq` package implementing full Signal PQXDH key agreement (with OPKs) and a Triple Ratchet session backed by an `MLKEMProvider` SCKA implementation. Verified by unit tests confirming round-trip decryption and PQ wire overhead. No Centrifugo, no Prometheus, no Docker. All downstream phases depend on this package being correct and building cleanly.

</domain>

<decisions>
## Implementation Decisions

### PQXDH Handshake

- **D-01:** Use the library's `pqxdh.SendHandshake` / `pqxdh.ReceiveHandshake` from `go-doubleratchet v0.0.2`. Do NOT reimplement PQXDH from scratch — the library already implements the full Signal PQXDH spec including signature verification, DH1–DH4, KEM encapsulation, and the correct KDF input order.
- **D-02:** KEM parameter set: **ML-KEM-768** (`pqxdh.MLKEM768`). The library defaults to MLKEM1024 — must explicitly pass `pqxdh.MLKEM768` when generating `KEMSignedPreKey` and `KEMOneTimePreKey`.
- **D-03:** **Include One-Time Prekeys (OPKs)** — full 4-DH handshake (DH1+DH2+DH3+DH4) plus a PQ OPK. Bob generates exactly **1 EC OPK + 1 PQ OPK** on startup. Alice uses them once. No OPK rotation (demo resets on restart; no persistent key store).
- **D-04:** `pqxdh.HandshakeResult` produces `RootKey [32]byte` + `ChainKey [32]byte` + `PQRKey [32]byte`. Pass `RootKey[:]` to `doubleratchet.InitAliceTripleRatchet` / `InitBobTripleRatchet` as the `sharedSecret`. `PQRKey` is not used directly — the library's Triple Ratchet expands the shared secret internally into separate EC and PQ components via HKDF.

### MLKEMProvider (scka.Provider)

- **D-05:** `MLKEMProvider` struct implements `scka.Provider` and rotates the ML-KEM keypair **every message** — `Send()` always encapsulates fresh KEM ciphertext. This maximizes PQ overhead visibility in Phase 4 metrics: every metric sample captures a full KEM operation.
- **D-06:** KEM epoch direction: **receiver generates new keypair, sender encapsulates**. In SPQR terms: when Bob sends, he generates a new ML-KEM keypair and puts the encapsulation key in `SCKAHeader.Msg`. When Alice receives it, she records it. On Alice's next send, she encapsulates against Bob's latest key, producing a ciphertext in her `SCKAHeader.Msg`. Bob decapsulates on receive. Roles are symmetric — each side generates KEM keypairs for the other to encapsulate against.
- **D-07:** `Snapshot()` must deep-copy ALL mutable state: current ML-KEM decapsulation seed, latest received encapsulation key bytes, send/receive counters, current epoch. No shallow copies. Reference: MockSCKA in `scka/testing/mock.go` as the pattern (see canonical refs).
- **D-08:** `MLKEMProvider.Close()` zeros all key material (decapsulation seed, cached encapsulation key bytes) before nil-ing fields.

### Triple Ratchet Session Initialization

- **D-09:** Bob's initial DR ratchet key pair: use `doubleratchet.GenerateKeyPair()` (= `crypto.GenerateKeyPair()`) to generate an X25519 key pair. Bob's DR public key is included in his prekey bundle so Alice can call `InitAliceTripleRatchet(rootKey, bobDRPK, aliceSCKA, nil)`.
- **D-10:** Both Alice and Bob get their own `MLKEMProvider` instance. Alice calls `InitAliceTripleRatchet`; Bob calls `InitBobTripleRatchet` (= `InitResponderTripleRatchet`). Neither provider is shared.

### Session Facade API

- **D-11:** `internal/pq.Session` wraps `*doubleratchet.TripleRatchetSession` and stores `ad []byte` from the PQXDH handshake (`HandshakeResult.AD`), threading it through every Encrypt/Decrypt call. Mirrors `internal/classical.Session` exactly:
  ```go
  func (s *Session) Encrypt(plaintext []byte) (*TripleRatchetMessage, error)
  func (s *Session) Decrypt(msg *TripleRatchetMessage) ([]byte, error)
  ```
  Returns pointer to `TripleRatchetMessage` (unlike library's value return) for consistency with classical. `RootKey [32]byte` exported field for PQ-04 assertion.
- **D-12:** `TripleRatchetMessage` is re-exported as a type alias or local type alias pointing to `doubleratchet.TripleRatchetMessage`. Do NOT redefine it — Phase 3 needs to pass the library type directly.

### Wire-Size Serialization

- **D-13:** `encoding/json` (`json.Marshal`) for **both** classical `*doubleratchet.Message` and `*doubleratchet.TripleRatchetMessage` in PQ-04 tests and in Phase 4 `ratchet_message_wire_bytes` histogram. `[]byte` fields are base64-encoded by JSON — this is acceptable and consistent. The KEM ciphertext (`SCKAHeader.Msg`) for ML-KEM-768 is ~1088 bytes raw (→ ~1452 bytes base64), dominating the PQ message size. Classical `Message.Ciphertext` for the same plaintext is ~40 bytes. The `len(json.Marshal(pqMsg)) > len(json.Marshal(classicalMsg))` assertion in PQ-04 will pass comfortably.
- **D-14:** Phase 4 `ratchet_message_wire_bytes` histogram observes `len(json.Marshal(msg))` — same formula as PQ-04. This ensures the Grafana comparison is apples-to-apples with the test assertion.

### Claude's Discretion

- Prekey bundle struct field layout (whether to use `pqxdh.PrekeyBundle` directly or wrap in a custom `PrekeyBundle` type in `internal/pq`) — prefer using library types directly to avoid translation.
- `MLKEMProvider` internal field names and epoch counter type.
- Test helper structure for PQ-04 (whether to share helpers with classical test or keep separate).
- PQXDH AD composition — use `HandshakeResult.AD` as-is (library already builds it as IKA ‖ IKB ‖ PQPK per spec §4).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Library API (go-doubleratchet v0.0.2)
- `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/pqxdh/pqxdh.go` — `SendHandshake`, `ReceiveHandshake`, `HandshakeResult`, `PrekeyBundle`, `InitialMessage` types
- `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/pqxdh/keys.go` — `IdentityKey`, `SignedPreKey`, `OneTimePreKey`, `KEMSignedPreKey`, `KEMOneTimePreKey`, `KEMPreKey`, `KEMParams`, `GenerateKEMSPK`, `GenerateKEMOPK`
- `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/pqxdh/kem.go` — `KEMParams` constants (`MLKEM768`, `MLKEM1024`), `kemRegistry` pattern
- `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/session_tr.go` — `InitInitiatorTripleRatchet`, `InitResponderTripleRatchet`, `TripleRatchetSession.Encrypt`, `TripleRatchetSession.Decrypt`
- `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/keys.go` — `InitAliceTripleRatchet`, `InitBobTripleRatchet` (aliases), `GenerateKeyPair`
- `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/scka/scka.go` — `scka.Provider` interface (all methods must be implemented by `MLKEMProvider`)
- `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/scka/testing/mock.go` — `MockSCKA` reference implementation for `Snapshot()`/`Restore()` deep-copy pattern
- `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/message.go` — `TripleRatchetMessage`, `TripleRatchetHeader`, `SCKAHeader` types

### Requirements
- `.planning/REQUIREMENTS.md` §PQ Protocol (PQ-01 through PQ-04) — exact implementation contracts and unit test assertions
- `.planning/PROJECT.md` §Constraints — no CGo, Go 1.25+, ML-KEM-768, go-doubleratchet v0.0.2 pinned

### Prior Phase Patterns
- `internal/classical/classical.go` — Session facade pattern to mirror (RootKey field, AD threading, Encrypt/Decrypt signatures, Close method)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/classical/classical.go` — Session struct pattern; copy exactly: `RootKey [32]byte`, `ad []byte`, session field, Encrypt/Decrypt with nil-check, Close method
- `go.mod` — already has `go-doubleratchet v0.0.2` pinned; no new dependencies needed for `internal/pq` (crypto/mlkem is stdlib Go 1.23+)

### Established Patterns
- Error wrapping: `fmt.Errorf("pq: FunctionName: %w", err)` — matches classical pattern
- Key material: `[32]byte` arrays for fixed-size keys, `[]byte` slices for variable (KEM ciphertext, encapsulation keys)
- Test pattern: `require.NoError`, `require.Equal` from `stretchr/testify` — already in go.mod

### Integration Points
- `internal/pq` → used by `cmd/alice-pq` and `cmd/bob-pq` (Phase 3 wires it)
- `internal/pq.Session` → consumed same way as `internal/classical.Session` in Phase 3

</code_context>

<specifics>
## Specific Ideas

- PQ-04 wire-size test: `require.Greater(t, len(pqJSON), len(classicalJSON))` where both are `json.Marshal` of their respective message types for the same plaintext. The ML-KEM-768 ciphertext (~1088 raw bytes → ~1452 base64 bytes) in `SCKAHeader.Msg` alone exceeds the entire classical message.
- PQ-02 Snapshot test: mutate `MLKEMProvider` after `Snapshot()`, then call `Restore(snap)`, then verify mutation is undone. Reference MockSCKA test pattern in `session_spqr_test.go`.
- OPK consumed-on-use: Bob's `OneTimePreKey` and `KEMOneTimePreKey` are passed into `pqxdh.ReceiveHandshake` once and then discarded. No rotation logic needed in Phase 2 (demo resets on restart).

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 02-pq-session*
*Context gathered: 2026-04-27*
