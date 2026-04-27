package classical_test

import (
	"bytes"
	"testing"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/classical"
)

// TestClassicalSession is the CLASS-03 acceptance test.
// Asserts:
//   1. bob.Decrypt(alice.Encrypt(plaintext)) == plaintext
//   2. alice.RootKey == bob.RootKey after handshake
func TestClassicalSession(t *testing.T) {
	// --- Bob generates keys and publishes bundle ---
	bobBundle, bobPrivate, err := classical.NewResponderBundle()
	if err != nil {
		t.Fatalf("NewResponderBundle: %v", err)
	}

	// --- Alice sends handshake ---
	aliceSess, aliceInitMsg, err := classical.InitiatorHandshake(bobBundle)
	if err != nil {
		t.Fatalf("InitiatorHandshake: %v", err)
	}

	// --- Bob receives handshake ---
	bobSess, err := classical.ResponderHandshake(bobPrivate, aliceInitMsg)
	if err != nil {
		t.Fatalf("ResponderHandshake: %v", err)
	}

	// Assertion 2: RootKey equality
	if aliceSess.RootKey != bobSess.RootKey {
		t.Errorf("RootKey mismatch: alice=%x, bob=%x", aliceSess.RootKey, bobSess.RootKey)
	}

	// Assertion 1: Encrypt/Decrypt roundtrip (Alice → Bob)
	plaintext := []byte("hello, ratchet")
	msg, err := aliceSess.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("alice.Encrypt: %v", err)
	}
	decrypted, err := bobSess.Decrypt(msg)
	if err != nil {
		t.Fatalf("bob.Decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("roundtrip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestClassicalSession_BidirectionalExchange(t *testing.T) {
	bobBundle, bobPrivate, err := classical.NewResponderBundle()
	if err != nil {
		t.Fatalf("NewResponderBundle: %v", err)
	}
	aliceSess, aliceInitMsg, err := classical.InitiatorHandshake(bobBundle)
	if err != nil {
		t.Fatalf("InitiatorHandshake: %v", err)
	}
	bobSess, err := classical.ResponderHandshake(bobPrivate, aliceInitMsg)
	if err != nil {
		t.Fatalf("ResponderHandshake: %v", err)
	}

	// Alice → Bob
	plainA := []byte("from alice")
	msgA, err := aliceSess.Encrypt(plainA)
	if err != nil {
		t.Fatalf("alice.Encrypt: %v", err)
	}
	gotA, err := bobSess.Decrypt(msgA)
	if err != nil {
		t.Fatalf("bob.Decrypt: %v", err)
	}
	if !bytes.Equal(gotA, plainA) {
		t.Errorf("A→B: got %q, want %q", gotA, plainA)
	}

	// Bob → Alice
	plainB := []byte("from bob")
	msgB, err := bobSess.Encrypt(plainB)
	if err != nil {
		t.Fatalf("bob.Encrypt: %v", err)
	}
	gotB, err := aliceSess.Decrypt(msgB)
	if err != nil {
		t.Fatalf("alice.Decrypt: %v", err)
	}
	if !bytes.Equal(gotB, plainB) {
		t.Errorf("B→A: got %q, want %q", gotB, plainB)
	}
}

func TestClassicalSession_ErrorOnNilMsg(t *testing.T) {
	bobBundle, bobPrivate, err := classical.NewResponderBundle()
	if err != nil {
		t.Fatalf("NewResponderBundle: %v", err)
	}
	aliceSess, aliceInitMsg, err := classical.InitiatorHandshake(bobBundle)
	if err != nil {
		t.Fatalf("InitiatorHandshake: %v", err)
	}
	_, err = classical.ResponderHandshake(bobPrivate, aliceInitMsg)
	if err != nil {
		t.Fatalf("ResponderHandshake: %v", err)
	}

	_, err = aliceSess.Decrypt(nil)
	if err == nil {
		t.Error("expected error from Decrypt(nil), got nil")
	}
}
