package middleware

import (
	"net/http"
	"strconv"
	"time"

	appMetrics "github.com/Alaghal/go-api-gateway/internal/metrics"
	"github.com/go-chi/chi/v5"
)

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *metricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func Metrics(m *appMetrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			m.HTTPInFlight.Inc()
			defer m.HTTPInFlight.Dec()

			rw := &metricsResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rw, r)

			duration := time.Since(start).Seconds()
			route := routePattern(r)

			labels := []string{
				r.Method,
				route,
				strconv.Itoa(rw.statusCode),
			}

			m.HTTPRequestTotal.WithLabelValues(labels...).Inc()
			m.HTTPRequestDuration.WithLabelValues(labels...).Observe(duration)
		})
	}
}

func routePattern(r *http.Request) string {
	routeContext := chi.RouteContext(r.Context())
	if routeContext == nil {
		return "unknown"
	}

	pattern := routeContext.RoutePattern()
	if pattern == "" {
		return "unknown"
	}

	return pattern
}
