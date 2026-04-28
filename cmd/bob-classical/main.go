package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	centrifuge "github.com/centrifugal/centrifuge-go"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/classical"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/metrics"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/protocol"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/transport"
)

func main() {
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		if err := http.ListenAndServe(":9091", mux); err != nil {
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
		echoCount int32
		sub       *centrifuge.Subscription
	)

	sub, err = cl.Subscribe(protocol.ChannelClassical, func(data []byte) {
		go func() {
			var env protocol.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				log.Printf("bob-classical: unmarshal envelope: %v", err)
				return
			}
			switch env.Type {
			case protocol.TypeInitialMsg:
				var initMsg classical.InitialMessage
				if err := json.Unmarshal(env.Payload, &initMsg); err != nil {
					log.Printf("bob-classical: unmarshal initial_msg: %v", err)
					return
				}
				s, err := classical.ResponderHandshake(priv, initMsg)
				if err != nil {
					log.Printf("bob-classical: ResponderHandshake: %v", err)
					return
				}
				sess = s
				log.Printf("bob-classical: handshake complete")

			case protocol.TypeRatchetMsg:
				if sess == nil {
					log.Printf("bob-classical: session not yet initialized, dropping ratchet_msg")
					return
				}
				var msg classical.Message
				if err := json.Unmarshal(env.Payload, &msg); err != nil {
					log.Printf("bob-classical: unmarshal ratchet_msg: %v", err)
					return
				}
				plain, err := sess.Decrypt(&msg)
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
				echoMsg, err := sess.Encrypt(echoJSON)
				if err != nil {
					log.Printf("bob-classical: Encrypt echo: %v", err)
					return
				}
				raw, err := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, echoMsg)
				if err != nil {
					log.Printf("bob-classical: MarshalEnvelope echo: %v", err)
					return
				}
				if err := cl.Publish(context.Background(), sub, raw); err != nil {
					log.Printf("bob-classical: Publish echo: %v", err)
					return
				}
				n := atomic.AddInt32(&echoCount, 1)
				if n == 5 {
					log.Printf("bob-classical: sent 5 echoes, exiting")
					os.Exit(0)
				}
			}
		}()
	})
	if err != nil {
		log.Fatalf("bob-classical: Subscribe: %v", err)
	}

	raw, err := protocol.MarshalEnvelope(protocol.TypePrekeyBundle, bundle)
	if err != nil {
		log.Fatalf("bob-classical: MarshalEnvelope prekey_bundle: %v", err)
	}
	if err := cl.Publish(context.Background(), sub, raw); err != nil {
		log.Fatalf("bob-classical: Publish prekey_bundle: %v", err)
	}
	log.Printf("bob-classical: published prekey bundle on %s, waiting for Alice", protocol.ChannelClassical)

	select {}
}
