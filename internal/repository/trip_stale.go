package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	"github.com/cench-dev/avito-labs/internal/txmanager"
)

type StaleTrip struct {
	ID       string
	DriverID string
}

func (r *TripRepository) GetStaleActiveTrips(
	ctx context.Context,
	staleAfter time.Duration,
) ([]StaleTrip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	executor := txmanager.ExecutorFromContext(ctx, r.pool)

	rows, err := executor.Query(ctx, `
		SELECT id, driver_id
		FROM trips
		WHERE status = $2
		  AND COALESCE(last_position_at, started_at)
		      < NOW() - ($1 * INTERVAL '1 second')
		ORDER BY COALESCE(last_position_at, started_at) ASC, id ASC
	`, staleAfter.Seconds(), string(tripDomain.StatusActive))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trips := make([]StaleTrip, 0)

	for rows.Next() {
		var id, driverID pgtype.UUID
		if err := rows.Scan(&id, &driverID); err != nil {
			return nil, err
		}

		trips = append(trips, StaleTrip{
			ID:       uuid.UUID(id.Bytes).String(),
			DriverID: uuid.UUID(driverID.Bytes).String(),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return trips, nil
}
