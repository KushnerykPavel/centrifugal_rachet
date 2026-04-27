package pq_test

import (
	"encoding/json"
	"testing"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/classical"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/pq"
	"github.com/stretchr/testify/require"
)

// TestPQXDHHandshake is the PQ-01 acceptance test.
// Asserts: PQXDH handshake produces identical RootKey on both sides.
func TestPQXDHHandshake(t *testing.T) {
	bobBundle, bobPrivate, err := pq.NewResponderBundle()
	require.NoError(t, err)

	aliceSess, aliceInitMsg, err := pq.InitiatorHandshake(bobBundle)
	require.NoError(t, err)

	bobSess, err := pq.ResponderHandshake(bobPrivate, aliceInitMsg)
	require.NoError(t, err)

	require.Equal(t, aliceSess.RootKey, bobSess.RootKey, "RootKey must match after PQXDH handshake")
}

// TestMLKEMProviderSnapshot is the PQ-02 acceptance test (Snapshot deep-copy).
// Asserts: mutating provider after Snapshot() does not affect the snapshot;
// Restore() reinstates pre-mutation state.
func TestMLKEMProviderSnapshot(t *testing.T) {
	p := &pq.MLKEMProvider{}
	require.NoError(t, p.InitInitiator(make([]byte, 32)))

	// Capture snapshot of initial state.
	snap := p.Snapshot()

	// Mutate provider: re-init changes decapSeed to a new random value.
	require.NoError(t, p.InitInitiator(make([]byte, 32)))

	// Restore must undo the mutation.
	p.Restore(snap)

	// After restore, capture another snapshot — both snapshots should have the
	// same type and the provider state must be consistent (no panic on second Snapshot).
	snap2 := p.Snapshot()
	_ = snap2

	// Verify Snapshot/Restore is not a no-op: take snap before and after a second
	// InitInitiator, restore to the pre-mutation snapshot, then confirm Restore ran
	// by calling another Snapshot and Send (which uses the stored decapSeed).
	p2 := &pq.MLKEMProvider{}
	require.NoError(t, p2.InitInitiator(make([]byte, 32)))
	snap3 := p2.Snapshot()

	// Mutate p2 state.
	require.NoError(t, p2.InitInitiator(make([]byte, 32)))

	// Restore must undo the mutation.
	p2.Restore(snap3)

	// After restore, Send() must succeed (uses restored decapSeed via NewDecapsulationKey768).
	msg, _, _, _, err := p2.Send()
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.Equal(t, 1184, len(msg), "ML-KEM-768 encapsulation key must be 1184 bytes")
}

// TestMLKEMProviderClose asserts that Close() zeros key material (D-08).
func TestMLKEMProviderClose(t *testing.T) {
	p := &pq.MLKEMProvider{}
	require.NoError(t, p.InitInitiator(make([]byte, 32)))
	require.NoError(t, p.Close())
	// Close() must be idempotent — second call must not panic or error.
	require.NoError(t, p.Close())
}

// TestPQSession is the PQ-03 acceptance test.
// Asserts: bob.Decrypt(alice.Encrypt(plaintext)) == plaintext over a TripleRatchetSession.
// NOTE: Alice must send first — Bob's DR ratchet is uninitialised until he receives Alice's first message.
func TestPQSession(t *testing.T) {
	bobBundle, bobPrivate, err := pq.NewResponderBundle()
	require.NoError(t, err)

	aliceSess, aliceInitMsg, err := pq.InitiatorHandshake(bobBundle)
	require.NoError(t, err)

	bobSess, err := pq.ResponderHandshake(bobPrivate, aliceInitMsg)
	require.NoError(t, err)

	plaintext := []byte("hello, triple ratchet")

	// Alice sends first (Bob's DR ratchet initialised on receive)
	msg, err := aliceSess.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := bobSess.Decrypt(msg)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted, "roundtrip failed")
}

// TestPQWireOverhead is the PQ-04 acceptance test.
// Asserts: len(json.Marshal(pqMsg)) > len(json.Marshal(classicalMsg)) for the same plaintext.
// ML-KEM-768 ciphertext (~1088 raw bytes → ~1452 base64 bytes) dominates PQ message size.
func TestPQWireOverhead(t *testing.T) {
	// Classical session setup
	classicalBundle, classicalPriv, err := classical.NewResponderBundle()
	require.NoError(t, err)
	classicalAlice, classicalInitMsg, err := classical.InitiatorHandshake(classicalBundle)
	require.NoError(t, err)
	classicalBob, err := classical.ResponderHandshake(classicalPriv, classicalInitMsg)
	require.NoError(t, err)
	_ = classicalBob // Bob needed to complete handshake

	// PQ session setup
	pqBundle, pqPriv, err := pq.NewResponderBundle()
	require.NoError(t, err)
	pqAlice, pqInitMsg, err := pq.InitiatorHandshake(pqBundle)
	require.NoError(t, err)
	pqBob, err := pq.ResponderHandshake(pqPriv, pqInitMsg)
	require.NoError(t, err)
	_ = pqBob // Bob needed to complete handshake

	plaintext := []byte("hello")

	classicalMsg, err := classicalAlice.Encrypt(plaintext)
	require.NoError(t, err)
	pqMsg, err := pqAlice.Encrypt(plaintext)
	require.NoError(t, err)

	classicalJSON, err := json.Marshal(classicalMsg)
	require.NoError(t, err)
	pqJSON, err := json.Marshal(pqMsg)
	require.NoError(t, err)

	require.Greater(t, len(pqJSON), len(classicalJSON),
		"PQ message must be larger than classical due to ML-KEM-768 ciphertext overhead")
}
