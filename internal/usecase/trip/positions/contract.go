package positions

import (
	"context"
	"time"

	"github.com/cench-dev/avito-labs/internal/model"
)

type tripRepository interface {
	LockActiveTrip(ctx context.Context, id string) error
	CreateTripPosition(ctx context.Context, position model.TripPosition) (int64, error)
	UpdateLastPositionAt(ctx context.Context, tripID string, recordedAt time.Time) error
	GetTripPositions(ctx context.Context, tripID string) ([]model.TripPosition, error)
	GetTrip(ctx context.Context, id string) (model.Trip, error)
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
