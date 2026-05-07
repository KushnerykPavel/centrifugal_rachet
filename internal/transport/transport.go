package transport

import (
	"context"
	"fmt"

	"github.com/centrifugal/centrifuge-go"
)

// Client wraps a centrifuge-go JSON client with a simple lifecycle API.
// Connect, subscribe, publish, and disconnect are the only operations Phase 3 needs.
type Client struct {
	addr string
	c    *centrifuge.Client
}

// NewClient creates a Client pointing at the given WebSocket address.
// addr example: "ws://localhost:8000/connection/websocket"
// No connection is made until Connect() is called.
func NewClient(addr string) *Client {
	return &Client{addr: addr}
}

// Connect establishes the WebSocket connection to Centrifugo.
// Registers no-op handlers for OnConnected/OnDisconnected/OnError so
// centrifuge-go does not log "handler not set" warnings.
func (cl *Client) Connect() error {
	cl.c = centrifuge.NewJsonClient(cl.addr, centrifuge.Config{
		Token: "",
	})
	cl.c.OnConnected(func(_ centrifuge.ConnectedEvent) {
		fmt.Println("transport: connected to centrifugo")
	})
	cl.c.OnDisconnected(func(_ centrifuge.DisconnectedEvent) {})
	cl.c.OnError(func(e centrifuge.ErrorEvent) {
		fmt.Printf("transport: centrifuge error: %v\n", e.Error)
	})
	return cl.c.Connect()
}

// Subscribe subscribes to a channel and registers an OnPublication handler.
// handler receives raw publication bytes.
//
// CRITICAL: Any blocking work inside handler MUST be dispatched via go func().
// centrifuge-go v0.10.12 runs OnPublication on the connection read goroutine;
// calling Publish or Disconnect inside the handler without a goroutine deadlocks.
func (cl *Client) Subscribe(channel string, handler func(data []byte)) (*centrifuge.Subscription, error) {
	sub, err := cl.c.NewSubscription(channel, centrifuge.SubscriptionConfig{Recoverable: true})
	if err != nil {
		return nil, fmt.Errorf("transport: NewSubscription %q: %w", channel, err)
	}
	sub.OnPublication(func(e centrifuge.PublicationEvent) {
		// handler is Phase 3's responsibility — it MUST use go func() if blocking.
		handler(e.Data)
	})
	if err := sub.Subscribe(); err != nil {
		return nil, fmt.Errorf("transport: Subscribe %q: %w", channel, err)
	}
	return sub, nil
}

// Publish sends data to a channel.
// sub must be a *centrifuge.Subscription returned by Subscribe().
func (cl *Client) Publish(ctx context.Context, sub *centrifuge.Subscription, data []byte) error {
	_, err := sub.Publish(ctx, data)
	if err != nil {
		return fmt.Errorf("transport: Publish: %w", err)
	}
	return nil
}

// Disconnect cleanly closes the WebSocket connection.
// Call this in a defer after Connect() succeeds.
func (cl *Client) Disconnect() {
	if cl.c != nil {
		cl.c.Disconnect()
		cl.c.Close()
	}
}
