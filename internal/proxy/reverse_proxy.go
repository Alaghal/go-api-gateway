package proxy

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
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

	for i := 0; i <= rp.retries; i++ {
		rec := httptest.NewRecorder()
		rp.handler.ServeHTTP(rec, r)

		if rec.Code < 500 {
			rp.breaker.Success()
			for k, v := range rec.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())
			return
		}

		rp.breaker.Failure()
		if i < rp.retries {
			time.Sleep(100 * time.Millisecond) // Simple backoff
		}
	}

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
