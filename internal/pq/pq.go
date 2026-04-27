package pq

import (
	"errors"
	"fmt"

	doubleratchet "github.com/KushnerykPavel/go-doubleratchet"
	"github.com/KushnerykPavel/go-doubleratchet/pqxdh"
)

// TripleRatchetMessage re-exports the library type. Do NOT redefine (D-12).
// Phase 3 passes this type directly to JSON serialization.
type TripleRatchetMessage = doubleratchet.TripleRatchetMessage

// InitialMessage re-exports the PQXDH initial message type.
type InitialMessage = pqxdh.InitialMessage

// ResponderKeys holds Bob's private key material needed to complete the handshake.
// Not transmitted — kept secret by Bob.
type ResponderKeys struct {
	ik     pqxdh.IdentityKey
	spk    pqxdh.SignedPreKey
	opk    pqxdh.OneTimePreKey
	kemSPK pqxdh.KEMSignedPreKey
	kemOPK pqxdh.KEMOneTimePreKey
	drPriv [32]byte // Bob's DR private key
	drPub  [32]byte // Bob's DR public key (included in PrekeyBundle)
}

// Session is the PQ encrypted-chat session facade.
// RootKey is exported so PQ-01/PQ-04 can assert alice.RootKey == bob.RootKey.
type Session struct {
	// RootKey is HandshakeResult.RootKey — the Triple Ratchet shared secret seed.
	RootKey [32]byte

	ad []byte                              // HandshakeResult.AD, threaded through every Encrypt/Decrypt
	tr *doubleratchet.TripleRatchetSession // nil until handshake completes
}

// NewResponderBundle generates Bob's full PQXDH prekey bundle (D-03).
// Returns the public pqxdh.PrekeyBundle to publish and the private ResponderKeys to keep secret.
func NewResponderBundle() (*pqxdh.PrekeyBundle, *ResponderKeys, error) {
	// stub — implemented in Plan 03
	return nil, nil, fmt.Errorf("pq: NewResponderBundle: not implemented")
}

// InitiatorHandshake performs Alice's side of PQXDH (D-01, D-02, D-09, D-10).
// Returns Alice's Session (with RootKey set) and the InitialMessage to send to Bob.
func InitiatorHandshake(bundle *pqxdh.PrekeyBundle) (*Session, InitialMessage, error) {
	// stub — implemented in Plan 03
	return nil, InitialMessage{}, fmt.Errorf("pq: InitiatorHandshake: not implemented")
}

// ResponderHandshake performs Bob's side of PQXDH (D-01, D-04, D-09, D-10).
// Returns Bob's Session (with RootKey set). Bob must have already published his bundle.
func ResponderHandshake(priv *ResponderKeys, initMsg InitialMessage) (*Session, error) {
	// stub — implemented in Plan 03
	return nil, fmt.Errorf("pq: ResponderHandshake: not implemented")
}

// Encrypt encrypts plaintext and returns a *TripleRatchetMessage (D-11).
// The associated data from the PQXDH handshake is threaded automatically.
// NOTE: TripleRatchetSession.Encrypt returns VALUE — facade wraps it in pointer.
func (s *Session) Encrypt(plaintext []byte) (*TripleRatchetMessage, error) {
	// stub — implemented in Plan 03
	_ = errors.New("") // keep errors import referenced
	return nil, fmt.Errorf("pq: Encrypt: not implemented")
}

// Decrypt decrypts msg and returns the original plaintext (D-11).
// The associated data from the PQXDH handshake is threaded automatically.
// NOTE: TripleRatchetSession.Decrypt takes VALUE — facade dereferences pointer.
func (s *Session) Decrypt(msg *TripleRatchetMessage) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("pq: Decrypt: nil message")
	}
	return nil, fmt.Errorf("pq: Decrypt: not implemented")
}

// Close releases resources held by the underlying Triple Ratchet session.
func (s *Session) Close() {
	if s.tr != nil {
		s.tr.Close()
	}
}
