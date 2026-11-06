package create

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
)

type UseCase struct {
	tripRepository tripRepository
	txManager      txManager
	metrics        businessMetrics
}

func NewUseCase(repo tripRepository, manager txManager, metrics businessMetrics) *UseCase {
	return &UseCase{
		tripRepository: repo,
		txManager:      manager,
		metrics:        metrics,
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

	if uc.metrics != nil {
		uc.metrics.RecordTripCreated(ctx)
	}

	return nil
}
