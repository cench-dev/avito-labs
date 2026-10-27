package httpserver

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	httpMeter = otel.Meter("trip-service/http")

	requestDuration, _ = httpMeter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests"),
	)

	activeRequests, _ = httpMeter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
	)
)

func RequestMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		recorder := &responseRecorder{
			ResponseWriter: w,
		}

		activeRequests.Add(r.Context(), 1)
		defer activeRequests.Add(r.Context(), -1)

		next.ServeHTTP(recorder, r)

		route := routePattern(r)
		if route == "" {
			route = "unmatched"
		}

		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}

		attrs := metric.WithAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", recorder.status),
		)

		requestDuration.Record(
			r.Context(),
			time.Since(start).Seconds(),
			attrs,
		)
	})
}

func routePattern(r *http.Request) string {
	routeContext := chi.RouteContext(r.Context())
	if routeContext == nil {
		return ""
	}
	return routeContext.RoutePattern()
}
