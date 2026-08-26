package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/wallet-ledger/internal/observability"
)

// MetricsMiddleware tracks HTTP request latencies and status codes.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Use Chi's WrapResponseWriter to get the final status code
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()

		// Get the registered route pattern (e.g. /api/v1/users/{id}) instead of raw URL path
		// to avoid exploding the cardinality of metrics.
		routeContext := chi.RouteContext(r.Context())
		path := r.URL.Path
		if routeContext != nil && routeContext.RoutePattern() != "" {
			path = routeContext.RoutePattern()
		}

		status := strconv.Itoa(ww.Status())
		method := r.Method

		observability.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		observability.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	})
}
