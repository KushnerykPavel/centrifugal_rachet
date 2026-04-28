package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	centrifuge "github.com/centrifugal/centrifuge-go"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/classical"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/metrics"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/protocol"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/transport"
)

func main() {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9091"
	}
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		if err := http.ListenAndServe(":"+port, mux); err != nil {
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
		mu        sync.Mutex // guards sess, Encrypt, Decrypt
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
				mu.Lock()
				sessNil := sess == nil
				mu.Unlock()
				if sessNil {
					log.Printf("alice-classical: session not yet initialized, dropping ratchet_msg")
					return
				}
				var msg classical.Message
				if err := json.Unmarshal(env.Payload, &msg); err != nil {
					log.Printf("alice-classical: unmarshal ratchet_msg: %v", err)
					return
				}
				mu.Lock()
				t0 := time.Now()
				plain, err := sess.Decrypt(&msg)
				metrics.DecryptDurationSeconds.Observe(time.Since(t0).Seconds())
				metrics.MessageWireBytes.Observe(float64(len(data)))
				mu.Unlock()
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

		t0 := time.Now()
		s, initMsg, err := classical.InitiatorHandshake(&bundle)
		metrics.HandshakeDurationSeconds.Observe(time.Since(t0).Seconds())
		if err != nil {
			log.Fatalf("alice-classical: InitiatorHandshake: %v", err)
		}
		mu.Lock()
		sess = s
		mu.Unlock()
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
			mu.Lock()
			t0 := time.Now()
			msg, err := sess.Encrypt(payloadJSON)
			metrics.EncryptDurationSeconds.Observe(time.Since(t0).Seconds())
			mu.Unlock()
			if err != nil {
				log.Fatalf("alice-classical: Encrypt msg %d: %v", i, err)
			}
			raw, err := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, msg)
			if err != nil {
				log.Fatalf("alice-classical: MarshalEnvelope ratchet_msg %d: %v", i, err)
			}
			metrics.MessageWireBytes.Observe(float64(len(raw)))
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
