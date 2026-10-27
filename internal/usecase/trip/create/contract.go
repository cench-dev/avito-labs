package create

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
)

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type tripRepository interface {
	CreateTrip(ctx context.Context, params model.Trip) error
	CreateTripStatusHistory(ctx context.Context, params model.TripStatusHistory) error
}

//go:generate go run go.uber.org/mock/mockgen -source=contract.go -destination=mocks_test.go -package=create
