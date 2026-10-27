package get

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
)

type tripRepository interface {
	GetTrip(ctx context.Context, tripID string) (model.Trip, error)
}

//go:generate go run go.uber.org/mock/mockgen -source=contract.go -destination=mocks_test.go -package=get
