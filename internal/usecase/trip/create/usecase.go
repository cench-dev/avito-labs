package create

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
	"github.com/cench-dev/avito-labs/internal/observability"
)

type UseCase struct {
	tripRepository tripRepository
	txManager      txManager
}

func NewUseCase(repo tripRepository, manager txManager) *UseCase {
	return &UseCase{
		tripRepository: repo,
		txManager:      manager,
	}
}

func (uc *UseCase) Create(ctx context.Context, trip model.Trip) error {
	res := uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.tripRepository.CreateTrip(ctx, trip); err != nil {
			return err
		}
		tripHistory := model.TripStatusHistory{
			TripID:     trip.ID,
			FromStatus: nil,
			ToStatus:   trip.Status,
			Reason:     nil,
		}

		if err := uc.tripRepository.CreateTripStatusHistory(ctx, tripHistory); err != nil {
			return err
		}
		return nil
	})

	if res != nil {
		return res
	}

	observability.RecordTripCreated(ctx)
	return nil
}
