package create

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
)

type TripCreator interface {
	Create(ctx context.Context, trip model.Trip) error
}

//go:generate go run go.uber.org/mock/mockgen -source=contract.go -destination=mocks_test.go -package=create
