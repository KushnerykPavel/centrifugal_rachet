//go:build integration

package transport_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/transport"
)

const centrifugoAddr = "ws://localhost:8000/connection/websocket"

// TestTransport_ConnectSubscribePublishDisconnect verifies the full lifecycle
// against a real Centrifugo instance started with client.insecure: true.
//
// Run: go test -tags=integration ./internal/transport/... -v
// Prerequisite: Centrifugo running at ws://localhost:8000/connection/websocket
func TestTransport_ConnectSubscribePublishDisconnect(t *testing.T) {
	const channel = "ch-integration-test"
	const payload = "hello-transport"

	// --- Connect ---
	cl := transport.NewClient(centrifugoAddr)
	if err := cl.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer cl.Disconnect()

	// --- Subscribe ---
	var (
		received []byte
		once     sync.Once
		done     = make(chan struct{})
	)
	sub, err := cl.Subscribe(channel, func(data []byte) {
		once.Do(func() {
			received = data
			close(done)
		})
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	_ = sub

	// Brief settle to ensure subscription is active before publishing
	time.Sleep(200 * time.Millisecond)

	// --- Publish ---
	if err := cl.Publish(context.Background(), sub, []byte(payload)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// --- Wait for publication to arrive ---
	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for publication")
	}

	// --- Verify payload ---
	if string(received) != payload {
		t.Errorf("received %q, want %q", received, payload)
	}

	// --- Disconnect (also called via defer, but explicit for clarity) ---
	cl.Disconnect()
}
