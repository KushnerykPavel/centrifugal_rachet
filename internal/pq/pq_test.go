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
	t.Skip("stub — implemented in Plan 03")

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
	t.Skip("stub — implemented in Plan 02")

	p := &pq.MLKEMProvider{}
	require.NoError(t, p.InitInitiator(make([]byte, 32)))

	snap := p.Snapshot()
	// mutation and restore assertions added in Plan 02
	p.Restore(snap)
}

// TestMLKEMProviderClose asserts that Close() zeros key material (D-08).
func TestMLKEMProviderClose(t *testing.T) {
	t.Skip("stub — implemented in Plan 02")

	p := &pq.MLKEMProvider{}
	require.NoError(t, p.InitInitiator(make([]byte, 32)))
	require.NoError(t, p.Close())
}

// TestPQSession is the PQ-03 acceptance test.
// Asserts: bob.Decrypt(alice.Encrypt(plaintext)) == plaintext over a TripleRatchetSession.
// NOTE: Alice must send first — Bob's DR ratchet is uninitialised until he receives Alice's first message.
func TestPQSession(t *testing.T) {
	t.Skip("stub — implemented in Plan 03")

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
	t.Skip("stub — implemented in Plan 03")

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
