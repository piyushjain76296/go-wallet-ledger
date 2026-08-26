package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed, partitioned by status code and HTTP method.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Latency of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Business Metrics
	TransfersTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transfers_total",
			Help: "Total number of transfer attempts partitioned by status (success/failure).",
		},
		[]string{"status"},
	)

	PaymentsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_total",
			Help: "Total number of payments partitioned by status.",
		},
		[]string{"status"},
	)
)

func RecordTransfer(success bool) {
	if success {
		TransfersTotal.WithLabelValues("success").Inc()
	} else {
		TransfersTotal.WithLabelValues("failure").Inc()
	}
}

func RecordPayment(status string) {
	PaymentsTotal.WithLabelValues(status).Inc()
}
