package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

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
		if err := http.ListenAndServe(":9090", mux); err != nil {
			log.Printf("alice-classical: metrics server: %v", err)
		}
	}()

	url := os.Getenv("CENTRIFUGO_URL")
	if url == "" {
		url = "ws://localhost:8000/connection/websocket"
	}

	cl := transport.NewClient(url)
	if err := cl.Connect(); err != nil {
		log.Fatalf("alice-classical: Connect: %v", err)
	}
	defer cl.Disconnect()

	bundleCh := make(chan []byte, 1)

	var (
		sess      *classical.Session
		recvCount int32
		sub       *centrifuge.Subscription
		err       error
	)

	sub, err = cl.Subscribe(protocol.ChannelClassical, func(data []byte) {
		go func() {
			var env protocol.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				log.Printf("alice-classical: unmarshal envelope: %v", err)
				return
			}
			switch env.Type {
			case protocol.TypePrekeyBundle:
				select {
				case bundleCh <- env.Payload:
				default:
					log.Printf("alice-classical: duplicate prekey_bundle dropped")
				}
			case protocol.TypeRatchetMsg:
				if sess == nil {
					log.Printf("alice-classical: session not yet initialized, dropping ratchet_msg")
					return
				}
				var msg classical.Message
				if err := json.Unmarshal(env.Payload, &msg); err != nil {
					log.Printf("alice-classical: unmarshal ratchet_msg: %v", err)
					return
				}
				plain, err := sess.Decrypt(&msg)
				if err != nil {
					log.Printf("alice-classical: Decrypt echo: %v", err)
					return
				}
				var rp protocol.RatchetPayload
				if err := json.Unmarshal(plain, &rp); err != nil {
					log.Printf("alice-classical: unmarshal echo payload: %v", err)
					return
				}
				log.Printf("alice-classical: recv echo: seq=%d text=%s", rp.Seq, rp.Text)
				n := atomic.AddInt32(&recvCount, 1)
				if n == 5 {
					log.Printf("alice-classical: received 5 echoes, exiting")
					os.Exit(0)
				}
			}
		}()
	})
	if err != nil {
		log.Fatalf("alice-classical: Subscribe: %v", err)
	}

	select {
	case payload := <-bundleCh:
		var bundle classical.PrekeyBundle
		if err := json.Unmarshal(payload, &bundle); err != nil {
			log.Fatalf("alice-classical: unmarshal PrekeyBundle: %v", err)
		}

		s, initMsg, err := classical.InitiatorHandshake(&bundle)
		if err != nil {
			log.Fatalf("alice-classical: InitiatorHandshake: %v", err)
		}
		sess = s
		log.Printf("alice-classical: handshake complete")

		initRaw, err := protocol.MarshalEnvelope(protocol.TypeInitialMsg, initMsg)
		if err != nil {
			log.Fatalf("alice-classical: MarshalEnvelope initial_msg: %v", err)
		}
		if err := cl.Publish(context.Background(), sub, initRaw); err != nil {
			log.Fatalf("alice-classical: Publish initial_msg: %v", err)
		}

		for i := 1; i <= 5; i++ {
			rp := protocol.RatchetPayload{
				Seq:  i,
				Text: fmt.Sprintf("hello from alice-classical %d", i),
			}
			payloadJSON, err := json.Marshal(rp)
			if err != nil {
				log.Fatalf("alice-classical: Marshal ratchetPayload %d: %v", i, err)
			}
			msg, err := sess.Encrypt(payloadJSON)
			if err != nil {
				log.Fatalf("alice-classical: Encrypt msg %d: %v", i, err)
			}
			raw, err := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, msg)
			if err != nil {
				log.Fatalf("alice-classical: MarshalEnvelope ratchet_msg %d: %v", i, err)
			}
			if err := cl.Publish(context.Background(), sub, raw); err != nil {
				log.Fatalf("alice-classical: Publish ratchet_msg %d: %v", i, err)
			}
			log.Printf("alice-classical: sent ratchet_msg seq=%d", i)
		}

	case <-time.After(30 * time.Second):
		log.Fatalf("alice-classical: timed out waiting for Bob's prekey bundle on %s", protocol.ChannelClassical)
	}

	select {}
}
