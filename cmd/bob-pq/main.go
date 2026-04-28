package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	centrifuge "github.com/centrifugal/centrifuge-go"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/metrics"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/pq"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/protocol"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/transport"
)

func main() {
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		if err := http.ListenAndServe(":9093", mux); err != nil {
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
		echoCount int32
		sub       *centrifuge.Subscription
	)

	sub, err = cl.Subscribe(protocol.ChannelPQ, func(data []byte) {
		go func() {
			var env protocol.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				log.Printf("bob-pq: unmarshal envelope: %v", err)
				return
			}
			switch env.Type {
			case protocol.TypeInitialMsg:
				var initMsg pq.InitialMessage
				if err := json.Unmarshal(env.Payload, &initMsg); err != nil {
					log.Printf("bob-pq: unmarshal initial_msg: %v", err)
					return
				}
				s, err := pq.ResponderHandshake(priv, initMsg)
				if err != nil {
					log.Printf("bob-pq: ResponderHandshake: %v", err)
					return
				}
				sess = s
				log.Printf("bob-pq: handshake complete")

			case protocol.TypeRatchetMsg:
				if sess == nil {
					log.Printf("bob-pq: session not yet initialized, dropping ratchet_msg")
					return
				}
				var msg pq.TripleRatchetMessage
				if err := json.Unmarshal(env.Payload, &msg); err != nil {
					log.Printf("bob-pq: unmarshal ratchet_msg: %v", err)
					return
				}
				plain, err := sess.Decrypt(&msg)
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
				echoMsg, err := sess.Encrypt(echoJSON)
				if err != nil {
					log.Printf("bob-pq: Encrypt echo: %v", err)
					return
				}
				raw, err := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, echoMsg)
				if err != nil {
					log.Printf("bob-pq: MarshalEnvelope echo: %v", err)
					return
				}
				if err := cl.Publish(context.Background(), sub, raw); err != nil {
					log.Printf("bob-pq: Publish echo: %v", err)
					return
				}
				n := atomic.AddInt32(&echoCount, 1)
				if n == 5 {
					log.Printf("bob-pq: sent 5 echoes, exiting")
					os.Exit(0)
				}
			}
		}()
	})
	if err != nil {
		log.Fatalf("bob-pq: Subscribe: %v", err)
	}

	raw, err := protocol.MarshalEnvelope(protocol.TypePrekeyBundle, bundle)
	if err != nil {
		log.Fatalf("bob-pq: MarshalEnvelope prekey_bundle: %v", err)
	}
	if err := cl.Publish(context.Background(), sub, raw); err != nil {
		log.Fatalf("bob-pq: Publish prekey_bundle: %v", err)
	}
	log.Printf("bob-pq: published prekey bundle on %s, waiting for Alice", protocol.ChannelPQ)

	select {}
}
