# Phase 2: PQ Session - Research

**Researched:** 2026-04-27
**Domain:** Post-Quantum cryptography — PQXDH + Triple Ratchet via go-doubleratchet v0.0.2
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Use `pqxdh.SendHandshake` / `pqxdh.ReceiveHandshake` from go-doubleratchet v0.0.2. Do NOT reimplement PQXDH.
- **D-02:** KEM parameter set: ML-KEM-768 (`pqxdh.MLKEM768`). Library defaults to MLKEM1024 — must explicitly pass `pqxdh.MLKEM768`.
- **D-03:** Include OPKs — 1 EC OPK + 1 PQ OPK per session. No rotation (demo resets on restart).
- **D-04:** Pass `HandshakeResult.RootKey[:]` to `InitAliceTripleRatchet` / `InitBobTripleRatchet` as `sharedSecret`. `PQRKey` not used directly.
- **D-05:** `MLKEMProvider` rotates ML-KEM keypair every message — `Send()` always encapsulates fresh KEM ciphertext.
- **D-06:** Receiver generates new keypair, sender encapsulates. Symmetric — each side generates for the other.
- **D-07:** `Snapshot()` must deep-copy ALL mutable state: decapsulation seed, latest received encapsulation key bytes, send/receive counters, current epoch.
- **D-08:** `MLKEMProvider.Close()` zeros all key material before nil-ing fields.
- **D-09:** Bob's initial DR ratchet key pair: use `doubleratchet.GenerateKeyPair()`. Bob's DR public key included in prekey bundle.
- **D-10:** Alice and Bob each get their own `MLKEMProvider` instance. Neither is shared.
- **D-11:** `internal/pq.Session` wraps `*doubleratchet.TripleRatchetSession`, stores `ad []byte` from `HandshakeResult.AD`. Mirrors classical facade exactly: `Encrypt([]byte) (*TripleRatchetMessage, error)` and `Decrypt(msg *TripleRatchetMessage) ([]byte, error)`. Returns pointer to `TripleRatchetMessage`. `RootKey [32]byte` exported.
- **D-12:** `TripleRatchetMessage` is re-exported as type alias pointing to `doubleratchet.TripleRatchetMessage`. Do NOT redefine.
- **D-13:** `encoding/json` for both classical `*doubleratchet.Message` and `*doubleratchet.TripleRatchetMessage` in PQ-04 tests. ML-KEM-768 ciphertext ~1088 bytes raw → ~1452 bytes base64.
- **D-14:** Phase 4 `ratchet_message_wire_bytes` observes `len(json.Marshal(msg))`.

### Claude's Discretion

- Prekey bundle struct field layout — prefer library types directly.
- `MLKEMProvider` internal field names and epoch counter type.
- Test helper structure for PQ-04 (separate from classical test or shared).
- PQXDH AD composition — use `HandshakeResult.AD` as-is.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PQ-01 | `internal/pq` implements PQXDH key agreement combining ML-KEM-768 with X25519 DH via HKDF in Signal-spec KDF input order | `pqxdh.SendHandshake`/`ReceiveHandshake` implement this internally; caller passes `pqxdh.MLKEM768` param |
| PQ-02 | `internal/pq` provides `MLKEMProvider` implementing `scka.Provider`, wrapping `crypto/mlkem`; `Snapshot()` deep-copies all key material | `scka.Provider` interface verified; `MockSCKA` deep-copy pattern verified |
| PQ-03 | `internal/pq` initializes `TripleRatchetSession` from PQXDH outputs; exposes `Encrypt`/`Decrypt` facade | `InitAliceTripleRatchet`/`InitBobTripleRatchet` signatures verified; `TripleRatchetSession.Encrypt` returns value type |
| PQ-04 | Unit test confirms round-trip and `len(serialized TripleRatchetMessage) > len(serialized *Message)` for same plaintext | ML-KEM-768 CT size ~1088 bytes raw dominates PQ message; classical message ~40 bytes CT |
</phase_requirements>

---

## Summary

Phase 2 creates `internal/pq` — a fully self-contained Go package implementing PQXDH key agreement and a Triple Ratchet session. The library `go-doubleratchet v0.0.2` provides all cryptographic primitives; this phase's job is to wire them together correctly and implement the `scka.Provider` interface with a real ML-KEM-768 provider.

All API signatures have been verified directly from library source. The three critical correctness risks are: (1) passing `pqxdh.MLKEM768` explicitly — the library defaults to MLKEM1024; (2) `MLKEMProvider.Snapshot()` must be a genuine deep copy of all `[]byte` fields — the SPQR layer calls `Restore()` on auth failure and shallow copies corrupt session state; (3) `TripleRatchetSession.Encrypt` returns a **value** (`TripleRatchetMessage`, not `*TripleRatchetMessage`) — the Session facade wraps it in a pointer for API consistency with classical.

The PQ-04 wire-size assertion will pass comfortably: ML-KEM-768 produces 1088-byte ciphertexts, which JSON-encodes to ~1452 base64 bytes; the entire classical `doubleratchet.Message` for the same plaintext is ~80-120 bytes total.

**Primary recommendation:** Build in three files — `provider.go` (MLKEMProvider), `pq.go` (Session facade + handshake functions + prekey bundle types), `pq_test.go` (PQ-01 through PQ-04 acceptance tests). No new dependencies needed.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| PQXDH key agreement | Library (`pqxdh` pkg) | `internal/pq` orchestration | Library implements Signal spec; `internal/pq` calls it |
| ML-KEM operations (encap/decap) | stdlib `crypto/mlkem` | `MLKEMProvider` | Provider wraps stdlib; no direct KEM calls in session code |
| Triple Ratchet state machine | Library (`doubleratchet` pkg) | `internal/pq.Session` facade | Library owns ratchet; facade threads AD and exposes simplified API |
| SCKA epoch management | Library (`SPQRSession`) | `MLKEMProvider.Send()/Receive()` | Library calls Provider; Provider controls when to rotate KEM keys |
| Snapshot/Restore rollback | `MLKEMProvider` | Library calls it | Library triggers rollback on auth failure; Provider must deep-copy |
| Serialization (wire size) | Test code | Phase 4 metric layer | JSON marshaling for PQ-04 test; same formula reused in Phase 4 |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| go-doubleratchet | v0.0.2 (pinned) | PQXDH + Triple Ratchet + scka.Provider interface | Project-mandated; implements Signal PQXDH spec |
| crypto/mlkem | Go stdlib (Go 1.23+) | ML-KEM-768/1024 encapsulation/decapsulation | No CGo, hermetic build, FIPS 203 compliant |
| encoding/json | Go stdlib | Wire-size serialization in tests | Project-mandated (D-13) |
| stretchr/testify | v1.11.1 (in go.mod) | `require.NoError`, `require.Equal`, `require.Greater` in tests | Already in go.mod; used in Phase 1 tests |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| crypto/rand | Go stdlib | Random seed generation for KEM keypairs | `MLKEMProvider.Send()` — generate fresh 64-byte seed |
| fmt | Go stdlib | Error wrapping | `fmt.Errorf("pq: FunctionName: %w", err)` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| crypto/mlkem | liboqs-go | CGo required — explicitly out of scope per REQUIREMENTS.md |
| encoding/json | protobuf | Simpler but inconsistent with test assertion formula |

**Installation:** No new dependencies — all packages already in `go.mod`.

**Version verification:** [VERIFIED: go.mod] `go-doubleratchet v0.0.2`, `go 1.25.0`. `crypto/mlkem` available since Go 1.23 — confirmed via library source which imports it.

---

## Architecture Patterns

### System Architecture Diagram

```
Bob startup:
  pqxdh.GenerateIdentityKey()  →  bobIK
  pqxdh.GenerateSPK(bobIK, 1)  →  bobSPK
  pqxdh.GenerateOPK(1)          →  bobOPK (EC)
  pqxdh.GenerateKEMSPK(bobIK, 1, MLKEM768) →  bobKEMSPK
  pqxdh.GenerateKEMOPK(bobIK, 1, MLKEM768) →  bobKEMOPK
  doubleratchet.GenerateKeyPair()  →  bobDRPriv, bobDRPub
  Publish: PrekeyBundle{..., bobDRPub}

Alice startup:
  pqxdh.GenerateIdentityKey()  →  aliceIK
  pqxdh.SendHandshake(aliceIK, &PrekeyBundle{...}) 
    → HandshakeResult{RootKey, ChainKey, PQRKey, AD}, InitialMessage
  MLKEMProvider (alice) — new instance, InitInitiator called by Triple Ratchet
  doubleratchet.InitAliceTripleRatchet(RootKey[:], bobDRPub, aliceSCKA, nil)
    → *TripleRatchetSession
  internal/pq.Session{RootKey, ad=AD, tr=session}
  Send: InitialMessage

Bob on receive InitialMessage:
  bobOPKPreKey := bobOPK.DecapsKey()  (EC OPK — not used here, pass as *OneTimePreKey)
  bobKEMOPKPreKey := bobKEMOPK.DecapsKey()
  pqxdh.ReceiveHandshake(bobIK, &bobSPK, &bobOPK, bobKEMOPKPreKey, &InitialMessage)
    → HandshakeResult{RootKey, AD}
  MLKEMProvider (bob) — new instance, InitResponder called by Triple Ratchet
  bobDRKP := crypto.KeyPair{PrivateKey: bobDRPriv, PublicKey: bobDRPub}
  doubleratchet.InitBobTripleRatchet(RootKey[:], bobDRKP, bobSCKA, nil)
    → *TripleRatchetSession
  internal/pq.Session{RootKey, ad=AD, tr=session}

Message send (e.g., Alice):
  Session.Encrypt(plaintext)
    → tr.Encrypt(plaintext, s.ad)  [scka.Provider.Send() called internally]
    → TripleRatchetMessage (value)
    → return &msg, nil

Message receive (e.g., Bob):
  Session.Decrypt(msg *TripleRatchetMessage)
    → tr.Decrypt(*msg, s.ad)  [scka.Provider.Receive() called internally]
    → []byte plaintext
```

### Recommended Project Structure
```
internal/pq/
├── provider.go    # MLKEMProvider — scka.Provider implementation
├── pq.go          # Session facade, handshake functions, PrekeyBundle/ResponderKeys types
└── pq_test.go     # PQ-01 through PQ-04 acceptance tests
```

### Pattern 1: PQXDH SendHandshake (Initiator/Alice)

**What:** Alice-side PQXDH using library's full Signal spec implementation.

**Exact signature:** [VERIFIED: pqxdh/pqxdh.go line 59]
```go
// Source: pqxdh/pqxdh.go
func SendHandshake(senderIK IdentityKey, bundle *PrekeyBundle) (HandshakeResult, InitialMessage, error)
```

**PrekeyBundle fields required by SendHandshake:** [VERIFIED: pqxdh/pqxdh.go]
```go
// Source: pqxdh/pqxdh.go — PrekeyBundle type
bundle := &pqxdh.PrekeyBundle{
    IdentityKey:       bobIK.PublicKey,   // [32]byte
    SignedPreKey:      bobSPK.PublicKey,  // [32]byte
    SPKID:             bobSPK.KeyID,      // uint32
    SPKSignature:      bobSPK.Signature,  // [64]byte
    PQPreKey:          bobKEMSPK.EncapsulationKey, // []byte — the encapsulation key bytes
    PQPreKeyID:        bobKEMSPK.KeyID,  // uint32
    PQPreKeySignature: bobKEMSPK.Signature, // [64]byte
    PQParams:          pqxdh.MLKEM768,   // CRITICAL: must be explicit
    OneTimePreKey:     &bobOPK.PublicKey, // *[32]byte — nil omits DH4
    OPKID:             &bobOPK.KeyID,    // *uint32
}
```

**HandshakeResult:** [VERIFIED: pqxdh/pqxdh.go lines 40-46]
```go
type HandshakeResult struct {
    AD       []byte   // IKA_pub ‖ IKB_pub ‖ PQPK_encapKey (built by library)
    RootKey  [32]byte // feeds Triple Ratchet shared secret
    ChainKey [32]byte // not used by this project
    PQRKey   [32]byte // not used directly — library expands internally
}
```

### Pattern 2: PQXDH ReceiveHandshake (Responder/Bob)

**Exact signature:** [VERIFIED: pqxdh/pqxdh.go lines 154-160]
```go
// Source: pqxdh/pqxdh.go
func ReceiveHandshake(
    receiverIK IdentityKey,
    spk *SignedPreKey,
    opk *OneTimePreKey,   // EC OPK — pass pointer; nil if not used
    pqpk *KEMPreKey,      // obtain via bobKEMOPK.DecapsKey()
    msg *InitialMessage,
) (HandshakeResult, error)
```

**CRITICAL:** `pqpk` is a `*KEMPreKey`, obtained via `.DecapsKey()` on `KEMOneTimePreKey` or `KEMSignedPreKey`. [VERIFIED: pqxdh/kem.go lines 132-138]
```go
// Source: pqxdh/kem.go
func (k *KEMOneTimePreKey) DecapsKey() *KEMPreKey {
    return &KEMPreKey{EncapsulationKey: k.EncapsulationKey, Seed: k.Seed, Params: k.Params}
}
```

**KEM params guard:** ReceiveHandshake validates `pqpk.Params == msg.PQParams` — mismatch returns error. [VERIFIED: pqxdh/pqxdh.go line 163]

### Pattern 3: Triple Ratchet Initialization

**InitAliceTripleRatchet signature:** [VERIFIED: keys.go line 60 + session_tr.go line 36]
```go
// Source: keys.go (alias for InitInitiatorTripleRatchet)
func InitAliceTripleRatchet(
    sharedSecret []byte,     // HandshakeResult.RootKey[:] — must be >= 32 bytes
    bobDRPK [32]byte,        // Bob's DR public key — [32]byte value, NOT slice
    sckaProvider scka.Provider,
    cfg *Config,             // nil → DefaultMaxSkip
) (*TripleRatchetSession, error)
```

**InitBobTripleRatchet signature:** [VERIFIED: keys.go line 67 + session_tr.go line 73]
```go
// Source: keys.go (alias for InitResponderTripleRatchet)
func InitBobTripleRatchet(
    sharedSecret []byte,
    bobKeyPair crypto.KeyPair,   // crypto.KeyPair = doubleratchet.KeyPair (type alias)
    sckaProvider scka.Provider,
    cfg *Config,
) (*TripleRatchetSession, error)
```

**KeyPair type:** [VERIFIED: keys.go lines 8-10]
```go
// Source: keys.go
type KeyPair = crypto.KeyPair  // field names: PrivateKey [32]byte, PublicKey [32]byte

// Generate with:
privKey, pubKey, err := doubleratchet.GenerateKeyPair()
bobKP := doubleratchet.KeyPair{PrivateKey: privKey, PublicKey: pubKey}
```

**Note:** `InitBobTripleRatchet` is deprecated in favor of `InitResponderTripleRatchet` — both do the same thing (it's just an alias). Either is correct; use `InitBobTripleRatchet` for naming consistency with classical.

### Pattern 4: TripleRatchetSession.Encrypt/Decrypt

**CRITICAL: returns VALUE, not pointer:** [VERIFIED: session_tr.go lines 110, 190]
```go
// Source: session_tr.go
func (s *TripleRatchetSession) Encrypt(plaintext, ad []byte) (TripleRatchetMessage, error)
func (s *TripleRatchetSession) Decrypt(msg TripleRatchetMessage, ad []byte) ([]byte, error)
```

The Session facade wraps this:
```go
// internal/pq/pq.go — Session facade pattern
func (s *Session) Encrypt(plaintext []byte) (*TripleRatchetMessage, error) {
    msg, err := s.tr.Encrypt(plaintext, s.ad)
    if err != nil {
        return nil, fmt.Errorf("pq: Encrypt: %w", err)
    }
    return &msg, nil
}

func (s *Session) Decrypt(msg *TripleRatchetMessage) ([]byte, error) {
    if msg == nil {
        return nil, errors.New("pq: Decrypt: nil message")
    }
    plain, err := s.tr.Decrypt(*msg, s.ad)
    if err != nil {
        return nil, fmt.Errorf("pq: Decrypt: %w", err)
    }
    return plain, nil
}
```

### Pattern 5: scka.Provider Interface (MLKEMProvider)

**Complete interface:** [VERIFIED: scka/scka.go]
```go
// Source: scka/scka.go
type Provider interface {
    InitInitiator(sk []byte) error
    InitResponder(sk []byte) error
    Send() (msg []byte, sendingEpoch uint32, outputKey []byte, keyEpoch uint32, err error)
    Receive(msg []byte) (receivingEpoch uint32, outputKey []byte, keyEpoch uint32, err error)
    Snapshot() any
    Restore(snapshot any)
    Close() error
}
```

**How SPQR calls the provider:** [VERIFIED: session_spqr.go lines 143-170]
- `Send()` is called once per message encrypt. The `msg []byte` return is placed in `SCKAHeader.Msg` and transmitted.
- `Receive(header.Msg)` is called once per message decrypt. The `msg` is the `SCKAHeader.Msg` from the incoming message.
- `outputKey` (if non-nil) triggers a KDF ratchet step — SPQR derives new chain keys. `keyEpoch` must equal `s.epoch + 1`.
- `sendingEpoch` / `receivingEpoch` are used for chain key lookup — the SPQR uses epoch-keyed chain maps.

**MLKEMProvider epoch and key rotation design:**

Per D-05/D-06: receiver generates new ML-KEM keypair and puts encapsulation key in `Send()` msg. Sender reads it from `Receive()` msg, then encapsulates on next `Send()`.

```
Alice Send():
  - Generate fresh 64-byte seed → mlkem.NewDecapsulationKey768(seed) → dk
  - encapKeyBytes = dk.EncapsulationKey().Bytes()
  - msg = encapKeyBytes  (Alice's new KEM encapsulation key for Bob to use)
  - outputKey, keyEpoch = <derived from encapsulation result if Bob's key is known>

Bob Receive(aliceMsg):
  - Store aliceMsg as latestPeerEncapKey
  - Encapsulate against latestPeerEncapKey → (ct, ss)
  - outputKey = ss[:32]  (new epoch key material)
  - keyEpoch = s.epoch + 1

Bob Send():
  - msg = ct  (ciphertext for Alice to decapsulate)
  - sendingEpoch = current epoch

Alice Receive(bobMsg):
  - Decapsulate bobMsg using current decapsulation key seed → ss
  - outputKey = ss[:32]
  - keyEpoch = s.epoch + 1
```

**Wire content of SCKAHeader.Msg (ML-KEM-768):**
- On "key announcement" send: 1184 bytes (ML-KEM-768 encapsulation key size)
- On "encapsulation" send: 1088 bytes (ML-KEM-768 ciphertext size)
These are the values that make `len(json.Marshal(pqMsg)) >> len(json.Marshal(classicalMsg))`.

### Pattern 6: Snapshot/Restore Deep Copy

**Reference implementation:** [VERIFIED: scka/testing/mock.go lines 114-146]
```go
// Source: scka/testing/mock.go — canonical pattern for deep copy
func (m *MockSCKA) Snapshot() any {
    snap := &mockSCKASnapshot{
        sendCount:    m.SendCount,
        receiveCount: m.ReceiveCount,
        // ... scalar fields copied by value
    }
    if m.SharedKey != nil {
        snap.sharedKey = append([]byte(nil), m.SharedKey...)  // deep copy []byte
    }
    // ...
    return snap
}

func (m *MockSCKA) Restore(snapshot any) {
    snap, ok := snapshot.(*mockSCKASnapshot)
    if !ok {
        return
    }
    m.SharedKey = snap.sharedKey  // replace entire slice
    // ...
}
```

**For MLKEMProvider, the snapshot struct must capture:**
- `decapSeed [64]byte` — current decapsulation key seed (value copy)
- `latestPeerEncapKey []byte` — latest received encapsulation key (deep copy)
- `sendEpoch uint32`
- `recvEpoch uint32`
- `initialized bool`

### Pattern 7: GenerateKEMOPK

**Signature:** [VERIFIED: pqxdh/kem.go line 161]
```go
// Source: pqxdh/kem.go
func GenerateKEMOPK(ik IdentityKey, keyID uint32, params KEMParams) (KEMOneTimePreKey, error)
```

**Fields:** `KEMOneTimePreKey{EncapsulationKey []byte, Params KEMParams, KeyID uint32, Seed [64]byte, Signature [64]byte}`

**PrekeyBundle.PQPreKey** takes `EncapsulationKey []byte` (not the full struct). Bob stores the full `KEMOneTimePreKey` privately and uses `.DecapsKey()` when processing Alice's `InitialMessage`.

### Anti-Patterns to Avoid

- **Passing `MLKEM1024` (library default):** The `kemRegistry` defaults to MLKEM1024 in the library's HKDF info label. Explicitly pass `pqxdh.MLKEM768` everywhere.
- **Shallow-copying `[]byte` fields in Snapshot:** `snap.key = m.key` copies the slice header, not the data. Use `append([]byte(nil), m.key...)`.
- **Passing `sharedSecret` shorter than 32 bytes:** `InitInitiatorTripleRatchet` and `InitResponderTripleRatchet` return `ErrSharedSecretTooShort` if `len(sharedSecret) < 32`. `HandshakeResult.RootKey[:]` is exactly 32 bytes — safe.
- **Redefining `TripleRatchetMessage`:** D-12 forbids it. Use a type alias: `type TripleRatchetMessage = doubleratchet.TripleRatchetMessage`.
- **Calling `Bob.Encrypt` before receiving Alice's first message:** `TripleRatchetSession.Encrypt` returns `ErrSessionNotInitialized` on Bob's side before his DR ratchet is initialized. Bob must receive at least one message first.
- **Returning value instead of pointer from Session.Encrypt:** The facade must return `*TripleRatchetMessage`, not `TripleRatchetMessage`, even though the library returns a value.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| PQXDH key derivation | Custom DH1-DH4 + KEM + HKDF | `pqxdh.SendHandshake` / `ReceiveHandshake` | Library handles XEdDSA signature verification, DH4 conditional, kemSS zero-on-return, exact KDF input order |
| ML-KEM operations | Direct `mlkem.NewDecapsulationKey768` in session code | Wrap in `MLKEMProvider.Send()/Receive()` | SPQR layer requires Provider interface; raw KEM calls bypass rollback mechanism |
| Epoch management | Custom epoch counter with manual ratchet | SPQR layer's epoch system (Provider just supplies key material) | SPQR manages chain keys, skipped key storage, and epoch advancement — MLKEMProvider only produces/consumes KEM material |
| AD construction | Custom `IKA ‖ IKB ‖ PQPK` concatenation | `HandshakeResult.AD` (built by library) | Library's `buildAD` already implements PQXDH spec §4 correctly |

**Key insight:** The MLKEMProvider is purely a KEM key exchange primitive. It does NOT manage ratchet chains or epoch state — the SPQR layer does all of that. The Provider only needs to: (1) produce a message containing a new encapsulation key or KEM ciphertext, and (2) produce `outputKey` when a new epoch key has been established.

---

## Common Pitfalls

### Pitfall 1: KEMParams default is MLKEM1024, not MLKEM768
**What goes wrong:** `GenerateKEMSPK(ik, 1, pqxdh.MLKEM1024)` produces a 1568-byte encapsulation key instead of 1184 bytes. `ReceiveHandshake` will return `ErrUnsupportedKEMParams` or a params-mismatch error when Alice sends `PQParams: MLKEM768` but Bob's key has `Params: MLKEM1024`.
**Why it happens:** Library defaults suggest MLKEM1024 for libsignal compatibility. Our spec requires 768.
**How to avoid:** Pass `pqxdh.MLKEM768` to every `GenerateKEMSPK`, `GenerateKEMOPK`, and verify `PrekeyBundle.PQParams = pqxdh.MLKEM768`.
**Warning signs:** ReceiveHandshake error mentioning "params mismatch".

### Pitfall 2: Shallow copy in MLKEMProvider.Snapshot()
**What goes wrong:** After auth failure, SPQR calls `Restore(snap)`. If snapshot contains a shared slice reference, restoring it also restores a reference to the now-mutated data — the rollback is a no-op.
**Why it happens:** `snap.key = m.key` is idiomatic Go for value copy of scalars but incorrect for `[]byte`.
**How to avoid:** Always use `append([]byte(nil), src...)` for every `[]byte` field. For `[64]byte` seed, direct value assignment is safe (arrays copy by value in Go).
**Warning signs:** PQ-02 Snapshot test passes on first run but fails after mutation.

### Pitfall 3: Bob encrypts before receiving Alice's first message
**What goes wrong:** `TripleRatchetSession.Encrypt` returns `ErrSessionNotInitialized` on Bob's side before his EC DR ratchet is initialized.
**Why it happens:** Bob's DR component has `dhRSet=false` after `InitBobTripleRatchet` — he cannot compute a send chain key until he performs a DH ratchet step (triggered by receiving Alice's first message header).
**How to avoid:** In tests, always have Alice send first, then Bob.
**Warning signs:** `ErrSessionNotInitialized` on `bob.Encrypt()`.

### Pitfall 4: MLKEMProvider.Send() outputKey epoch must be exactly epoch+1
**What goes wrong:** SPQR validates `s.epoch+1 == keyEpoch` and returns `ErrEpochMismatch` if violated.
**Why it happens:** Provider returns wrong keyEpoch (e.g., 0 when it should be current+1).
**How to avoid:** Track current epoch in MLKEMProvider; increment before returning new outputKey.
**Warning signs:** `ErrEpochMismatch` from Triple Ratchet session operations.

### Pitfall 5: ReceiveHandshake requires *KEMPreKey, not *KEMOneTimePreKey
**What goes wrong:** Passing `&bobKEMOPK` directly causes a type error — the parameter is `*KEMPreKey`, not `*KEMOneTimePreKey`.
**Why it happens:** Library separates the public key type from the decapsulation key type.
**How to avoid:** Always call `.DecapsKey()`: `pqpk := bobKEMOPK.DecapsKey()`.
**Warning signs:** Go compile error on `ReceiveHandshake` argument type.

---

## Code Examples

### Full handshake wiring (verified against library source)

```go
// Source: verified against pqxdh/pqxdh.go, pqxdh/keys.go, pqxdh/kem.go, keys.go

// --- Bob side ---
bobIK, _ := pqxdh.GenerateIdentityKey()
bobSPK, _ := pqxdh.GenerateSPK(bobIK, 1)
bobOPK, _ := pqxdh.GenerateOPK(1)
bobKEMSPK, _ := pqxdh.GenerateKEMSPK(bobIK, 1, pqxdh.MLKEM768)
bobKEMOPK, _ := pqxdh.GenerateKEMOPK(bobIK, 1, pqxdh.MLKEM768)
bobDRPriv, bobDRPub, _ := doubleratchet.GenerateKeyPair()

bundle := &pqxdh.PrekeyBundle{
    IdentityKey:       bobIK.PublicKey,
    SignedPreKey:      bobSPK.PublicKey,
    SPKID:             bobSPK.KeyID,
    SPKSignature:      bobSPK.Signature,
    PQPreKey:          bobKEMOPK.EncapsulationKey, // use OPK not SPK (consumed once)
    PQPreKeyID:        bobKEMOPK.KeyID,
    PQPreKeySignature: bobKEMOPK.Signature,
    PQParams:          pqxdh.MLKEM768,
    OneTimePreKey:     &bobOPK.PublicKey,
    OPKID:             &bobOPK.KeyID,
}

// --- Alice side ---
aliceIK, _ := pqxdh.GenerateIdentityKey()
aliceResult, initMsg, _ := pqxdh.SendHandshake(aliceIK, bundle)

aliceSCKA := &MLKEMProvider{}
aliceTR, _ := doubleratchet.InitAliceTripleRatchet(aliceResult.RootKey[:], bobDRPub, aliceSCKA, nil)
aliceSess := &Session{RootKey: aliceResult.RootKey, ad: aliceResult.AD, tr: aliceTR}

// --- Bob receives InitialMessage ---
bobKEMOPKPreKey := bobKEMOPK.DecapsKey()
bobResult, _ := pqxdh.ReceiveHandshake(bobIK, &bobSPK, &bobOPK, bobKEMOPKPreKey, &initMsg)

bobSCKA := &MLKEMProvider{}
bobDRKP := doubleratchet.KeyPair{PrivateKey: bobDRPriv, PublicKey: bobDRPub}
bobTR, _ := doubleratchet.InitBobTripleRatchet(bobResult.RootKey[:], bobDRKP, bobSCKA, nil)
bobSess := &Session{RootKey: bobResult.RootKey, ad: bobResult.AD, tr: bobTR}
```

### MLKEMProvider skeleton

```go
// Source: pattern derived from scka/testing/mock.go + pqxdh/kem.go

type mlkemProviderSnapshot struct {
    decapSeed         [64]byte
    latestPeerEncapKey []byte
    sendEpoch         uint32
    recvEpoch         uint32
    initialized       bool
}

type MLKEMProvider struct {
    decapSeed         [64]byte // 64-byte FIPS 203 seed (d‖z)
    latestPeerEncapKey []byte   // encap key received from peer; nil until first Receive
    sendEpoch         uint32
    recvEpoch         uint32
    initialized       bool
}

func (p *MLKEMProvider) Snapshot() any {
    snap := &mlkemProviderSnapshot{
        decapSeed:   p.decapSeed, // [64]byte copies by value
        sendEpoch:   p.sendEpoch,
        recvEpoch:   p.recvEpoch,
        initialized: p.initialized,
    }
    if p.latestPeerEncapKey != nil {
        snap.latestPeerEncapKey = append([]byte(nil), p.latestPeerEncapKey...)
    }
    return snap
}

func (p *MLKEMProvider) Restore(snapshot any) {
    snap, ok := snapshot.(*mlkemProviderSnapshot)
    if !ok {
        return
    }
    p.decapSeed = snap.decapSeed
    p.latestPeerEncapKey = snap.latestPeerEncapKey
    p.sendEpoch = snap.sendEpoch
    p.recvEpoch = snap.recvEpoch
    p.initialized = snap.initialized
}
```

### PQ-04 wire-size assertion

```go
// Source: pattern from CONTEXT.md + pqxdh/kem.go size analysis
// ML-KEM-768 ciphertext: 1088 bytes raw → ~1452 bytes base64 in JSON
// Classical message.Ciphertext: ~40 bytes for short plaintext
func TestPQWireOverhead(t *testing.T) {
    // ... set up alice/bob classical and PQ sessions ...
    plaintext := []byte("hello")

    classicalMsg, _ := aliceClassical.Encrypt(plaintext)
    pqMsg, _ := alicePQ.Encrypt(plaintext)

    classicalJSON, _ := json.Marshal(classicalMsg)
    pqJSON, _ := json.Marshal(pqMsg)

    require.Greater(t, len(pqJSON), len(classicalJSON),
        "PQ message must be larger than classical due to KEM ciphertext overhead")
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| liboqs-go (CGo, Kyber) | `crypto/mlkem` (stdlib, FIPS 203) | Go 1.23 (Aug 2024) | Hermetic build, no CGo, standardized API |
| Kyber-1024 only (libsignal default) | ML-KEM-768 or ML-KEM-1024 configurable | go-doubleratchet v0.0.2 | `pqxdh.MLKEM768` constant selects 768 explicitly |
| `Encapsulate() (ss, ct)` order | `Encapsulate() (ss, ct)` | FIPS 203 final | Library's `kemRegistry` returns `(ct, ss)` in ops.encapsulate — note ordering |

**Note on Encapsulate return order:** [VERIFIED: pqxdh/kem.go lines 62-65]
The `kemRegistry.MLKEM768.encapsulate` function calls `ek.Encapsulate()` which returns `(ss, ct)` from stdlib, then the registry function returns `(ct, ss, nil)` — i.e., ciphertext first, shared secret second. `MLKEMProvider` does not call encapsulate directly (that is done by `pqxdh.SendHandshake`). For `MLKEMProvider.Send()`, the provider needs to generate a fresh keypair and emit the encapsulation key — not call encapsulate itself (the remote peer encapsulates against it).

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | ML-KEM-768 encapsulation key is 1184 bytes; ciphertext is 1088 bytes | Code Examples / PQ-04 | PQ-04 wire-size test still passes (any non-zero KEM overhead makes it pass); actual byte counts in Phase 6 README would differ |
| A2 | `MLKEMProvider.Send()` emits the new encapsulation key (not ciphertext) on first send, then alternates | Architecture diagram | If the epoch protocol is different, both sides could fail to agree on epoch transitions — requires integration test to validate |
| A3 | The `[64]byte` decapsulation seed copies by value in Go struct assignment | Pattern 6 Snapshot | If future Go compiler changes this (not expected), Snapshot would be shallow — low risk |

---

## Open Questions (RESOLVED)

1. **MLKEMProvider Send/Receive epoch protocol — exact sequencing** ✅ RESOLVED
   - **Decision:** Produce `outputKey` on every `Receive()` call that successfully decapsulates a peer encapsulation key (i.e., `latestPeerEncapKey != nil` at call time → decapsulate → `outputKey = shared_secret`, `keyEpoch = epoch+1`). Produce non-nil `outputKey` from `Send()` only when a newly generated keypair is being announced for the first time (i.e., on the message where the new `latestSelfEncapKey` is first included in `SCKAHeader.Msg`). Every subsequent `Send()` using the same keypair returns `outputKey = nil`. Keep `selfEpoch` and `peerEpoch` counters; SPQR enforces `keyEpoch == epoch+1`.
   - **Rationale:** This matches the SPQR contract: the receiver drives epoch advancement by decapsulating, the sender announces a new keypair once. An integration test (PQ-03) validates correctness.

2. **`internal/pq.PrekeyBundle` vs `pqxdh.PrekeyBundle` directly** ✅ RESOLVED
   - **Decision:** Use `pqxdh.PrekeyBundle` directly — no wrapper struct. `pqxdh.PrekeyBundle.OneTimePreKey` is `*[32]byte` and `OPKID` is `*uint32`; both JSON-serialize as nullable, which is correct for Phase 3.
   - **Rationale:** Per Claude's Discretion in CONTEXT.md; keeps `internal/pq` thin and avoids translation layers.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build | Yes | 1.25.0 (from go.mod) | — |
| go-doubleratchet | PQXDH + Triple Ratchet | Yes | v0.0.2 (pinned in go.mod) | — |
| crypto/mlkem | ML-KEM operations | Yes | stdlib Go 1.23+ | — |
| stretchr/testify | Tests | Yes | v1.11.1 (in go.mod) | — |

No missing dependencies.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./internal/pq/... -run TestPQ -v` |
| Full suite command | `go test ./... -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PQ-01 | PQXDH handshake produces matching RootKey on both sides | unit | `go test ./internal/pq/... -run TestPQSession -v` | Wave 0 |
| PQ-02 | `MLKEMProvider.Snapshot()` deep-copies all state | unit | `go test ./internal/pq/... -run TestMLKEMProviderSnapshot -v` | Wave 0 |
| PQ-03 | `bob.Decrypt(alice.Encrypt(plaintext)) == plaintext` | unit | `go test ./internal/pq/... -run TestPQSession -v` | Wave 0 |
| PQ-04 | `len(json.Marshal(pqMsg)) > len(json.Marshal(classicalMsg))` | unit | `go test ./internal/pq/... -run TestPQWireOverhead -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/pq/... -v`
- **Per wave merge:** `go test ./... -v`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/pq/pq.go` — package scaffold
- [ ] `internal/pq/provider.go` — MLKEMProvider scaffold
- [ ] `internal/pq/pq_test.go` — test file with PQ-01 through PQ-04 test stubs

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A (no user auth in this phase) |
| V3 Session Management | yes | Key zeroization via `Close()`, Snapshot/Restore rollback on auth failure |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes | `ecutil.ValidatePublicKey` in PQXDH, params mismatch guard in ReceiveHandshake |
| V6 Cryptography | yes | stdlib `crypto/mlkem`, no hand-rolled KEM; key material zeroed on Close() |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Low-order point injection (X25519) | Spoofing | `ecutil.ValidatePublicKey` called by library on all received EC keys |
| KEM parameter downgrade | Tampering | `ReceiveHandshake` validates `pqpk.Params == msg.PQParams` |
| Session state corruption on failed decrypt | Tampering | SPQR calls `Snapshot()` before decrypt; `Restore()` on auth failure |
| Key material exposure via shallow copy | Information Disclosure | Deep copy in `Snapshot()` using `append([]byte(nil), ...)` |
| Key material persistence after session end | Information Disclosure | `Close()` zeros decapSeed and latestPeerEncapKey before nil-ing |

---

## Sources

### Primary (HIGH confidence)
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/pqxdh/pqxdh.go` — exact `SendHandshake`/`ReceiveHandshake` signatures, `HandshakeResult` fields, `PrekeyBundle` fields, AD construction
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/pqxdh/keys.go` — `IdentityKey`, `SignedPreKey`, `OneTimePreKey`, `GenerateOPK`, `GenerateSPK`, `GenerateIdentityKey` signatures
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/pqxdh/kem.go` — `KEMParams` constants, `KEMSignedPreKey`, `KEMOneTimePreKey`, `KEMPreKey`, `DecapsKey()`, `GenerateKEMSPK`, `GenerateKEMOPK`, `kemRegistry` with ML-KEM-768/1024
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/session_tr.go` — `InitInitiatorTripleRatchet`/`InitResponderTripleRatchet` signatures, `Encrypt`/`Decrypt` return types (value, not pointer)
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/keys.go` — `InitAliceTripleRatchet`/`InitBobTripleRatchet` aliases, `KeyPair` type alias, `GenerateKeyPair`
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/scka/scka.go` — complete `Provider` interface with all method signatures
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/scka/testing/mock.go` — `Snapshot()`/`Restore()` deep copy pattern
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/message.go` — `TripleRatchetMessage`, `TripleRatchetHeader`, `SCKAHeader` types
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/session_spqr.go` — how SPQR calls `scka.Provider` methods, `sendKey()`/`receiveKey()` internals
- [VERIFIED: library source] `~/go/pkg/mod/github.com/!kushneryk!pavel/go-doubleratchet@v0.0.2/session_tr_test.go` — minimal test setup pattern (`newTRPair`), usage of `crypto.KeyPair`
- [VERIFIED: project source] `internal/classical/classical.go` — Session facade pattern to mirror exactly
- [VERIFIED: project file] `go.mod` — Go 1.25.0, go-doubleratchet v0.0.2 pinned, testify v1.11.1 present

### Secondary (MEDIUM confidence)
- [ASSUMED] ML-KEM-768 specific byte sizes (1184 byte encap key, 1088 byte ciphertext) — derived from NIST FIPS 203 specification and Go stdlib source, not verified via running code in this session

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified from go.mod and library source
- Architecture: HIGH — all API signatures read directly from library source code
- Pitfalls: HIGH — most derived from reading actual error paths in library source; A1/A2/A3 in Assumptions Log are the only uncertain items
- MLKEMProvider epoch protocol: MEDIUM — the sequencing of when `outputKey` is non-nil requires integration testing to confirm (Open Question 1)

**Research date:** 2026-04-27
**Valid until:** 2026-07-27 (go-doubleratchet v0.0.2 pinned; stable until dependency is updated)
