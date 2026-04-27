package keys

import "fmt"

// ToKey32 converts a byte slice to a [32]byte array.
// Returns an error if len(b) != 32.
func ToKey32(b []byte) ([32]byte, error) {
	if len(b) != 32 {
		return [32]byte{}, fmt.Errorf("keys: expected 32 bytes, got %d", len(b))
	}
	var k [32]byte
	copy(k[:], b)
	return k, nil
}
