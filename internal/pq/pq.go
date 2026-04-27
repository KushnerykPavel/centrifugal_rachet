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
	ik, err := pqxdh.GenerateIdentityKey()
	if err != nil {
		return nil, nil, fmt.Errorf("pq: NewResponderBundle: GenerateIdentityKey: %w", err)
	}
	spk, err := pqxdh.GenerateSPK(ik, 1)
	if err != nil {
		return nil, nil, fmt.Errorf("pq: NewResponderBundle: GenerateSPK: %w", err)
	}
	opk, err := pqxdh.GenerateOPK(1)
	if err != nil {
		return nil, nil, fmt.Errorf("pq: NewResponderBundle: GenerateOPK: %w", err)
	}
	kemSPK, err := pqxdh.GenerateKEMSPK(ik, 1, pqxdh.MLKEM768)
	if err != nil {
		return nil, nil, fmt.Errorf("pq: NewResponderBundle: GenerateKEMSPK: %w", err)
	}
	kemOPK, err := pqxdh.GenerateKEMOPK(ik, 1, pqxdh.MLKEM768)
	if err != nil {
		return nil, nil, fmt.Errorf("pq: NewResponderBundle: GenerateKEMOPK: %w", err)
	}
	drPriv, drPub, err := doubleratchet.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("pq: NewResponderBundle: GenerateKeyPair: %w", err)
	}

	bundle := &pqxdh.PrekeyBundle{
		IdentityKey:       ik.PublicKey,
		SignedPreKey:      spk.PublicKey,
		SPKID:             spk.KeyID,
		SPKSignature:      spk.Signature,
		PQPreKey:          kemOPK.EncapsulationKey, // use OPK (consumed once per session)
		PQPreKeyID:        kemOPK.KeyID,
		PQPreKeySignature: kemOPK.Signature,
		PQParams:          pqxdh.MLKEM768, // D-02: explicit, never default
		OneTimePreKey:     &opk.PublicKey,
		OPKID:             &opk.KeyID,
	}
	priv := &ResponderKeys{
		ik:     ik,
		spk:    spk,
		opk:    opk,
		kemSPK: kemSPK,
		kemOPK: kemOPK,
		drPriv: drPriv,
		drPub:  drPub,
	}
	return bundle, priv, nil
}

// InitiatorHandshake performs Alice's side of PQXDH (D-01, D-02, D-09, D-10).
// Returns Alice's Session (with RootKey set) and the InitialMessage to send to Bob.
func InitiatorHandshake(bundle *pqxdh.PrekeyBundle) (*Session, InitialMessage, error) {
	aliceIK, err := pqxdh.GenerateIdentityKey()
	if err != nil {
		return nil, InitialMessage{}, fmt.Errorf("pq: InitiatorHandshake: GenerateIdentityKey: %w", err)
	}

	result, initMsg, err := pqxdh.SendHandshake(aliceIK, bundle)
	if err != nil {
		return nil, InitialMessage{}, fmt.Errorf("pq: InitiatorHandshake: SendHandshake: %w", err)
	}

	aliceSCKA := &MLKEMProvider{}
	tr, err := doubleratchet.InitAliceTripleRatchet(result.RootKey[:], bundle.SignedPreKey, aliceSCKA, nil)
	if err != nil {
		return nil, InitialMessage{}, fmt.Errorf("pq: InitiatorHandshake: InitAliceTripleRatchet: %w", err)
	}

	sess := &Session{
		RootKey: result.RootKey,
		ad:      result.AD,
		tr:      tr,
	}
	return sess, initMsg, nil
}

// ResponderHandshake performs Bob's side of PQXDH (D-01, D-04, D-09, D-10).
// Returns Bob's Session (with RootKey set). Bob must have already published his bundle.
func ResponderHandshake(priv *ResponderKeys, initMsg InitialMessage) (*Session, error) {
	pqpk := priv.kemOPK.DecapsKey() // *KEMPreKey — required by ReceiveHandshake (Pitfall 5)
	result, err := pqxdh.ReceiveHandshake(priv.ik, &priv.spk, &priv.opk, pqpk, &initMsg)
	if err != nil {
		return nil, fmt.Errorf("pq: ResponderHandshake: ReceiveHandshake: %w", err)
	}

	bobSCKA := &MLKEMProvider{}
	bobDRKP := doubleratchet.KeyPair{PrivateKey: priv.drPriv, PublicKey: priv.drPub}
	tr, err := doubleratchet.InitBobTripleRatchet(result.RootKey[:], bobDRKP, bobSCKA, nil)
	if err != nil {
		return nil, fmt.Errorf("pq: ResponderHandshake: InitBobTripleRatchet: %w", err)
	}

	sess := &Session{
		RootKey: result.RootKey,
		ad:      result.AD,
		tr:      tr,
	}
	return sess, nil
}

// Encrypt encrypts plaintext and returns a *TripleRatchetMessage (D-11).
// The associated data from the PQXDH handshake is threaded automatically.
// NOTE: TripleRatchetSession.Encrypt returns VALUE — facade wraps it in pointer.
func (s *Session) Encrypt(plaintext []byte) (*TripleRatchetMessage, error) {
	msg, err := s.tr.Encrypt(plaintext, s.ad)
	if err != nil {
		return nil, fmt.Errorf("pq: Encrypt: %w", err)
	}
	return &msg, nil
}

// Decrypt decrypts msg and returns the original plaintext (D-11).
// The associated data from the PQXDH handshake is threaded automatically.
// NOTE: TripleRatchetSession.Decrypt takes VALUE — facade dereferences pointer.
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

// Close releases resources held by the underlying Triple Ratchet session.
func (s *Session) Close() error {
	if s.tr != nil {
		return s.tr.Close()
	}
	return nil
}
