package protocol

import "encoding/json"

// Envelope is the JSON wrapper for all Centrifugo channel messages.
// Type field drives dispatch; From identifies the sender so subscribers can
// filter out their own publications; Payload carries the inner struct as raw JSON.
type Envelope struct {
	Type    string          `json:"type"`
	From    string          `json:"from"`
	Payload json.RawMessage `json:"payload"`
}

// Type constants for envelope dispatch.
const (
	TypePrekeyBundle = "prekey_bundle"
	TypeInitialMsg   = "initial_msg"
	TypeRatchetMsg   = "ratchet_msg"
)

// Channel name constants (D-11).
const (
	ChannelClassical = "ch-classical"
	ChannelPQ        = "ch-pq"
)

// RatchetPayload is the JSON structure inside a ratchet_msg Envelope.Payload (D-05).
// Seq is the 1-based message sequence number; Text is the human-readable content.
// Example: {"seq": 1, "text": "hello from alice-classical 1"}
// Echo:    {"seq": 1, "text": "echo: hello from alice-classical 1"}
type RatchetPayload struct {
	Seq  int    `json:"seq"`
	Text string `json:"text"`
}

// MarshalEnvelope marshals inner to JSON and wraps it in an Envelope.
// from identifies the sending party so subscribers can filter their own publications.
// Returns the outer Envelope as JSON bytes ready to publish.
func MarshalEnvelope(typ, from string, inner any) ([]byte, error) {
	payload, err := json.Marshal(inner)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Type: typ, From: from, Payload: json.RawMessage(payload)})
}
