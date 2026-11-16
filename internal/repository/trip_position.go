package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cench-dev/avito-labs/internal/model"
	"github.com/cench-dev/avito-labs/internal/txmanager"
)

func (r *TripRepository) CreateTripPosition(ctx context.Context, position model.TripPosition) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	tripUUID, err := uuid.Parse(position.TripID)
	if err != nil {
		return 0, err
	}

	tripID := pgtype.UUID{Bytes: tripUUID, Valid: true}
	executor := txmanager.ExecutorFromContext(ctx, r.pool)

	var id int64
	err = executor.QueryRow(ctx, `
		INSERT INTO trip_positions (
			trip_id, latitude, longitude, recorded_at
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`,
		tripID,
		position.Latitude,
		position.Longitude,
		position.RecordedAt,
	).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (r *TripRepository) GetTripPositions(ctx context.Context, tripID string) ([]model.TripPosition, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	tripUUID, err := uuid.Parse(tripID)
	if err != nil {
		return nil, err
	}

	id := pgtype.UUID{Bytes: tripUUID, Valid: true}
	executor := txmanager.ExecutorFromContext(ctx, r.pool)

	rows, err := executor.Query(ctx, `
		SELECT id, trip_id, latitude, longitude, recorded_at, created_at
		FROM trip_positions
		WHERE trip_id = $1
		ORDER BY recorded_at ASC, id ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := make([]model.TripPosition, 0)

	for rows.Next() {
		var position model.TripPosition

		if err := rows.Scan(
			&position.ID,
			&position.TripID,
			&position.Latitude,
			&position.Longitude,
			&position.RecordedAt,
			&position.CreatedAt,
		); err != nil {
			return nil, err
		}

		positions = append(positions, position)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return positions, nil
}

var _ pgx.Tx
