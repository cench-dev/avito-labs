package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	"github.com/cench-dev/avito-labs/internal/model"
	"github.com/cench-dev/avito-labs/internal/txmanager"
)

type TripRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewTripRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *TripRepository {
	return &TripRepository{
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func (r *TripRepository) CreateTrip(ctx context.Context, params model.Trip) error {
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
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "trips_limitation_for_driver_idx" {
		return tripDomain.ErrDriverBusy
	}

	return err
}

func (r *TripRepository) GetTrip(ctx context.Context, id string) (model.Trip, error) {
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
		return model.Trip{}, err
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
			return model.Trip{}, tripDomain.ErrNotFound
		}
		return model.Trip{}, err
	}

	return model.Trip{
		ID:             tripID.String(),
		UserID:         userID.String(),
		DriverID:       driverID.String(),
		StartLatitude:  startLatitude,
		StartLongitude: startLongitude,
		EndLatitude:    endLatitude,
		EndLongitude:   endLongitude,
		Price:          price,
		Status:         status,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
	}, nil
}

func (r *TripRepository) FinishTrip(ctx context.Context, id string, finishedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	builder := squirrel.Update("trips")
	builder = builder.Set("status", string(tripDomain.StatusCompleted))
	builder = builder.Set("finished_at", finishedAt)
	builder = builder.Where(squirrel.Eq{
		"id":     id,
		"status": string(tripDomain.StatusActive),
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
			return tripDomain.ErrNotFound
		}

		if err != nil {
			return err
		}

		if status == string(tripDomain.StatusCompleted) {
			return tripDomain.ErrAlreadyCompleted
		}

		return tripDomain.ErrNotFound
	}

	return nil
}

func (r *TripRepository) CreateTripStatusHistory(ctx context.Context, params model.TripStatusHistory) error {
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
