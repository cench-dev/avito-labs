package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Masterminds/squirrel"
	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/txmanager"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type TripRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

var ErrTripAlreadyCompleted = errors.New("trip already completed")

type CreateTripParams struct {
	ID             pgtype.UUID
	UserID         pgtype.UUID
	DriverID       pgtype.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         string
	StartedAt      time.Time
}

type CreateTripStatusHistoryParams struct {
	TripID     pgtype.UUID
	FromStatus *string
	ToStatus   string
	Reason     *string
}

func NewTripRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *TripRepository {
	return &TripRepository{
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func (r *TripRepository) CreateTrip(ctx context.Context, params CreateTripParams) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	builder := squirrel.Insert("trips")
	builder = builder.Columns(
		"id",
		"user_id",
		"driver_id",
		"start_latitude",
		"start_longitude",
		"end_latitude",
		"end_longitude",
		"price",
		"status",
		"started_at",
	)
	builder = builder.Values(
		params.ID,
		params.UserID,
		params.DriverID,
		params.StartLatitude,
		params.StartLongitude,
		params.EndLatitude,
		params.EndLongitude,
		params.Price,
		params.Status,
		params.StartedAt,
	)
	builder = builder.PlaceholderFormat(squirrel.Dollar)
	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}

	executor := txmanager.ExecutorFromContext(ctx, r.pool)
	_, err = executor.Exec(ctx, query, args...)
	return err
}

func (r *TripRepository) GetTrip(ctx context.Context, id pgtype.UUID) (api.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	builder := squirrel.Select(
		"id",
		"user_id",
		"driver_id",
		"start_latitude",
		"start_longitude",
		"end_latitude",
		"end_longitude",
		"price",
		"status",
		"started_at",
		"finished_at",
	).From("trips")

	builder = builder.Where(squirrel.Eq{"id": id})
	builder = builder.PlaceholderFormat(squirrel.Dollar)
	query, args, err := builder.ToSql()
	if err != nil {
		return api.Trip{}, err
	}

	executor := txmanager.ExecutorFromContext(ctx, r.pool)
	row := executor.QueryRow(ctx, query, args...)

	var (
		tripID         pgtype.UUID
		userID         pgtype.UUID
		driverID       pgtype.UUID
		startLatitude  float64
		startLongitude float64
		endLatitude    float64
		endLongitude   float64
		price          int64
		status         string
		startedAt      time.Time
		finishedAt     *time.Time
	)

	err = row.Scan(
		&tripID,
		&userID,
		&driverID,
		&startLatitude,
		&startLongitude,
		&endLatitude,
		&endLongitude,
		&price,
		&status,
		&startedAt,
		&finishedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.Trip{}, pgx.ErrNoRows
		}
		return api.Trip{}, err
	}

	return api.Trip{
		Id:       openapi_types.UUID(tripID.Bytes),
		UserId:   openapi_types.UUID(userID.Bytes),
		DriverId: openapi_types.UUID(driverID.Bytes),
		StartPoint: api.Coordinates{
			Latitude:  startLatitude,
			Longitude: startLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  endLatitude,
			Longitude: endLongitude,
		},
		Price:      price,
		Status:     api.TripStatus(status),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}, nil
}

func (r *TripRepository) FinishTrip(ctx context.Context, id pgtype.UUID, finishedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	builder := squirrel.Update("trips")
	builder = builder.Set("status", "completed")
	builder = builder.Set("finished_at", finishedAt)
	builder = builder.Where(squirrel.Eq{
		"id":     id,
		"status": "active",
	})
	builder = builder.PlaceholderFormat(squirrel.Dollar)
	query, args, err := builder.ToSql()

	if err != nil {
		return err
	}

	executor := txmanager.ExecutorFromContext(ctx, r.pool)
	result, err := executor.Exec(ctx, query, args...)

	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()

	if rowsAffected == 0 {
		var status string

		err := executor.QueryRow(
			ctx,
			"SELECT status FROM trips WHERE id = $1",
			id,
		).Scan(&status)

		if errors.Is(err, pgx.ErrNoRows) {
			return pgx.ErrNoRows
		}

		if err != nil {
			return err
		}

		if status == "completed" {
			return ErrTripAlreadyCompleted
		}

		return pgx.ErrNoRows
	}

	return nil
}

func (r *TripRepository) CreateTripStatusHistory(ctx context.Context, params CreateTripStatusHistoryParams) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	builder := squirrel.Insert("trip_status_history")
	builder = builder.Columns(
		"trip_id",
		"from_status",
		"to_status",
		"reason",
	)
	builder = builder.Values(
		params.TripID,
		params.FromStatus,
		params.ToStatus,
		params.Reason,
	)
	builder = builder.PlaceholderFormat(squirrel.Dollar)

	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}

	executor := txmanager.ExecutorFromContext(ctx, r.pool)
	_, err = executor.Exec(ctx, query, args...)
	return err
}
