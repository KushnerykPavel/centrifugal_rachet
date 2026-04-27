package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Reg is the custom Prometheus registry used by all four binaries.
// Phase 4 calls Observe() on the exported histogram vars below.
var Reg *prometheus.Registry

// Pre-registered histograms — all names are final (per D-03).
// Phase 4 only calls .Observe(); no edits to this package needed.
var (
	MessageWireBytes         prometheus.Histogram
	HandshakeDurationSeconds prometheus.Histogram
	EncryptDurationSeconds   prometheus.Histogram
	DecryptDurationSeconds   prometheus.Histogram
)

func init() {
	Reg = prometheus.NewRegistry()

	MessageWireBytes = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ratchet_message_wire_bytes",
		Help:    "Wire size of serialized ratchet messages in bytes.",
		Buckets: prometheus.DefBuckets,
	})
	HandshakeDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ratchet_handshake_duration_seconds",
		Help:    "Wall time of full key exchange and session initialisation in seconds.",
		Buckets: prometheus.DefBuckets,
	})
	EncryptDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ratchet_encrypt_duration_seconds",
		Help:    "Wall time of ratchet Encrypt call in seconds.",
		Buckets: prometheus.DefBuckets,
	})
	DecryptDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ratchet_decrypt_duration_seconds",
		Help:    "Wall time of ratchet Decrypt call in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	Reg.MustRegister(
		MessageWireBytes,
		HandshakeDurationSeconds,
		EncryptDurationSeconds,
		DecryptDurationSeconds,
	)
}

// Handler returns an http.Handler serving the custom Prometheus registry.
// Mount this on "/metrics" in each binary. Per D-03, no label differences
// between binaries — Prometheus scrape config attaches protocol/role labels.
func Handler() http.Handler {
	return promhttp.HandlerFor(Reg, promhttp.HandlerOpts{})
}
