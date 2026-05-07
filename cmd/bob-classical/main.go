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

	"github.com/centrifugal/centrifuge-go"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/classical"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/metrics"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/protocol"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/transport"
)

const msgCount = 10000
const timeout = 10 * time.Minute

func main() {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9092"
	}
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			log.Printf("bob-classical: metrics server: %v", err)
		}
	}()

	url := os.Getenv("CENTRIFUGO_URL")
	if url == "" {
		url = "ws://localhost:8000/connection/websocket"
	}

	cl := transport.NewClient(url)
	if err := cl.Connect(); err != nil {
		log.Fatalf("bob-classical: Connect: %v", err)
	}
	defer cl.Disconnect()

	bundle, priv, err := classical.NewResponderBundle()
	if err != nil {
		log.Fatalf("bob-classical: NewResponderBundle: %v", err)
	}

	var (
		sess      *classical.Session
		mu        sync.Mutex // guards sess, Encrypt, Decrypt
		echoCount int32
		sub       *centrifuge.Subscription
	)

	const selfID = "bob-classical"

	sub, err = cl.Subscribe(protocol.ChannelClassical, func(data []byte) {
		go func() {
			var env protocol.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				log.Printf("bob-classical: unmarshal envelope: %v", err)
				return
			}
			if env.From == selfID {
				return
			}
			switch env.Type {
			case protocol.TypeInitialMsg:
				t0 := time.Now()
				var initMsg classical.InitialMessage
				if err := json.Unmarshal(env.Payload, &initMsg); err != nil {
					log.Printf("bob-classical: unmarshal initial_msg: %v", err)
					return
				}
				s, err := classical.ResponderHandshake(priv, initMsg)
				metrics.HandshakeDurationSeconds.Observe(time.Since(t0).Seconds())
				if err != nil {
					log.Printf("bob-classical: ResponderHandshake: %v", err)
					return
				}
				mu.Lock()
				sess = s
				mu.Unlock()
				log.Printf("bob-classical: handshake complete")

			case protocol.TypeRatchetMsg:
				mu.Lock()
				sessNil := sess == nil
				mu.Unlock()
				if sessNil {
					log.Printf("bob-classical: session not yet initialized, dropping ratchet_msg")
					return
				}
				var msg classical.Message
				if err := json.Unmarshal(env.Payload, &msg); err != nil {
					log.Printf("bob-classical: unmarshal ratchet_msg: %v", err)
					return
				}
				log.Printf("bob-classical: DEBUG: recv msg Header.N=%d, RatchetPublicKey=%x", msg.DR.Header.N, msg.DR.Header.RatchetPublicKey[:4])
				mu.Lock()
				t0 := time.Now()
				plain, err := sess.Decrypt(&msg)
				metrics.DecryptDurationSeconds.Observe(time.Since(t0).Seconds())
				metrics.MessageWireBytes.Observe(float64(len(data)))
				mu.Unlock()
				if err != nil {
					log.Printf("bob-classical: Decrypt: %v", err)
					return
				}

				var rp protocol.RatchetPayload
				if err := json.Unmarshal(plain, &rp); err != nil {
					log.Printf("bob-classical: unmarshal ratchet payload: %v", err)
					return
				}
				log.Printf("bob-classical: recv: seq=%d text=%s", rp.Seq, rp.Text)

				echoRP := protocol.RatchetPayload{Seq: rp.Seq, Text: "echo: " + rp.Text}
				echoJSON, err := json.Marshal(echoRP)
				if err != nil {
					log.Printf("bob-classical: marshal echo payload: %v", err)
					return
				}
				mu.Lock()
				t0 = time.Now()
				echoMsg, err := sess.Encrypt(echoJSON)
				metrics.EncryptDurationSeconds.Observe(time.Since(t0).Seconds())
				mu.Unlock()
				if err != nil {
					log.Printf("bob-classical: Encrypt echo: %v", err)
					return
				}
				raw, err := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, selfID, echoMsg)
				if err != nil {
					log.Printf("bob-classical: MarshalEnvelope echo: %v", err)
					return
				}
				metrics.MessageWireBytes.Observe(float64(len(raw)))
				if err := cl.Publish(context.Background(), sub, raw); err != nil {
					log.Printf("bob-classical: Publish echo: %v", err)
					return
				}
				n := atomic.AddInt32(&echoCount, 1)
			if n == msgCount {
				log.Printf("bob-classical: sent %d echoes, exiting", msgCount)
					os.Exit(0)
				}
			}
		}()
	})
	if err != nil {
		log.Fatalf("bob-classical: Subscribe: %v", err)
	}

	raw, err := protocol.MarshalEnvelope(protocol.TypePrekeyBundle, selfID, bundle)
	if err != nil {
		log.Fatalf("bob-classical: MarshalEnvelope prekey_bundle: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := cl.Publish(ctx, sub, raw); err != nil {
		log.Fatalf("bob-classical: Publish prekey_bundle: %v", err)
	}
	log.Printf("bob-classical: published prekey bundle on %s, waiting for Alice", protocol.ChannelClassical)

	select {}
}
