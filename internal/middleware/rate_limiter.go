package middleware

import (
	"net/http"
	"time"

	"github.com/wallet-ledger/internal/redis"
)

// RateLimiter returns a middleware that rate-limits requests by IP.
func RateLimiter(rdb *redis.Client, requestsPerMinute int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract IP
			ip := r.Header.Get("X-Real-IP")
			if ip == "" {
				ip = r.Header.Get("X-Forwarded-For")
			}
			if ip == "" {
				ip = r.RemoteAddr
			}

			key := "ratelimit:ip:" + ip
			err := rdb.Allow(r.Context(), key, requestsPerMinute, time.Minute)
			if err != nil {
				if err == redis.ErrRateLimitExceeded {
					http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
					return
				}
				// If Redis fails, we can choose to fail-open or fail-closed. 
				// For high availability, fail-open is generally preferred unless under active DDoS.
				// We'll log the error and allow the request to proceed (fail-open).
			}

			next.ServeHTTP(w, r)
		})
	}
}
