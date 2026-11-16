package positions

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
)

type PositionUseCase interface {
	Create(ctx context.Context, position model.TripPosition) (int64, error)
	Get(ctx context.Context, tripID string) ([]model.TripPosition, error)
}

type PositionCanceler interface {
	Cancel(tripID string)
}
