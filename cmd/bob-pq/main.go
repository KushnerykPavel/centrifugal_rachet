package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	centrifuge "github.com/centrifugal/centrifuge-go"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/metrics"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/pq"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/protocol"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/transport"
)

const (
	msgCount = 10000
	timeout  = 10 * time.Minute
)

func main() {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9094"
	}
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			log.Printf("bob-pq: metrics server: %v", err)
		}
	}()

	url := os.Getenv("CENTRIFUGO_URL")
	if url == "" {
		url = "ws://localhost:8000/connection/websocket"
	}

	cl := transport.NewClient(url)
	if err := cl.Connect(); err != nil {
		log.Fatalf("bob-pq: Connect: %v", err)
	}
	defer cl.Disconnect()

	bundle, priv, err := pq.NewResponderBundle()
	if err != nil {
		log.Fatalf("bob-pq: NewResponderBundle: %v", err)
	}

	var (
		sess      *pq.Session
		mu        sync.Mutex // guards sess, Encrypt, Decrypt
		echoCount int32
		sub       *centrifuge.Subscription
	)

	const selfID = "bob-pq"

	sub, err = cl.Subscribe(protocol.ChannelPQ, func(data []byte) {
		go func() {
			var env protocol.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				log.Printf("bob-pq: unmarshal envelope: %v", err)
				return
			}
			if env.From == selfID {
				return
			}
			switch env.Type {
			case protocol.TypeInitialMsg:
				t0 := time.Now()
				var initMsg pq.InitialMessage
				if err := json.Unmarshal(env.Payload, &initMsg); err != nil {
					log.Printf("bob-pq: unmarshal initial_msg: %v", err)
					return
				}
				s, err := pq.ResponderHandshake(priv, initMsg)
				metrics.HandshakeDurationSeconds.Observe(time.Since(t0).Seconds())
				if err != nil {
					log.Printf("bob-pq: ResponderHandshake: %v", err)
					return
				}
				mu.Lock()
				sess = s
				mu.Unlock()
				log.Printf("bob-pq: handshake complete")

			case protocol.TypeRatchetMsg:
				mu.Lock()
				sessNil := sess == nil
				mu.Unlock()
				if sessNil {
					log.Printf("bob-pq: session not yet initialized, dropping ratchet_msg")
					return
				}
				var msg pq.TripleRatchetMessage
				if err := json.Unmarshal(env.Payload, &msg); err != nil {
					log.Printf("bob-pq: unmarshal ratchet_msg: %v", err)
					return
				}
				mu.Lock()
				t0 := time.Now()
				plain, err := sess.Decrypt(&msg)
				metrics.DecryptDurationSeconds.Observe(time.Since(t0).Seconds())
				metrics.MessageWireBytes.Observe(float64(len(data)))
				mu.Unlock()
				if err != nil {
					log.Printf("bob-pq: Decrypt: %v", err)
					return
				}

				// Unmarshal the decrypted bytes as RatchetPayload (D-05).
				var rp protocol.RatchetPayload
				if err := json.Unmarshal(plain, &rp); err != nil {
					log.Printf("bob-pq: unmarshal ratchet payload: %v", err)
					return
				}
				log.Printf("bob-pq: recv: seq=%d text=%s", rp.Seq, rp.Text)

				// Build echo payload and encrypt (D-05).
				echoRP := protocol.RatchetPayload{Seq: rp.Seq, Text: "echo: " + rp.Text}
				echoJSON, err := json.Marshal(echoRP)
				if err != nil {
					log.Printf("bob-pq: marshal echo payload: %v", err)
					return
				}
				mu.Lock()
				t0 = time.Now()
				echoMsg, err := sess.Encrypt(echoJSON)
				metrics.EncryptDurationSeconds.Observe(time.Since(t0).Seconds())
				mu.Unlock()
				if err != nil {
					log.Printf("bob-pq: Encrypt echo: %v", err)
					return
				}
				raw, err := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, selfID, echoMsg)
				if err != nil {
					log.Printf("bob-pq: MarshalEnvelope echo: %v", err)
					return
				}
				metrics.MessageWireBytes.Observe(float64(len(raw)))
				if err := cl.Publish(context.Background(), sub, raw); err != nil {
					log.Printf("bob-pq: Publish echo: %v", err)
					return
				}
				n := atomic.AddInt32(&echoCount, 1)
			if n == msgCount {
				log.Printf("bob-pq: sent %d echoes, exiting", msgCount)
					os.Exit(0)
				}
			}
		}()
	})
	if err != nil {
		log.Fatalf("bob-pq: Subscribe: %v", err)
	}

	raw, err := protocol.MarshalEnvelope(protocol.TypePrekeyBundle, selfID, bundle)
	if err != nil {
		log.Fatalf("bob-pq: MarshalEnvelope prekey_bundle: %v", err)
	}
	if err := cl.Publish(context.Background(), sub, raw); err != nil {
		log.Fatalf("bob-pq: Publish prekey_bundle: %v", err)
	}
	log.Printf("bob-pq: published prekey bundle on %s, waiting for Alice", protocol.ChannelPQ)

	select {}
}
