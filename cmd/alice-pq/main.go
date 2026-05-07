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

	"github.com/KushnerykPavel/go-doubleratchet/pqxdh"
	centrifuge "github.com/centrifugal/centrifuge-go"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/metrics"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/pq"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/protocol"
	"github.com/KushnerykPavel/centrifugal-ratchet/internal/transport"
)

const (
	msgCount  = 10000
	timeout   = 10 * time.Minute
	sendDelay = 20 * time.Millisecond
)

func main() {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9093"
	}
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			log.Printf("alice-pq: metrics server: %v", err)
		}
	}()

	url := os.Getenv("CENTRIFUGO_URL")
	if url == "" {
		url = "ws://localhost:8000/connection/websocket"
	}

	cl := transport.NewClient(url)
	if err := cl.Connect(); err != nil {
		log.Fatalf("alice-pq: Connect: %v", err)
	}
	defer cl.Disconnect()

	bundleCh := make(chan []byte, 1)

	var (
		sess      *pq.Session
		mu        sync.Mutex // guards sess, Encrypt, Decrypt
		recvCount int32
		sub       *centrifuge.Subscription
		err       error
	)

	const selfID = "alice-pq"

	sub, err = cl.Subscribe(protocol.ChannelPQ, func(data []byte) {
		go func() {
			var env protocol.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				log.Printf("alice-pq: unmarshal envelope: %v", err)
				return
			}
			if env.From == selfID {
				return
			}
			switch env.Type {
			case protocol.TypePrekeyBundle:
				select {
				case bundleCh <- env.Payload:
				default:
					log.Printf("alice-pq: duplicate prekey_bundle dropped")
				}
			case protocol.TypeRatchetMsg:
				mu.Lock()
				sessNil := sess == nil
				mu.Unlock()
				if sessNil {
					log.Printf("alice-pq: session not yet initialized, dropping ratchet_msg")
					return
				}
				var msg pq.TripleRatchetMessage
				if err := json.Unmarshal(env.Payload, &msg); err != nil {
					log.Printf("alice-pq: unmarshal ratchet_msg: %v", err)
					return
				}
				mu.Lock()
				t0 := time.Now()
				plain, err := sess.Decrypt(&msg)
				metrics.DecryptDurationSeconds.Observe(time.Since(t0).Seconds())
				metrics.MessageWireBytes.Observe(float64(len(data)))
				mu.Unlock()
				if err != nil {
					log.Printf("alice-pq: Decrypt echo: %v", err)
					return
				}
				// Unmarshal echo as RatchetPayload to log structured fields (D-05).
				var rp protocol.RatchetPayload
				if err := json.Unmarshal(plain, &rp); err != nil {
					log.Printf("alice-pq: unmarshal echo payload: %v", err)
					return
				}
				log.Printf("alice-pq: recv echo: seq=%d text=%s", rp.Seq, rp.Text)
				n := atomic.AddInt32(&recvCount, 1)
			if n == msgCount {
				log.Printf("alice-pq: received %d echoes, exiting", msgCount)
					os.Exit(0)
				}
			}
		}()
	})
	if err != nil {
		log.Fatalf("alice-pq: Subscribe: %v", err)
	}

	select {
	case payload := <-bundleCh:
		// IMPORTANT: unmarshal into pqxdh.PrekeyBundle (not a custom struct) — Pitfall 6.
		// pqxdh.PrekeyBundle has *[32]byte and *uint32 pointer fields that round-trip
		// correctly through encoding/json.
		var bundle pqxdh.PrekeyBundle
		if err := json.Unmarshal(payload, &bundle); err != nil {
			log.Fatalf("alice-pq: unmarshal PrekeyBundle: %v", err)
		}

		// Perform PQXDH handshake (Alice side).
		t0 := time.Now()
		s, initMsg, err := pq.InitiatorHandshake(&bundle)
		metrics.HandshakeDurationSeconds.Observe(time.Since(t0).Seconds())
		if err != nil {
			log.Fatalf("alice-pq: InitiatorHandshake: %v", err)
		}
		mu.Lock()
		sess = s
		mu.Unlock()
		log.Printf("alice-pq: handshake complete")

		// Publish initial message to Bob.
		initRaw, err := protocol.MarshalEnvelope(protocol.TypeInitialMsg, selfID, initMsg)
		if err != nil {
			log.Fatalf("alice-pq: MarshalEnvelope initial_msg: %v", err)
		}
		if err := cl.Publish(context.Background(), sub, initRaw); err != nil {
			log.Fatalf("alice-pq: Publish initial_msg: %v", err)
		}

		// Send ratchet messages with RatchetPayload JSON (D-04, D-05).
		for i := 1; i <= msgCount; i++ {
			rp := protocol.RatchetPayload{
				Seq:  i,
				Text: fmt.Sprintf("hello from alice-pq %d", i),
			}
			payloadJSON, err := json.Marshal(rp)
			if err != nil {
				log.Fatalf("alice-pq: Marshal ratchetPayload %d: %v", i, err)
			}
			mu.Lock()
			t0 := time.Now()
			msg, err := sess.Encrypt(payloadJSON)
			metrics.EncryptDurationSeconds.Observe(time.Since(t0).Seconds())
			mu.Unlock()
			if err != nil {
				log.Fatalf("alice-pq: Encrypt msg %d: %v", i, err)
			}
			raw, err := protocol.MarshalEnvelope(protocol.TypeRatchetMsg, selfID, msg)
			if err != nil {
				log.Fatalf("alice-pq: MarshalEnvelope ratchet_msg %d: %v", i, err)
			}
			metrics.MessageWireBytes.Observe(float64(len(raw)))
			if err := cl.Publish(context.Background(), sub, raw); err != nil {
				log.Fatalf("alice-pq: Publish ratchet_msg %d: %v", i, err)
			}
			log.Printf("alice-pq: sent ratchet_msg seq=%d", i)
			time.Sleep(sendDelay)
		}

	case <-time.After(timeout):
		log.Fatalf("alice-pq: timed out waiting for Bob's prekey bundle on %s", protocol.ChannelPQ)
	}

	select {}
}
