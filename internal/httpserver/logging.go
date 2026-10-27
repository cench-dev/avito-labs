package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func RequestLogging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &responseRecorder{ResponseWriter: w}

			defer func() {
				if recovered := recover(); recovered != nil {
					recorder.status = http.StatusInternalServerError
					logRequest(logger, r, recorder.status, time.Since(start))
					panic(recovered)
				}

				if recorder.status == 0 {
					recorder.status = http.StatusOK
				}
				logRequest(logger, r, recorder.status, time.Since(start))
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

func logRequest(logger *slog.Logger, r *http.Request, status int, duration time.Duration) {
	spanContext := trace.SpanContextFromContext(r.Context())

	attrs := []any{
		"service", "trip-service",
		"method", r.Method,
		"route", routePattern(r),
		"status", status,
		"duration_ms", float64(duration.Microseconds()) / 1000,
		"trace_id", "",
		"span_id", "",
	}

	if spanContext.IsValid() {
		attrs[11] = spanContext.TraceID().String()
		attrs[13] = spanContext.SpanID().String()
	}

	if status >= 500 {
		logger.Error("request handled", attrs...)
	} else {
		logger.Info("request handled", attrs...)
	}
}
