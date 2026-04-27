package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KushnerykPavel/centrifugal-ratchet/internal/metrics"
)

func TestMetricsHandler_Status200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMetricsHandler_ContainsHistograms(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(w, req)
	body, _ := io.ReadAll(w.Body)
	bodyStr := string(body)

	want := []string{
		"ratchet_message_wire_bytes",
		"ratchet_handshake_duration_seconds",
		"ratchet_encrypt_duration_seconds",
		"ratchet_decrypt_duration_seconds",
	}
	for _, name := range want {
		if !strings.Contains(bodyStr, name) {
			t.Errorf("metrics output missing histogram %q", name)
		}
	}
}

func TestMetricsHandler_CustomRegistry(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(w, req)
	body, _ := io.ReadAll(w.Body)
	if strings.Contains(string(body), "go_goroutines") {
		t.Error("metrics output contains go_goroutines — handler is using the global registry instead of custom Reg")
	}
}
