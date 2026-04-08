package proxy

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	appMiddleware "github.com/Alaghal/go-api-gateway/internal/middleware"
)

type ReverseProxy struct {
	target  *url.URL
	handler *httputil.ReverseProxy
	breaker *CircuitBreaker
	retries int
}

func NewReverseProxy(targetURL string, timeout time.Duration) (*ReverseProxy, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	rp := httputil.NewSingleHostReverseProxy(parsedURL)

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: timeout,
	}

	originalDirector := rp.Director
	rp.Transport = transport

	rp.Director = func(req *http.Request) {
		originalDirector(req)

		requestID := appMiddleware.GetRequestID(req.Context())
		if requestID != "" {
			req.Header.Set(appMiddleware.RequestIDHeader, requestID)
		}

		if req.Header.Get("X-Forwarded-For") == "" && req.RemoteAddr != "" {
			req.Header.Set("X-Forwarded-For", req.RemoteAddr)
		}

		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		requestID := appMiddleware.GetRequestID(r.Context())

		status := http.StatusBadGateway
		if isTimeoutError(err) {
			status = http.StatusGatewayTimeout
		}

		log.Printf(
			"request_id=%s proxy_error path=%s method=%s status=%d err=%v",
			requestID,
			r.URL.Path,
			r.Method,
			status,
			err,
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		if status == http.StatusGatewayTimeout {
			_, _ = w.Write([]byte(`{"error":"upstream request timed out"}`))
			return
		}

		_, _ = w.Write([]byte(`{"error":"upstream service unavailable"}`))
	}

	rp.ModifyResponse = func(resp *http.Response) error {
		requestID := appMiddleware.GetRequestID(resp.Request.Context())
		if requestID != "" {
			resp.Header.Set(appMiddleware.RequestIDHeader, requestID)
		}
		return nil
	}

	breaker := NewCircuitBreaker(5, 10*time.Second)

	return &ReverseProxy{
		target:  parsedURL,
		handler: rp,
		breaker: breaker,
		retries: 2,
	}, nil
}

func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !rp.breaker.Allow() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"service temporarily unavailable"}`))
		return
	}

	type result struct {
		err        error
		statusCode int
	}

	for i := 0; i <= rp.retries; i++ {
		done := make(chan result, 1)

		rec := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			mu:             &sync.Mutex{},
		}

		go func() {
			rp.handler.ServeHTTP(rec, r)
			done <- result{err: nil, statusCode: rec.statusCode}
		}()

		select {
		case res := <-done:
			if res.err == nil && res.statusCode < 500 {
				rp.breaker.Success()
				return
			}

		case <-time.After(2 * time.Second):
			rp.breaker.Failure()
			if i == rp.retries {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusGatewayTimeout)
				_, _ = w.Write([]byte(`{"error":"upstream request timed out"}`))
				return
			}
			continue
		}
	}

	rp.breaker.Failure()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = w.Write([]byte(`{"error":"upstream failed after retries"}`))
}

func (rp *ReverseProxy) Target() string {
	return rp.target.String()
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	mu         *sync.Mutex
}

func (rec *responseRecorder) WriteHeader(code int) {
	rec.mu.Lock()
	rec.statusCode = code
	rec.mu.Unlock()
	rec.ResponseWriter.WriteHeader(code)
}
