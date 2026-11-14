package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type RequestMetricsRecorder interface {
	AddActiveRequest(ctx context.Context, delta int64)
	RecordRequestDuration(
		ctx context.Context,
		duration time.Duration,
		method string,
		route string,
		status int,
	)
}

func RequestMetrics(metrics RequestMetricsRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			recorder := &responseRecorder{
				ResponseWriter: w,
			}

			metrics.AddActiveRequest(r.Context(), 1)
			defer metrics.AddActiveRequest(r.Context(), -1)

			next.ServeHTTP(recorder, r)

			route := routePattern(r)
			if route == "" {
				route = "unmatched"
			}

			if recorder.status == 0 {
				recorder.status = http.StatusOK
			}

			metrics.RecordRequestDuration(
				r.Context(),
				time.Since(start),
				r.Method,
				route,
				recorder.status,
			)
		})
	}
}

func routePattern(r *http.Request) string {
	routeContext := chi.RouteContext(r.Context())
	if routeContext == nil {
		return ""
	}
	return routeContext.RoutePattern()
}
