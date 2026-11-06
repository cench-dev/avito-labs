package trip

import (
	"github.com/google/uuid"

	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/model"
)

func ConvertAPITrip(trip model.Trip) (api.Trip, error) {
	id, err := uuid.Parse(trip.ID)
	if err != nil {
		return api.Trip{}, err
	}

	userID, err := uuid.Parse(trip.UserID)
	if err != nil {
		return api.Trip{}, err
	}

	driverID, err := uuid.Parse(trip.DriverID)
	if err != nil {
		return api.Trip{}, err
	}
	res := api.Trip{
		Id:       id,
		UserId:   userID,
		DriverId: driverID,
		StartPoint: api.Coordinates{
			Latitude:  trip.StartLatitude,
			Longitude: trip.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  trip.EndLatitude,
			Longitude: trip.EndLongitude,
		},
		Price:      trip.Price,
		Status:     api.TripStatus(trip.Status),
		StartedAt:  trip.StartedAt,
		FinishedAt: trip.FinishedAt,
	}

	return res, nil
}
