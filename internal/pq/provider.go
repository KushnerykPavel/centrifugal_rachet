package pq

import (
	"crypto/mlkem"
	"crypto/rand"
	"fmt"
)

const (
	mlkem768EncapKeySize   = 1184 // ML-KEM-768 encapsulation key size (FIPS 203)
	mlkem768CiphertextSize = 1088 // ML-KEM-768 ciphertext size (FIPS 203)
)

// mlkemProviderSnapshot holds a deep copy of MLKEMProvider state for rollback (D-07).
// Used by SPQR layer to restore state on auth failure.
type mlkemProviderSnapshot struct {
	decapSeed             [64]byte // safe to copy by value — Go arrays are value types
	latestPeerEncapKey    []byte   // deep-copied in Snapshot()
	pendingPeerCiphertext []byte   // deep-copied in Snapshot()
	sendEpoch             uint32
	recvEpoch             uint32
	initialized           bool
}

// MLKEMProvider implements scka.Provider using ML-KEM-768 (D-05, D-06).
// It implements the two-round KEM protocol: the announcer emits an encapsulation key
// (Round 1), the encapsulator returns a ciphertext (Round 2), and both peers derive
// the same 32-byte shared secret — the announcer via Decapsulate, the encapsulator
// via Encapsulate. This ensures ML-KEM-768 contributes genuine shared entropy to the
// Triple Ratchet KEM epoch.
type MLKEMProvider struct {
	decapSeed             [64]byte // 64-byte FIPS 203 seed (d‖z) for current decapsulation key
	latestPeerEncapKey    []byte   // encap key received from peer; nil until first Receive(encapKey)
	pendingPeerCiphertext []byte   // ciphertext received from peer (reserved for future use)
	sendEpoch             uint32
	recvEpoch             uint32
	initialized           bool
}

// InitInitiator initialises the provider as the Triple Ratchet initiator (Alice).
// Generates a fresh random decapsulation key seed.
func (p *MLKEMProvider) InitInitiator(sk []byte) error {
	if _, err := rand.Read(p.decapSeed[:]); err != nil {
		return fmt.Errorf("pq: MLKEMProvider.InitInitiator: %w", err)
	}
	p.sendEpoch = 0
	p.recvEpoch = 0
	p.initialized = true
	return nil
}

// InitResponder initialises the provider as the Triple Ratchet responder (Bob).
// Symmetric with InitInitiator per D-06.
func (p *MLKEMProvider) InitResponder(sk []byte) error {
	if _, err := rand.Read(p.decapSeed[:]); err != nil {
		return fmt.Errorf("pq: MLKEMProvider.InitResponder: %w", err)
	}
	p.sendEpoch = 0
	p.recvEpoch = 0
	p.initialized = true
	return nil
}

// Send produces the KEM message to include in SCKAHeader.Msg (D-05, D-06).
//
// Two-round protocol (D-06):
//
//   - Encapsulator path: if latestPeerEncapKey is set (peer previously sent their encap key),
//     encapsulate against it — emit the ciphertext (1088 bytes) as msg and return the shared
//     secret as outputKey. The peer will recover the same secret via Decapsulate.
//
//   - Announcer path: generate a fresh [64]byte seed, derive the decapsulation key, store the
//     seed, and emit the encapsulation key bytes (1184 bytes) as msg. No outputKey yet — the
//     shared secret is produced when the peer returns the ciphertext in a subsequent Receive().
func (p *MLKEMProvider) Send() (msg []byte, sendingEpoch uint32, outputKey []byte, keyEpoch uint32, err error) {
	sendingEpoch = p.sendEpoch
	p.sendEpoch++

	// Encapsulator path: peer sent us their encap key in a prior Receive().
	// We encapsulate against it and return the ciphertext as msg.
	// The peer will call Decapsulate(ct) to recover the same shared secret.
	if p.latestPeerEncapKey != nil {
		ek, parseErr := mlkem.NewEncapsulationKey768(p.latestPeerEncapKey)
		if parseErr != nil {
			return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: NewEncapsulationKey768: %w", parseErr)
		}
		ss, ct := ek.Encapsulate()
		p.latestPeerEncapKey = nil // consumed — clear to avoid re-use
		outputKey = ss[:32]
		keyEpoch = p.recvEpoch + 1
		return ct, sendingEpoch, outputKey, keyEpoch, nil
	}

	// Announcer path: emit a fresh encapsulation key for the peer to encapsulate against.
	// Store the seed so we can reconstruct the decapsulation key when the peer returns ct.
	var newSeed [64]byte
	if _, err = rand.Read(newSeed[:]); err != nil {
		return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: generate seed: %w", err)
	}
	dk, err := mlkem.NewDecapsulationKey768(newSeed[:])
	if err != nil {
		return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: NewDecapsulationKey768: %w", err)
	}
	p.decapSeed = newSeed
	msg = dk.EncapsulationKey().Bytes() // 1184 bytes
	return msg, sendingEpoch, nil, 0, nil
}

// Receive processes the KEM message from SCKAHeader.Msg (D-05, D-06).
//
// Dispatch is purely by message length:
//   - len(msg) == 1184 (mlkem768EncapKeySize): peer sent their encapsulation key (Round 1).
//     Store it for our next Send() to encapsulate against. No outputKey yet.
//   - len(msg) == 1088 (mlkem768CiphertextSize): peer sent the ciphertext (Round 2).
//     Reconstruct the decapsulation key from p.decapSeed and call Decapsulate to recover
//     the shared secret. Return outputKey = ss[:32].
//   - any other length: return a descriptive error (T-02gc-03).
func (p *MLKEMProvider) Receive(msg []byte) (receivingEpoch uint32, outputKey []byte, keyEpoch uint32, err error) {
	switch len(msg) {
	case mlkem768EncapKeySize:
		// Peer sent their encap key (Round 1). Store it so our next Send() encapsulates against it.
		p.latestPeerEncapKey = append([]byte(nil), msg...) // deep copy
		receivingEpoch = p.recvEpoch
		// No outputKey yet — shared secret is produced when we encapsulate in Send().
		return receivingEpoch, nil, 0, nil

	case mlkem768CiphertextSize:
		// Peer sent the ciphertext (Round 2). Decapsulate using our stored seed to recover ss.
		dk, parseErr := mlkem.NewDecapsulationKey768(p.decapSeed[:])
		if parseErr != nil {
			return 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Receive: NewDecapsulationKey768: %w", parseErr)
		}
		ss, decErr := dk.Decapsulate(msg)
		if decErr != nil {
			return 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Receive: Decapsulate: %w", decErr)
		}
		receivingEpoch = p.recvEpoch
		p.recvEpoch++
		outputKey = ss[:32]
		keyEpoch = p.recvEpoch // == old recvEpoch + 1

		return receivingEpoch, outputKey, keyEpoch, nil

	default:
		return 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Receive: unexpected message length %d (want %d or %d)",
			len(msg), mlkem768EncapKeySize, mlkem768CiphertextSize)
	}
}

// Snapshot returns a deep copy of all mutable provider state (D-07).
// [64]byte arrays copy by value; []byte slices require append([]byte(nil), ...).
func (p *MLKEMProvider) Snapshot() any {
	snap := &mlkemProviderSnapshot{
		decapSeed:   p.decapSeed, // [64]byte — Go array copies by value
		sendEpoch:   p.sendEpoch,
		recvEpoch:   p.recvEpoch,
		initialized: p.initialized,
	}
	if p.latestPeerEncapKey != nil {
		snap.latestPeerEncapKey = append([]byte(nil), p.latestPeerEncapKey...)
	}
	if p.pendingPeerCiphertext != nil {
		snap.pendingPeerCiphertext = append([]byte(nil), p.pendingPeerCiphertext...)
	}
	return snap
}

// Restore reinstates provider state from a snapshot produced by Snapshot() (D-07).
func (p *MLKEMProvider) Restore(snapshot any) {
	snap, ok := snapshot.(*mlkemProviderSnapshot)
	if !ok {
		return
	}
	p.decapSeed = snap.decapSeed
	p.latestPeerEncapKey = snap.latestPeerEncapKey
	p.pendingPeerCiphertext = snap.pendingPeerCiphertext
	p.sendEpoch = snap.sendEpoch
	p.recvEpoch = snap.recvEpoch
	p.initialized = snap.initialized
}

// Close zeros all key material before releasing resources (D-08, T-02gc-02).
func (p *MLKEMProvider) Close() error {
	for i := range p.decapSeed {
		p.decapSeed[i] = 0
	}
	for i := range p.latestPeerEncapKey {
		p.latestPeerEncapKey[i] = 0
	}
	p.latestPeerEncapKey = nil
	for i := range p.pendingPeerCiphertext {
		p.pendingPeerCiphertext[i] = 0
	}
	p.pendingPeerCiphertext = nil
	return nil
}
