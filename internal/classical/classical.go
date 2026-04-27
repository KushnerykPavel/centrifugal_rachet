package classical

import (
	"errors"
	"fmt"

	doubleratchet "github.com/KushnerykPavel/go-doubleratchet"
	"github.com/KushnerykPavel/go-doubleratchet/x3dh"
)

// ResponderKeys holds Bob's private key material needed to complete the handshake.
// Bob generates this once per session; it is NOT transmitted.
type ResponderKeys struct {
	ik  x3dh.IdentityKey
	spk x3dh.SignedPreKey
}

// PrekeyBundle is the public portion Bob publishes so Alice can initiate.
// In Phase 3 this struct is JSON-serialized and sent as the first Centrifugo message.
type PrekeyBundle struct {
	IdentityKey  [32]byte
	SignedPreKey [32]byte
	SPKID        uint32
	SPKSignature [64]byte
}

// InitialMessage is the handshake message Alice sends to Bob.
// In Phase 3 this is JSON-serialized and sent as the second Centrifugo message.
type InitialMessage = x3dh.InitialMessage

// Message is the wire type for encrypted ratchet messages.
// Wraps doubleratchet.Message (value type) and stores the associated data
// so that Decrypt callers do not need to manage AD themselves.
// In Phase 3 this is JSON-serialized into the Centrifugo message envelope.
type Message struct {
	DR doubleratchet.Message
}

// Session is the classical encrypted-chat session facade.
// RootKey is exported so CLASS-03 can assert alice.RootKey == bob.RootKey.
type Session struct {
	// RootKey is a copy of HandshakeResult.SharedSecret — the DR root key seed.
	// It is set during handshake and is read-only afterwards.
	RootKey [32]byte

	ad []byte               // HandshakeResult.AD (IKA_pub ‖ IKB_pub), threaded through every Encrypt/Decrypt
	dr *doubleratchet.Session
}

// NewResponderBundle generates Bob's identity key and signed pre-key.
// Returns the public PrekeyBundle to publish and the private ResponderKeys to keep secret.
func NewResponderBundle() (*PrekeyBundle, *ResponderKeys, error) {
	ik, err := x3dh.GenerateIdentityKey()
	if err != nil {
		return nil, nil, fmt.Errorf("classical: GenerateIdentityKey: %w", err)
	}
	spk, err := x3dh.GenerateSPK(ik, 1)
	if err != nil {
		return nil, nil, fmt.Errorf("classical: GenerateSPK: %w", err)
	}

	bundle := &PrekeyBundle{
		IdentityKey:  ik.PublicKey,
		SignedPreKey: spk.PublicKey,
		SPKID:        spk.KeyID,
		SPKSignature: spk.Signature,
	}
	priv := &ResponderKeys{ik: ik, spk: spk}
	return bundle, priv, nil
}

// InitiatorHandshake performs Alice's side of X3DH.
// Returns Alice's Session (with RootKey set) and the InitialMessage to send to Bob.
func InitiatorHandshake(bundle *PrekeyBundle) (*Session, InitialMessage, error) {
	aliceIK, err := x3dh.GenerateIdentityKey()
	if err != nil {
		return nil, InitialMessage{}, fmt.Errorf("classical: GenerateIdentityKey (alice): %w", err)
	}

	xBundle := &x3dh.PrekeyBundle{
		IdentityKey:  bundle.IdentityKey,
		SignedPreKey: bundle.SignedPreKey,
		SPKID:        bundle.SPKID,
		SPKSignature: bundle.SPKSignature,
	}

	aliceResult, initMsg, err := x3dh.SendHandshake(aliceIK, xBundle)
	if err != nil {
		return nil, InitialMessage{}, fmt.Errorf("classical: SendHandshake: %w", err)
	}

	// InitAlice: Alice is the initiator; she knows Bob's ratchet public key (= SPK public key).
	drSess, err := doubleratchet.InitAlice(aliceResult.SharedSecret[:], bundle.SignedPreKey, nil)
	if err != nil {
		return nil, InitialMessage{}, fmt.Errorf("classical: InitAlice: %w", err)
	}

	sess := &Session{
		RootKey: aliceResult.SharedSecret,
		ad:      aliceResult.AD,
		dr:      drSess,
	}
	return sess, initMsg, nil
}

// ResponderHandshake performs Bob's side of X3DH.
// Returns Bob's Session (with RootKey set). Bob must have already published his bundle.
func ResponderHandshake(priv *ResponderKeys, initMsg InitialMessage) (*Session, error) {
	bobResult, err := x3dh.ReceiveHandshake(priv.ik, &priv.spk, nil, initMsg)
	if err != nil {
		return nil, fmt.Errorf("classical: ReceiveHandshake: %w", err)
	}

	// InitBob: Bob is the responder; his SPK serves as his initial DR ratchet key pair.
	bobRatchetKP := doubleratchet.KeyPair{
		PrivateKey: priv.spk.PrivateKey,
		PublicKey:  priv.spk.PublicKey,
	}
	drSess, err := doubleratchet.InitBob(bobResult.SharedSecret[:], bobRatchetKP, nil)
	if err != nil {
		return nil, fmt.Errorf("classical: InitBob: %w", err)
	}

	sess := &Session{
		RootKey: bobResult.SharedSecret,
		ad:      bobResult.AD,
		dr:      drSess,
	}
	return sess, nil
}

// Encrypt encrypts plaintext and returns a *Message.
// The associated data from the X3DH handshake is threaded automatically.
func (s *Session) Encrypt(plaintext []byte) (*Message, error) {
	drMsg, err := s.dr.Encrypt(plaintext, s.ad)
	if err != nil {
		return nil, fmt.Errorf("classical: Encrypt: %w", err)
	}
	return &Message{DR: drMsg}, nil
}

// Decrypt decrypts msg and returns the original plaintext.
// The associated data from the X3DH handshake is threaded automatically.
func (s *Session) Decrypt(msg *Message) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("classical: Decrypt: nil message")
	}
	plain, err := s.dr.Decrypt(msg.DR, s.ad)
	if err != nil {
		return nil, fmt.Errorf("classical: Decrypt: %w", err)
	}
	return plain, nil
}

// Close releases resources held by the underlying DR session.
func (s *Session) Close() {
	if s.dr != nil {
		s.dr.Close()
	}
}
