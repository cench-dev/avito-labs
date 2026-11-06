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
	metrics        businessMetrics
}

func NewUseCase(repo tripRepository, manager txManager, metrics businessMetrics) *UseCase {
	return &UseCase{
		tripRepository: repo,
		txManager:      manager,
		metrics:        metrics,
	}
}

func (uc *UseCase) Finish(ctx context.Context, tripID string, finishedAt time.Time) error {
	res := uc.txManager.Do(ctx, func(ctx context.Context) error {
		tripModel, err := uc.tripRepository.GetTrip(ctx, tripID)
		if err != nil {
			return err
		}

		domainTrip := tripDomain.Trip{
			Status:     tripDomain.Status(tripModel.Status),
			FinishedAt: tripModel.FinishedAt,
		}

		fromStatus := string(domainTrip.Status)

		if err := domainTrip.Finish(finishedAt); err != nil {
			return err
		}

		if err := uc.tripRepository.FinishTrip(ctx, tripID, finishedAt); err != nil {
			return err
		}

		tripStatusHistory := model.TripStatusHistory{
			TripID:     tripID,
			FromStatus: &fromStatus,
			ToStatus:   string(domainTrip.Status),
			Reason:     nil,
		}

		if err := uc.tripRepository.CreateTripStatusHistory(ctx, tripStatusHistory); err != nil {
			return err
		}

		return nil
	})

	if res != nil {
		return res
	}

	if uc.metrics != nil {
		uc.metrics.RecordTripCompleted(ctx)
	}

	return nil
}
