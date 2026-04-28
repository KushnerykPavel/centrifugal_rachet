package protocol_test

import (
	"encoding/json"
	"testing"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/protocol"
)

// TestMarshalEnvelope_RoundTrip verifies that MarshalEnvelope produces valid JSON
// that round-trips through json.Unmarshal with the correct Type and Payload content.
func TestMarshalEnvelope_RoundTrip(t *testing.T) {
	inner := map[string]string{"k": "v"}
	b, err := protocol.MarshalEnvelope(protocol.TypePrekeyBundle, inner)
	if err != nil {
		t.Fatalf("MarshalEnvelope returned error: %v", err)
	}

	var env protocol.Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("json.Unmarshal into Envelope failed: %v", err)
	}

	if env.Type != protocol.TypePrekeyBundle {
		t.Errorf("env.Type = %q, want %q", env.Type, protocol.TypePrekeyBundle)
	}

	var got map[string]string
	if err := json.Unmarshal(env.Payload, &got); err != nil {
		t.Fatalf("json.Unmarshal Payload failed: %v", err)
	}
	if got["k"] != "v" {
		t.Errorf("Payload[\"k\"] = %q, want \"v\"", got["k"])
	}
}

// TestMarshalEnvelope_AllTypes verifies that all three type constants produce envelopes
// with the correct Type field after a round-trip.
func TestMarshalEnvelope_AllTypes(t *testing.T) {
	types := []string{
		protocol.TypePrekeyBundle,
		protocol.TypeInitialMsg,
		protocol.TypeRatchetMsg,
	}
	for _, typ := range types {
		b, err := protocol.MarshalEnvelope(typ, map[string]int{"n": 1})
		if err != nil {
			t.Fatalf("MarshalEnvelope(%q) returned error: %v", typ, err)
		}
		var env protocol.Envelope
		if err := json.Unmarshal(b, &env); err != nil {
			t.Fatalf("json.Unmarshal for type %q failed: %v", typ, err)
		}
		if env.Type != typ {
			t.Errorf("env.Type = %q, want %q", env.Type, typ)
		}
	}
}

// TestChannelConstants verifies the channel name constants match the locked D-11 values.
func TestChannelConstants(t *testing.T) {
	if protocol.ChannelClassical != "ch-classical" {
		t.Errorf("ChannelClassical = %q, want %q", protocol.ChannelClassical, "ch-classical")
	}
	if protocol.ChannelPQ != "ch-pq" {
		t.Errorf("ChannelPQ = %q, want %q", protocol.ChannelPQ, "ch-pq")
	}
}

// TestMarshalEnvelope_Error verifies that passing an unmarshalable type (channel) returns an error.
func TestMarshalEnvelope_Error(t *testing.T) {
	ch := make(chan int)
	_, err := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, ch)
	if err == nil {
		t.Error("expected error when marshaling channel type, got nil")
	}
}

// TestRatchetPayload_MarshalRoundTrip verifies that RatchetPayload marshals to the expected
// JSON shape and round-trips back to an identical struct.
func TestRatchetPayload_MarshalRoundTrip(t *testing.T) {
	orig := protocol.RatchetPayload{Seq: 3, Text: "hello"}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal RatchetPayload failed: %v", err)
	}

	// Verify JSON contains the expected keys.
	s := string(b)
	if s == "" {
		t.Fatal("marshaled JSON is empty")
	}
	// Check that seq and text appear in the output.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal into map failed: %v", err)
	}
	if _, ok := raw["seq"]; !ok {
		t.Errorf("marshaled JSON missing 'seq' key: %s", s)
	}
	if _, ok := raw["text"]; !ok {
		t.Errorf("marshaled JSON missing 'text' key: %s", s)
	}

	// Verify round-trip.
	var got protocol.RatchetPayload
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal into RatchetPayload failed: %v", err)
	}
	if got.Seq != 3 {
		t.Errorf("Seq = %d, want 3", got.Seq)
	}
	if got.Text != "hello" {
		t.Errorf("Text = %q, want \"hello\"", got.Text)
	}
}
