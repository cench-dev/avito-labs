package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	businessMeter = otel.Meter("trip-service/business")

	tripsCreated, _ = businessMeter.Int64Counter(
		"tripgo.trips.created",
		metric.WithDescription("Number of successfully created trips"),
	)

	tripsCompleted, _ = businessMeter.Int64Counter(
		"tripgo.trips.completed",
		metric.WithDescription("Number of successfully completed trips"),
	)
)

func RecordTripCreated(ctx context.Context) {
	tripsCreated.Add(ctx, 1)
}

func RecordTripCompleted(ctx context.Context) {
	tripsCompleted.Add(ctx, 1)
}
