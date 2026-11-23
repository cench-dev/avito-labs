package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type PositionMetrics struct {
	staleChecks      metric.Int64Counter
	queueSize        metric.Int64UpDownCounter
	activeWorkers    metric.Int64UpDownCounter
	positionRequests metric.Int64Counter
	requestDuration  metric.Float64Histogram
	positionsSaved   metric.Int64Counter
}

func NewPositionMetrics(provider metric.MeterProvider) (*PositionMetrics, error) {
	meter := provider.Meter("trip-service/positions")

	staleChecks, err := meter.Int64Counter(
		"tripgo.stale_checks",
		metric.WithDescription("Number of stale trip checks by result"),
	)
	if err != nil {
		return nil, fmt.Errorf("create stale checks counter: %w", err)
	}

	queueSize, err := meter.Int64UpDownCounter(
		"tripgo.position_requests.queue.size",
		metric.WithDescription("Current number of queued position requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create position queue size metric: %w", err)
	}

	activeWorkers, err := meter.Int64UpDownCounter(
		"tripgo.position_requests.workers.active",
		metric.WithDescription("Number of currently active workers"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers metric: %w", err)
	}

	positionRequests, err := meter.Int64Counter(
		"tripgo.position_requests",
		metric.WithDescription("Number of position push requests by result"),
	)
	if err != nil {
		return nil, fmt.Errorf("create position requests counter: %w", err)
	}

	requestDuration, err := meter.Float64Histogram(
		"tripgo.position_requests.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of position push requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create position request duration histogram: %w", err)
	}

	positionsSaved, err := meter.Int64Counter(
		"tripgo.positions.saved",
		metric.WithDescription("Number of successfully saved positions"),
	)
	if err != nil {
		return nil, fmt.Errorf("create positions saved counter: %w", err)
	}

	return &PositionMetrics{
		staleChecks:      staleChecks,
		queueSize:        queueSize,
		activeWorkers:    activeWorkers,
		positionRequests: positionRequests,
		requestDuration:  requestDuration,
		positionsSaved:   positionsSaved,
	}, nil
}

func (m *PositionMetrics) RecordStaleCheck(ctx context.Context, result string) {
	m.staleChecks.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", result),
	))
}

func (m *PositionMetrics) AddQueueSize(ctx context.Context, delta int64) {
	m.queueSize.Add(ctx, delta)
}

func (m *PositionMetrics) AddActiveWorkers(ctx context.Context, delta int64) {
	m.activeWorkers.Add(ctx, delta)
}

func (m *PositionMetrics) RecordPositionRequest(ctx context.Context, result string, duration time.Duration) {
	m.RecordPositionRequestResult(ctx, result)
	m.requestDuration.Record(ctx, duration.Seconds())
}

func (m *PositionMetrics) RecordPositionSaved(ctx context.Context) {
	m.positionsSaved.Add(ctx, 1)
}

func (m *PositionMetrics) RecordPositionRequestResult(ctx context.Context, result string) {
	m.positionRequests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", result),
	))
}
