package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	HTTPRequestTotal    *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPInFlight        prometheus.Gauge
}

func MustNew() *Metrics {
	m := &Metrics{
		HTTPRequestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "gateway",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests processed by the gateway.",
			},
			[]string{"method", "route", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "gateway",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),
		HTTPInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "gateway",
				Subsystem: "http",
				Name:      "in_flight_requests",
				Help:      "Current number of in-flight HTTP requests.",
			},
		),
	}

	prometheus.MustRegister(
		m.HTTPRequestTotal,
		m.HTTPRequestDuration,
		m.HTTPInFlight,
	)

	return m
}
