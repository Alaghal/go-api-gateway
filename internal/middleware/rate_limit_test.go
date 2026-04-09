package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	t.Run("allow requests within limit", func(t *testing.T) {
		rl := NewRateLimiter(10, 5)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := rl.Middleware(next)

		for range 5 {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.1:1234"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("block requests exceeding limit", func(t *testing.T) {
		rl := NewRateLimiter(1, 1) // 1 RPS, burst 1
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := rl.Middleware(next)

		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.2:1234"

		// First request allowed
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request blocked
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)

		var resp rateLimitResponse
		err := json.NewDecoder(w2.Body).Decode(&resp)
		assert.NoError(t, err)
		assert.Equal(t, "rate limit exceeded", resp.Error)
	})

	t.Run("separate limits for different IPs", func(t *testing.T) {
		rl := NewRateLimiter(1, 1)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := rl.Middleware(next)

		// First IP
		req1 := httptest.NewRequest("GET", "/", nil)
		req1.RemoteAddr = "192.168.1.3:1234"
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second IP should still be allowed even if first is limited
		req2 := httptest.NewRequest("GET", "/", nil)
		req2.RemoteAddr = "192.168.1.4:1234"
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{
			name:     "from remote addr",
			remote:   "192.168.1.1:1234",
			expected: "192.168.1.1",
		},
		{
			name:     "from x-forwarded-for",
			headers:  map[string]string{"X-Forwarded-For": "203.0.113.195, 70.41.3.18"},
			expected: "203.0.113.195",
		},
		{
			name:     "from x-real-ip",
			headers:  map[string]string{"X-Real-IP": "203.0.113.195"},
			expected: "203.0.113.195",
		},
		{
			name:     "prefer x-forwarded-for over x-real-ip",
			headers:  map[string]string{"X-Forwarded-For": "1.1.1.1", "X-Real-IP": "2.2.2.2"},
			expected: "1.1.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			assert.Equal(t, tt.expected, extractClientIP(req))
		})
	}
}
