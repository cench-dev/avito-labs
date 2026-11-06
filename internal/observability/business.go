package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type BusinessMetrics struct {
	tripsCreated   metric.Int64Counter
	tripsCompleted metric.Int64Counter
}

func NewBusinessMetrics(meterProvider metric.MeterProvider) (*BusinessMetrics, error) {
	meter := meterProvider.Meter("trip-service/business")

	tripsCreated, err := meter.Int64Counter(
		"tripgo.trips.created",
		metric.WithDescription("Number of successfully created trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("create trips created counter: %w", err)
	}

	tripsCompleted, err := meter.Int64Counter(
		"tripgo.trips.completed",
		metric.WithDescription("Number of successfully completed trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("create trips completed counter: %w", err)
	}

	return &BusinessMetrics{
		tripsCreated:   tripsCreated,
		tripsCompleted: tripsCompleted,
	}, nil
}

func (m *BusinessMetrics) RecordTripCreated(ctx context.Context) {
	m.tripsCreated.Add(ctx, 1)
}

func (m *BusinessMetrics) RecordTripCompleted(ctx context.Context) {
	m.tripsCompleted.Add(ctx, 1)
}

type HTTPMetrics struct {
	requestDuration metric.Float64Histogram
	activeRequests  metric.Int64UpDownCounter
}

func NewHTTPMetrics(meterProvider metric.MeterProvider) (*HTTPMetrics, error) {
	meter := meterProvider.Meter("trip-service/http")

	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP request duration histogram: %w", err)
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP active requests counter: %w", err)
	}

	return &HTTPMetrics{
		requestDuration: requestDuration,
		activeRequests:  activeRequests,
	}, nil
}

func (m *HTTPMetrics) AddActiveRequest(ctx context.Context, delta int64) {
	m.activeRequests.Add(ctx, delta)
}

func (m *HTTPMetrics) RecordRequestDuration(
	ctx context.Context,
	duration time.Duration,
	method string,
	route string,
	status int,
) {
	m.requestDuration.Record(
		ctx,
		duration.Seconds(),
		metric.WithAttributes(
			attribute.String("http.request.method", method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		),
	)
}
