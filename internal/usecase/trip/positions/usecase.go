package positions

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
	"github.com/cench-dev/avito-labs/internal/observability"
)

type UseCase struct {
	tripRepository tripRepository
	txManager      txManager
	metrics        *observability.PositionMetrics
}

func NewUseCase(repo tripRepository, manager txManager, metrics *observability.PositionMetrics) *UseCase {
	return &UseCase{
		tripRepository: repo,
		txManager:      manager,
		metrics:        metrics,
	}
}

func (uc *UseCase) Create(ctx context.Context, position model.TripPosition) (int64, error) {
	var positionID int64

	err := uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.tripRepository.LockActiveTrip(ctx, position.TripID); err != nil {
			return err
		}

		id, err := uc.tripRepository.CreateTripPosition(ctx, position)
		if err != nil {
			return err
		}

		if err := uc.tripRepository.UpdateLastPositionAt(
			ctx,
			position.TripID,
			position.RecordedAt,
		); err != nil {
			return err
		}

		positionID = id
		return nil
	})

	if err == nil {
		uc.metrics.RecordPositionSaved(ctx)
	}

	return positionID, err
}

func (uc *UseCase) Get(ctx context.Context, tripID string) ([]model.TripPosition, error) {
	_, err := uc.tripRepository.GetTrip(ctx, tripID)
	if err != nil {
		return nil, err
	}

	return uc.tripRepository.GetTripPositions(ctx, tripID)
}
