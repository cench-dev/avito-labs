package get

import (
	"context"

	"github.com/cench-dev/avito-labs/internal/model"
)

type UseCase struct {
	tripRepository tripRepository
}

func NewUseCase(repo tripRepository) *UseCase {
	return &UseCase{
		tripRepository: repo,
	}
}

func (uc *UseCase) Get(ctx context.Context, tripID string) (model.Trip, error) {
	res, err := uc.tripRepository.GetTrip(ctx, tripID)
	if err != nil {
		return model.Trip{}, err
	}

	return res, nil
}
