package keys_test

import (
	"testing"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/keys"
)

func TestToKey32_ValidInput(t *testing.T) {
	input := make([]byte, 32)
	for i := range input {
		input[i] = byte(i)
	}
	got, err := keys.ToKey32(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, b := range got {
		if b != byte(i) {
			t.Errorf("byte %d: got %d, want %d", i, b, byte(i))
		}
	}
}

func TestToKey32_TooShort(t *testing.T) {
	_, err := keys.ToKey32(make([]byte, 16))
	if err == nil {
		t.Fatal("expected error for 16-byte input, got nil")
	}
}

func TestToKey32_TooLong(t *testing.T) {
	_, err := keys.ToKey32(make([]byte, 64))
	if err == nil {
		t.Fatal("expected error for 64-byte input, got nil")
	}
}

func TestToKey32_Empty(t *testing.T) {
	_, err := keys.ToKey32([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestToKey32_NoCopy(t *testing.T) {
	input := make([]byte, 32)
	input[0] = 0xAA
	got, _ := keys.ToKey32(input)
	input[0] = 0xFF
	if got[0] != 0xAA {
		t.Error("mutation of input slice should not affect the returned [32]byte")
	}
}
