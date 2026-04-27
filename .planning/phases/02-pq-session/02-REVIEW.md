---
phase: 02-pq-session
reviewed: 2026-04-27T12:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - internal/pq/provider.go
  - internal/pq/pq.go
  - internal/pq/pq_test.go
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 02: Code Review Report (Gap-Closure)

**Reviewed:** 2026-04-27T12:00:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

This is a gap-closure review against the prior report, which raised CR-01 (KEM ciphertext discarded) and CR-02 (Decapsulate never called). Both critical issues are resolved in the current code. The encapsulator path in `Send()` now correctly returns the ciphertext as `msg` and the shared secret as `outputKey`; the announcer path in `Receive()` now dispatches on message length and calls `dk.Decapsulate(msg)` to recover the shared secret from the peer's ciphertext. The dead `drPriv`/`drPub` fields (WR-01) have also been removed from `ResponderKeys`.

Two info-level items from the prior report remain open (WR-02 and IN-01, now downgraded to Info given no correctness impact), and one new Warning is raised for dead struct state (`pendingPeerCiphertext`).

---

## Warnings

### WR-01: `pendingPeerCiphertext` field is dead state — stored, snapshotted, zeroed, never written

**File:** `internal/pq/provider.go:34`

**Issue:** `pendingPeerCiphertext []byte` is declared in both `MLKEMProvider` and `mlkemProviderSnapshot`, deep-copied in `Snapshot()` (line 162), reinstated in `Restore()` (line 176), and zeroed in `Close()` (lines 192-194). However it is never written anywhere in the current code. The two-round protocol now routes the ciphertext through the `msg` return value of `Send()`, so the field has no purpose. Carrying dead key-material-shaped state through the snapshot/restore lifecycle is misleading and inflates the attack surface of `Close()` (a reviewer must check whether it could hold live secrets).

**Fix:** Remove `pendingPeerCiphertext` from `MLKEMProvider`, `mlkemProviderSnapshot`, `Snapshot()`, `Restore()`, and `Close()`:
```go
// Delete from MLKEMProvider struct:
// pendingPeerCiphertext []byte   ← remove

// Delete from mlkemProviderSnapshot:
// pendingPeerCiphertext []byte   ← remove

// Delete from Snapshot():
// if p.pendingPeerCiphertext != nil {
//     snap.pendingPeerCiphertext = append([]byte(nil), p.pendingPeerCiphertext...)
// }

// Delete from Restore():
// p.pendingPeerCiphertext = snap.pendingPeerCiphertext

// Delete from Close():
// for i := range p.pendingPeerCiphertext { p.pendingPeerCiphertext[i] = 0 }
// p.pendingPeerCiphertext = nil
```

---

## Info

### IN-01: `InitInitiator` and `InitResponder` silently ignore the `sk` parameter

**File:** `internal/pq/provider.go:42`, `internal/pq/provider.go:54`

**Issue:** Both `InitInitiator(sk []byte)` and `InitResponder(sk []byte)` accept a session-key seed parameter but never read it — `p.decapSeed` is always overwritten with fresh random bytes from `rand.Read`. The `sk` argument is silently discarded. Callers that expect deterministic seeding will be surprised, and static analysis tools will flag the unused parameter.

**Fix:** If `sk` is intentionally unused (pure ephemeral KEM), use a blank identifier to document the intent:
```go
func (p *MLKEMProvider) InitInitiator(_ []byte) error {
```
If `sk` is intended to seed the initial decapsulation key deterministically:
```go
func (p *MLKEMProvider) InitInitiator(sk []byte) error {
    if len(sk) < 64 {
        return fmt.Errorf("pq: MLKEMProvider.InitInitiator: sk must be at least 64 bytes, got %d", len(sk))
    }
    copy(p.decapSeed[:], sk[:64])
    p.sendEpoch = 0
    p.recvEpoch = 0
    p.initialized = true
    return nil
}
```

### IN-02: `Restore()` silently no-ops on type mismatch

**File:** `internal/pq/provider.go:169-172`

**Issue:** If `snapshot` is not `*mlkemProviderSnapshot`, `Restore()` returns without modifying provider state and without signalling an error. A wrong-type snapshot passed during a D-07 rollback will silently fail to restore state, leaving the provider in a post-failure state while the caller believes it has been rolled back.

**Fix:** Panic with a descriptive message to surface the logic error during development (the method signature cannot return an error without changing the interface):
```go
func (p *MLKEMProvider) Restore(snapshot any) {
    snap, ok := snapshot.(*mlkemProviderSnapshot)
    if !ok {
        panic(fmt.Sprintf("pq: MLKEMProvider.Restore: unexpected snapshot type %T", snapshot))
    }
    // ... rest unchanged
}
```

---

## Prior Critical Issues — Verified Fixed

### CR-01 (resolved): KEM ciphertext is now transmitted

The prior report found `ek.Encapsulate()` results discarded with `_` in both `Send()` and `Receive()`. In the current code, `Send()` (encapsulator path, line 87) captures `ss, ct := ek.Encapsulate()` and returns `ct` as `msg` (line 91). The ciphertext is now transmitted to the peer.

### CR-02 (resolved): Decapsulate is now called

The prior report found `Decapsulate` was never called anywhere. In the current code, `Receive()` dispatches on `len(msg) == mlkem768CiphertextSize` (line 127) and calls `dk.Decapsulate(msg)` (line 133) to recover the shared secret from the peer's ciphertext.

### WR-01 (resolved): Dead `drPriv`/`drPub` fields removed

The prior report found `drPriv`/`drPub` generated in `NewResponderBundle()` but never used. These fields are absent from the current `ResponderKeys` struct.

---

_Reviewed: 2026-04-27T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
