package finish

import (
	"context"
	"time"

	"github.com/cench-dev/avito-labs/internal/model"
)

type tripRepository interface {
	FinishTrip(ctx context.Context, tripID string, finishedAt time.Time) error
	CreateTripStatusHistory(ctx context.Context, history model.TripStatusHistory) error
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

//go:generate go run go.uber.org/mock/mockgen -source=contract.go -destination=mocks_test.go -package=finish
