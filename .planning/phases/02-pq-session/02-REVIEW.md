---
phase: 02-pq-session
reviewed: 2026-04-27T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - internal/pq/pq.go
  - internal/pq/provider.go
  - internal/pq/pq_test.go
findings:
  critical: 2
  warning: 2
  info: 1
  total: 5
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-04-27
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

Three files implement the PQ session layer: a handshake facade (`pq.go`), an ML-KEM-768 SCKA provider (`provider.go`), and acceptance tests (`pq_test.go`). The handshake facade and tests are structurally sound. The critical problems are concentrated in `provider.go`, where the ML-KEM encapsulation protocol is architecturally broken: both peers independently call `Encapsulate()` (generating fresh, unshared randomness each time) rather than using the correct `Encapsulate()`/`Decapsulate()` pairing. The ciphertext produced by `Encapsulate()` is silently discarded with `_`, so the peer can never derive the same shared secret. This means the KEM ratchet layer produces no real shared entropy between the two sides. In addition, `pq.go` generates a DR keypair (`drPriv`/`drPub`) that is never used.

---

## Critical Issues

### CR-01: KEM ciphertext silently discarded — peers cannot share a secret

**File:** `internal/pq/provider.go:85` and `internal/pq/provider.go:112`

**Issue:** `ek.Encapsulate()` returns `(sharedKey, ciphertext)`. Both `Send()` and `Receive()` discard the ciphertext with `_`. In ML-KEM, only the sender calls `Encapsulate()` — the resulting `ciphertext` must be transmitted to the holder of the matching `DecapsulationKey`, who calls `dk.Decapsulate(ciphertext)` to recover the same `sharedKey`. Without transmitting the ciphertext, both sides independently generate unrelated random shared keys. The KEM ratchet contributes no real shared entropy.

The two affected lines:
```go
// provider.go:85
ss, _ := ek.Encapsulate() // ciphertext DROPPED — peer can never derive ss

// provider.go:112
ss, _ := ek.Encapsulate() // same defect in Receive()
```

**Fix:** The protocol must be restructured to the standard Diffie-Hellman-style KEM flow:

- **Sender side (`Send()`):** The sender already advertises their fresh encapsulation key (`dk.EncapsulationKey().Bytes()`) so the peer can encapsulate against it. The sender does NOT call `Encapsulate()` here. Instead, the sender decapsulates the ciphertext that arrives in the peer's next `SCKAHeader.Msg` (see CR-02).
- **Receiver side (`Receive()`):** On receiving the peer's encapsulation key, call `ek.Encapsulate()` to produce `(sharedKey, ciphertext)`, return `ciphertext` in `SCKAHeader.Msg` alongside (or instead of) the raw encap key, and use `sharedKey` as `outputKey`.
- **Decapsulation (`Send()` or a new `Recv()` path):** When the peer's encapsulated ciphertext arrives, reconstruct the decapsulation key from `p.decapSeed` and call `dk.Decapsulate(ciphertext)` to recover the matching `sharedKey`.

Minimal corrected sketch for `Receive()`:
```go
func (p *MLKEMProvider) Receive(msg []byte) (receivingEpoch uint32, outputKey []byte, keyEpoch uint32, err error) {
    if len(msg) == 0 {
        return 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Receive: empty message")
    }
    ek, parseErr := mlkem.NewEncapsulationKey768(msg)
    if parseErr != nil {
        return 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Receive: NewEncapsulationKey768: %w", parseErr)
    }
    sharedKey, ciphertext := ek.Encapsulate()
    // ciphertext must be returned and transmitted back to the sender
    // so they can call dk.Decapsulate(ciphertext) to recover sharedKey.
    p.pendingCiphertext = ciphertext   // new field, sent in next SCKAHeader.Msg
    outputKey = sharedKey[:32]
    receivingEpoch = p.recvEpoch
    p.recvEpoch++
    keyEpoch = p.recvEpoch
    return receivingEpoch, outputKey, keyEpoch, nil
}
```

### CR-02: `decapSeed` stored but decapsulation (`dk.Decapsulate`) is never called

**File:** `internal/pq/provider.go:66-70`

**Issue:** `NewDecapsulationKey768(newSeed[:])` is called in `Send()` solely to derive and broadcast the encapsulation key bytes. The `decapSeed` is saved to `p.decapSeed`, presumably so that the decapsulation key can be reconstructed later. However, nowhere in the codebase is `mlkem.NewDecapsulationKey768(p.decapSeed[:])` followed by `dk.Decapsulate(ciphertext)` ever called. The `decapSeed` persists across `Snapshot()`/`Restore()` cycles but is permanently unused — the decapsulation half of the KEM is dead code.

```go
// Send() — dk is only used for EncapsulationKey().Bytes(), then discarded:
dk, err := mlkem.NewDecapsulationKey768(newSeed[:])
// ...
msg = dk.EncapsulationKey().Bytes()
// p.decapSeed = newSeed  ← stored but never used for Decapsulate()
```

**Fix:** Add decapsulation in `Send()` (or a dedicated path) when the peer's ciphertext is available. Once CR-01 is fixed and `Receive()` transmits a ciphertext back, `Send()` should:
```go
// Reconstruct our current decapsulation key and decapsulate the peer's ciphertext.
if p.pendingPeerCiphertext != nil {
    dk, err := mlkem.NewDecapsulationKey768(p.decapSeed[:])
    if err != nil { ... }
    sharedKey, err := dk.Decapsulate(p.pendingPeerCiphertext)
    if err != nil { ... }
    outputKey = sharedKey[:32]
    p.pendingPeerCiphertext = nil
}
```

---

## Warnings

### WR-01: Dead key material — `drPriv`/`drPub` generated but never used

**File:** `internal/pq/pq.go:26-27`, `63-66`, `86-87`

**Issue:** `NewResponderBundle()` generates a dedicated DR keypair (`drPriv`, `drPub`) via `doubleratchet.GenerateKeyPair()` and stores it in `ResponderKeys`. The comment on line 27 says "Bob's DR public key (included in PrekeyBundle)", but neither field is placed in `PrekeyBundle` nor read in `ResponderHandshake()`. Instead, `ResponderHandshake()` reuses `priv.spk` (the PQXDH SignedPreKey) directly as the DR keypair (line 130). The generated `drPriv`/`drPub` material is wasted entropy and the comment is misleading.

**Fix:** Either remove the `drPriv`/`drPub` fields and the `GenerateKeyPair()` call, or use them as intended by placing `drPub` in the `PrekeyBundle` and using `drPriv`/`drPub` in `ResponderHandshake()` instead of reusing `spk`:
```go
// Option A — remove dead fields:
// Delete drPriv, drPub from ResponderKeys; delete GenerateKeyPair() call.

// Option B — use them:
bundle.DRPublicKey = priv.drPub  // add to PrekeyBundle if schema supports it
// In ResponderHandshake:
bobDRKP := doubleratchet.KeyPair{PrivateKey: priv.drPriv, PublicKey: priv.drPub}
```

### WR-02: `InitInitiator` and `InitResponder` silently ignore the `sk` parameter

**File:** `internal/pq/provider.go:32-52`

**Issue:** Both `InitInitiator(sk []byte)` and `InitResponder(sk []byte)` accept a session-key seed parameter but never use it — the `sk` argument is ignored. Both methods overwrite `p.decapSeed` with fresh random bytes regardless of `sk`. If the caller expects the provider to derive its initial KEM keypair deterministically from `sk` (e.g., for test reproducibility or session binding), this is silently wrong. The test passes `make([]byte, 32)` (all-zeros) but the provider still generates a random seed.

**Fix:** If `sk` is intentionally unused (pure ephemeral KEM), rename or remove the parameter to avoid confusion:
```go
func (p *MLKEMProvider) InitInitiator(_ []byte) error {
```
If `sk` is meant to seed the initial decapsulation key deterministically, use it:
```go
func (p *MLKEMProvider) InitInitiator(sk []byte) error {
    if len(sk) < 64 {
        return fmt.Errorf("pq: MLKEMProvider.InitInitiator: sk must be 64 bytes, got %d", len(sk))
    }
    copy(p.decapSeed[:], sk[:64])
    // ...
}
```

---

## Info

### IN-01: `Restore()` silently no-ops on type mismatch

**File:** `internal/pq/provider.go:138-148`

**Issue:** If `snapshot` is not `*mlkemProviderSnapshot`, `Restore()` returns without error and leaves provider state unchanged. This makes misuse invisible to callers — a wrong-type snapshot silently fails to restore state, which could cause the SPQR rollback (D-07) to proceed with corrupted state.

**Fix:** Return an error (requires changing the method signature to match the interface) or at minimum panic with a descriptive message to surface misuse during development:
```go
func (p *MLKEMProvider) Restore(snapshot any) {
    snap, ok := snapshot.(*mlkemProviderSnapshot)
    if !ok {
        panic(fmt.Sprintf("pq: MLKEMProvider.Restore: unexpected snapshot type %T", snapshot))
    }
    // ...
}
```
If the interface requires `Restore(any)` with no return value, the panic approach is appropriate for a logic-error guard.

---

_Reviewed: 2026-04-27_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
