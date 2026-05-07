package pq

import (
	"bytes"
	"crypto/mlkem"
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

const (
	mlkem768EncapKeySize      = 1184 // ML-KEM-768 encapsulation key size (FIPS 203)
	mlkem768CiphertextSize    = 1088 // ML-KEM-768 ciphertext size (FIPS 203)
	mlkem768ContinuationSize  = 4    // continuation message: 4-byte big-endian sendEpoch
)

// mlkemProviderSnapshot holds a deep copy of MLKEMProvider state for rollback (D-07).
type mlkemProviderSnapshot struct {
	decapSeed           [64]byte
	latestPeerEncapKey  []byte
	pendingEncapKey     []byte
	lastSeenAnnounceKey []byte
	sendEpoch           uint32
	recvEpoch           uint32
	announceEpoch       uint32
	isInitiator         bool
	initialized         bool
}

// MLKEMProvider implements scka.Provider using ML-KEM-768 (D-05, D-06).
//
// Two-round KEM protocol per epoch:
//   - Initiator (Alice) announces an encapsulation key (Round 1, 1184 bytes).
//     Multiple consecutive sends reuse the same pending key so the decapSeed
//     remains valid until the peer responds.
//   - Responder (Bob) encapsulates against it, returning a ciphertext (Round 2, 1088 bytes).
//     Between KEM exchanges Bob sends 4-byte continuation messages to stay on the
//     current chain without triggering spurious epoch advances.
//   - Initiator decapsulates the ciphertext, both derive the same shared secret,
//     and the SPQR layer advances to the new KDF epoch.
type MLKEMProvider struct {
	decapSeed           [64]byte // FIPS 203 seed (d‖z) for current decapsulation key
	latestPeerEncapKey  []byte   // encap key received from peer; nil until Receive(encapKey)
	pendingEncapKey     []byte   // encap key announced but not yet responded to
	lastSeenAnnounceKey []byte   // last encap key bytes received, to detect duplicates
	sendEpoch           uint32
	recvEpoch           uint32
	announceEpoch       uint32 // count of distinct peer announcements received
	isInitiator         bool
	initialized         bool
}

// InitInitiator initialises the provider as the Triple Ratchet initiator (Alice).
func (p *MLKEMProvider) InitInitiator(sk []byte) error {
	if _, err := rand.Read(p.decapSeed[:]); err != nil {
		return fmt.Errorf("pq: MLKEMProvider.InitInitiator: %w", err)
	}
	p.sendEpoch = 0
	p.recvEpoch = 0
	p.announceEpoch = 0
	p.isInitiator = true
	p.initialized = true
	return nil
}

// InitResponder initialises the provider as the Triple Ratchet responder (Bob).
func (p *MLKEMProvider) InitResponder(sk []byte) error {
	if _, err := rand.Read(p.decapSeed[:]); err != nil {
		return fmt.Errorf("pq: MLKEMProvider.InitResponder: %w", err)
	}
	p.sendEpoch = 0
	p.recvEpoch = 0
	p.announceEpoch = 0
	p.isInitiator = false
	p.initialized = true
	return nil
}

// Send produces the KEM message for SCKAHeader.Msg.
//
// Three paths:
//   - Encapsulator: latestPeerEncapKey is set → encapsulate, emit ciphertext, advance epoch.
//   - Initiator announcer: no peer key → reuse or generate pendingEncapKey, stay in epoch.
//   - Responder continuation: no peer key → emit 4-byte sendEpoch, stay in epoch.
func (p *MLKEMProvider) Send() (msg []byte, sendingEpoch uint32, outputKey []byte, keyEpoch uint32, err error) {
	sendingEpoch = p.sendEpoch

	// Encapsulator path: encapsulate peer's announced key, triggering an epoch transition.
	if p.latestPeerEncapKey != nil {
		ek, parseErr := mlkem.NewEncapsulationKey768(p.latestPeerEncapKey)
		if parseErr != nil {
			return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: NewEncapsulationKey768: %w", parseErr)
		}
		ss, ct := ek.Encapsulate()
		p.latestPeerEncapKey = nil
		outputKey = ss[:32]
		keyEpoch = p.announceEpoch // matches the epoch established when the announcement was received
		p.sendEpoch++              // next send uses the new chain
		return ct, sendingEpoch, outputKey, keyEpoch, nil
	}

	// Initiator (Announcer) path: reuse pending encap key across consecutive sends so
	// the decapSeed stays valid until the peer returns the ciphertext.
	if p.isInitiator {
		if p.pendingEncapKey == nil {
			var newSeed [64]byte
			if _, err = rand.Read(newSeed[:]); err != nil {
				return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: generate seed: %w", err)
			}
			dk, err := mlkem.NewDecapsulationKey768(newSeed[:])
			if err != nil {
				return nil, 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Send: NewDecapsulationKey768: %w", err)
			}
			p.decapSeed = newSeed
			p.pendingEncapKey = dk.EncapsulationKey().Bytes()
		}
		// Do NOT increment sendEpoch — epoch advances only when peer returns ciphertext.
		return p.pendingEncapKey, sendingEpoch, nil, 0, nil
	}

	// Responder continuation path: no peer encap key available, send a 4-byte epoch
	// indicator so the peer can route the message to the correct KDF chain.
	epochBytes := make([]byte, mlkem768ContinuationSize)
	binary.BigEndian.PutUint32(epochBytes, p.sendEpoch)
	return epochBytes, sendingEpoch, nil, 0, nil
}

// Receive processes the KEM message from SCKAHeader.Msg.
//
// Dispatch by message length:
//   - 1184 bytes: encapsulation key announcement. Duplicate announcements (same bytes)
//     return the already-established epoch without re-storing, preventing spurious
//     re-encapsulation. New announcements increment announceEpoch.
//   - 1088 bytes: ciphertext. Decapsulate to recover shared secret, advance epochs.
//   - 4 bytes: continuation. Extract sendEpoch for KDF chain routing, no KEM action.
func (p *MLKEMProvider) Receive(msg []byte) (receivingEpoch uint32, outputKey []byte, keyEpoch uint32, err error) {
	switch len(msg) {
	case mlkem768EncapKeySize:
		// Duplicate announcement: same encap key bytes as the last one we stored.
		// The peer is still in the same SPQR epoch; return that epoch and skip re-storing.
		if bytes.Equal(msg, p.lastSeenAnnounceKey) {
			receivingEpoch = p.announceEpoch - 1
			return receivingEpoch, nil, 0, nil
		}
		// New announcement: store for encapsulation in next Send().
		p.lastSeenAnnounceKey = append([]byte(nil), msg...)
		p.latestPeerEncapKey = append([]byte(nil), msg...)
		receivingEpoch = p.announceEpoch
		p.announceEpoch++
		return receivingEpoch, nil, 0, nil

	case mlkem768CiphertextSize:
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
		p.sendEpoch = p.recvEpoch // epoch transition: future Initiator sends use the new chain
		p.pendingEncapKey = nil   // announcement consumed — next Send generates a fresh key
		outputKey = ss[:32]
		keyEpoch = p.recvEpoch // == old recvEpoch + 1
		return receivingEpoch, outputKey, keyEpoch, nil

	case mlkem768ContinuationSize:
		// Responder sent a continuation: no KEM action, just route to the correct chain.
		receivingEpoch = binary.BigEndian.Uint32(msg)
		return receivingEpoch, nil, 0, nil

	default:
		return 0, nil, 0, fmt.Errorf("pq: MLKEMProvider.Receive: unexpected message length %d (want %d, %d, or %d)",
			len(msg), mlkem768EncapKeySize, mlkem768CiphertextSize, mlkem768ContinuationSize)
	}
}

// Snapshot returns a deep copy of all mutable provider state (D-07).
func (p *MLKEMProvider) Snapshot() any {
	snap := &mlkemProviderSnapshot{
		decapSeed:     p.decapSeed,
		sendEpoch:     p.sendEpoch,
		recvEpoch:     p.recvEpoch,
		announceEpoch: p.announceEpoch,
		isInitiator:   p.isInitiator,
		initialized:   p.initialized,
	}
	if p.latestPeerEncapKey != nil {
		snap.latestPeerEncapKey = append([]byte(nil), p.latestPeerEncapKey...)
	}
	if p.pendingEncapKey != nil {
		snap.pendingEncapKey = append([]byte(nil), p.pendingEncapKey...)
	}
	if p.lastSeenAnnounceKey != nil {
		snap.lastSeenAnnounceKey = append([]byte(nil), p.lastSeenAnnounceKey...)
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
	p.pendingEncapKey = snap.pendingEncapKey
	p.lastSeenAnnounceKey = snap.lastSeenAnnounceKey
	p.sendEpoch = snap.sendEpoch
	p.recvEpoch = snap.recvEpoch
	p.announceEpoch = snap.announceEpoch
	p.isInitiator = snap.isInitiator
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
	for i := range p.pendingEncapKey {
		p.pendingEncapKey[i] = 0
	}
	p.pendingEncapKey = nil
	for i := range p.lastSeenAnnounceKey {
		p.lastSeenAnnounceKey[i] = 0
	}
	p.lastSeenAnnounceKey = nil
	return nil
}
