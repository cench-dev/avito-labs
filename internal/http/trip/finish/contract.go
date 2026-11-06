package finish

import (
	"context"
	"time"

	"github.com/cench-dev/avito-labs/internal/model"
)

type TripFinisher interface {
	Finish(ctx context.Context, tripID string, finishedAt time.Time) error
}

type TripGetter interface {
	Get(ctx context.Context, tripID string) (model.Trip, error)
}

//go:generate go run go.uber.org/mock/mockgen -source=contract.go -destination=mocks_test.go -package=finish
