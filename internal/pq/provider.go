package pq

import (
	"crypto/mlkem"
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
	// stub — implemented in Plan 02
	_ = mlkem.NewDecapsulationKey768 // keep mlkem import referenced
	return fmt.Errorf("pq: MLKEMProvider.InitInitiator: not implemented")
}

// InitResponder initialises the provider as the Triple Ratchet responder (Bob).
// Generates a fresh random decapsulation key seed.
func (p *MLKEMProvider) InitResponder(sk []byte) error {
	// stub — implemented in Plan 02
	return fmt.Errorf("pq: MLKEMProvider.InitResponder: not implemented")
}

// Send produces the KEM message to include in SCKAHeader.Msg (D-05, D-06).
// Receiver generates new keypair and emits encapsulation key; sender encapsulates against it.
func (p *MLKEMProvider) Send() (msg []byte, sendingEpoch uint32, outputKey []byte, keyEpoch uint32, err error) {
	// stub — implemented in Plan 02
	return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: not implemented")
}

// Receive processes the KEM message from SCKAHeader.Msg and derives new epoch key material.
func (p *MLKEMProvider) Receive(msg []byte) (receivingEpoch uint32, outputKey []byte, keyEpoch uint32, err error) {
	// stub — implemented in Plan 02
	return 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Receive: not implemented")
}

// Snapshot returns a deep copy of all mutable provider state (D-07).
// Called by SPQR before each decrypt attempt; Restore() is called on auth failure.
func (p *MLKEMProvider) Snapshot() any {
	// stub — implemented in Plan 02
	return &mlkemProviderSnapshot{}
}

// Restore reinstates provider state from a snapshot produced by Snapshot() (D-07).
func (p *MLKEMProvider) Restore(snapshot any) {
	// stub — implemented in Plan 02
}

// Close zeros all key material before releasing resources (D-08).
func (p *MLKEMProvider) Close() error {
	// stub — implemented in Plan 02
	return nil
}
