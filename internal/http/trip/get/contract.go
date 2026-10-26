package get

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
)

type TripGetter interface {
	Get(ctx context.Context, tripID string) (model.Trip, error)
}
