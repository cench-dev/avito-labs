package finish

import (
	"context"
	"time"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	"github.com/cench-dev/avito-labs/internal/model"
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

func (uc *UseCase) Finish(ctx context.Context, tripID string, finishedAt time.Time) error {
	res := uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.tripRepository.FinishTrip(ctx, tripID, finishedAt); err != nil {
			return err
		}

		fromStatus := string(tripDomain.StatusActive)

		tripStatusHistory := model.TripStatusHistory{
			TripID:     tripID,
			FromStatus: &fromStatus,
			ToStatus:   string(tripDomain.StatusCompleted),
			Reason:     nil,
		}

		if err := uc.tripRepository.CreateTripStatusHistory(ctx, tripStatusHistory); err != nil {
			return err
		}

		return nil
	})

	return res
}
