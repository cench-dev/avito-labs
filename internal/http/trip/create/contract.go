package create

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
)

type TripCreator interface {
	Create(ctx context.Context, trip model.Trip) error
}
