package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReverseProxy(t *testing.T) {
	t.Run("proxy success", func(t *testing.T) {
		// Mock upstream server
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"status":"ok"}`))
			assert.NoError(t, err)
		}))
		defer backend.Close()

		rp, err := NewReverseProxy(backend.URL, 1*time.Second)
		assert.NoError(t, err)

		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()

		rp.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ok")
		assert.Equal(t, StateClosed, rp.breaker.state)
	})

	t.Run("circuit breaker trips on consecutive failures", func(t *testing.T) {
		// Mock upstream server that fails
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer backend.Close()

		rp, err := NewReverseProxy(backend.URL, 1*time.Second)
		assert.NoError(t, err)

		// Set threshold low for testing
		rp.breaker = NewCircuitBreaker(2, 10*time.Second)
		rp.retries = 0 // No retries for simplicity

		// First failure
		req1 := httptest.NewRequest("GET", "/", nil)
		w1 := httptest.NewRecorder()
		rp.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusBadGateway, w1.Code)
		assert.Equal(t, StateClosed, rp.breaker.state)

		// Second failure trips the breaker
		req2 := httptest.NewRequest("GET", "/", nil)
		w2 := httptest.NewRecorder()
		rp.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusBadGateway, w2.Code)
		assert.Equal(t, StateOpen, rp.breaker.state)

		// Third request blocked by breaker
		req3 := httptest.NewRequest("GET", "/", nil)
		w3 := httptest.NewRecorder()
		rp.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusServiceUnavailable, w3.Code)

		var resp map[string]string
		err = json.Unmarshal(w3.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "service temporarily unavailable", resp["error"])
	})

	t.Run("retries on upstream error", func(t *testing.T) {
		callCount := 0
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer backend.Close()

		rp, err := NewReverseProxy(backend.URL, 1*time.Second)
		assert.NoError(t, err)
		rp.retries = 1

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		rp.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 2, callCount) // 1 initial + 1 retry
	})
}
