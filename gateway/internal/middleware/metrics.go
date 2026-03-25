package middleware

import (
	"fmt"
	"net/http"
	"time"

	"smol-cluster/gateway/internal/metrics"

	"github.com/go-chi/chi/v5"
)

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher for streaming/SSE support.
func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// PrometheusMetrics returns a chi-compatible middleware that records HTTP
// request count (by method, route pattern, and status code) and request
// duration histogram (by method and route pattern).
func PrometheusMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := newStatusRecorder(w)

		next.ServeHTTP(rec, r)

		duration := time.Since(start).Seconds()

		// Use the chi route pattern when available so high-cardinality path
		// parameters (UUIDs, IDs) are collapsed into a single label value.
		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = r.URL.Path
		}

		status := fmt.Sprintf("%d", rec.statusCode)

		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, routePattern, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, routePattern).Observe(duration)
	})
}
