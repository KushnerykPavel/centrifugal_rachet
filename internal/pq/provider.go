package pq

import (
	"crypto/mlkem"
	"crypto/rand"
	"fmt"
)

// mlkemProviderSnapshot holds a deep copy of MLKEMProvider state for rollback (D-07).
// Used by SPQR layer to restore state on auth failure.
type mlkemProviderSnapshot struct {
	decapSeed          [64]byte // safe to copy by value — Go arrays are value types
	latestPeerEncapKey []byte   // deep-copied in Snapshot()
	sendEpoch          uint32
	recvEpoch          uint32
	initialized        bool
}

// MLKEMProvider implements scka.Provider using ML-KEM-768 (D-05, D-06).
// It rotates the ML-KEM keypair every message — Send() always emits a fresh
// encapsulation key so every metric sample in Phase 4 captures a full KEM operation.
type MLKEMProvider struct {
	decapSeed          [64]byte // 64-byte FIPS 203 seed (d‖z) for current decapsulation key
	latestPeerEncapKey []byte   // encap key received from peer; nil until first Receive()
	sendEpoch          uint32
	recvEpoch          uint32
	initialized        bool
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
// Protocol: generates a fresh ML-KEM-768 decapsulation key from a new random seed,
// stores the seed, and emits the encapsulation key bytes as msg (1184 bytes).
// If latestPeerEncapKey is set from a prior Receive(), encapsulates against it
// to produce outputKey and keyEpoch for the SPQR KDF ratchet step.
func (p *MLKEMProvider) Send() (msg []byte, sendingEpoch uint32, outputKey []byte, keyEpoch uint32, err error) {
	// Generate a fresh decapsulation key for the peer to encapsulate against next time.
	var newSeed [64]byte
	if _, err = rand.Read(newSeed[:]); err != nil {
		return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: generate seed: %w", err)
	}
	dk, err := mlkem.NewDecapsulationKey768(newSeed[:])
	if err != nil {
		return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: NewDecapsulationKey768: %w", err)
	}
	p.decapSeed = newSeed

	// Emit our new encapsulation key for the peer.
	msg = dk.EncapsulationKey().Bytes()

	sendingEpoch = p.sendEpoch
	p.sendEpoch++

	// If we have a peer encap key from a prior Receive(), encapsulate against it to
	// produce new epoch key material. This triggers a KDF ratchet step in SPQR.
	if p.latestPeerEncapKey != nil {
		ek, parseErr := mlkem.NewEncapsulationKey768(p.latestPeerEncapKey)
		if parseErr != nil {
			return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: NewEncapsulationKey768: %w", parseErr)
		}
		ss, _ := ek.Encapsulate() // returns (sharedKey, ciphertext []byte) — no error
		outputKey = ss[:32]
		keyEpoch = p.recvEpoch + 1
	}

	return msg, sendingEpoch, outputKey, keyEpoch, nil
}

// Receive processes the KEM message from SCKAHeader.Msg and derives new epoch key material.
//
// Protocol: the incoming msg is the peer's new encapsulation key (1184 bytes for ML-KEM-768).
// We store it as latestPeerEncapKey (deep copy). We encapsulate against it to produce shared
// secret material. outputKey is returned to trigger a SPQR KDF ratchet step.
// keyEpoch equals recvEpoch + 1 — SPQR validates this (Pitfall 4).
func (p *MLKEMProvider) Receive(msg []byte) (receivingEpoch uint32, outputKey []byte, keyEpoch uint32, err error) {
	if len(msg) == 0 {
		return 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Receive: empty message")
	}

	// Store peer's encapsulation key for our next Send() to encapsulate against (deep copy).
	p.latestPeerEncapKey = append([]byte(nil), msg...)

	// Encapsulate against the peer's key to derive shared secret material.
	ek, parseErr := mlkem.NewEncapsulationKey768(msg)
	if parseErr != nil {
		return 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Receive: NewEncapsulationKey768: %w", parseErr)
	}
	ss, _ := ek.Encapsulate() // returns (sharedKey, ciphertext []byte) — no error

	receivingEpoch = p.recvEpoch
	p.recvEpoch++
	outputKey = ss[:32]
	keyEpoch = p.recvEpoch // == old recvEpoch + 1

	return receivingEpoch, outputKey, keyEpoch, nil
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
	p.sendEpoch = snap.sendEpoch
	p.recvEpoch = snap.recvEpoch
	p.initialized = snap.initialized
}

// Close zeros all key material before releasing resources (D-08).
func (p *MLKEMProvider) Close() error {
	for i := range p.decapSeed {
		p.decapSeed[i] = 0
	}
	for i := range p.latestPeerEncapKey {
		p.latestPeerEncapKey[i] = 0
	}
	p.latestPeerEncapKey = nil
	return nil
}
