//go:build integration

package txmanager

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTxManager_RollbackTripOnError(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL must be set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	tripID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	startedAt := time.Now().UTC()

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM trip_status_history WHERE trip_id = $1`, tripID)
		_, _ = pool.Exec(ctx, `DELETE FROM trips WHERE id = $1`, tripID)
	})

	tm := NewTxManager(pool)

	err = tm.Do(ctx, func(txCtx context.Context) error {
		exec := ExecutorFromContext(txCtx, pool)

		_, err := exec.Exec(txCtx, `
			INSERT INTO trips (
				id, user_id, driver_id,
				start_latitude, start_longitude,
				end_latitude, end_longitude,
				price, status, started_at
			) VALUES ($1, $2, $3, 10, 20, 30, 40, 100, 'active', $4)
		`, tripID, userID, driverID, startedAt)
		if err != nil {
			return err
		}

		_, err = exec.Exec(txCtx, `
			INSERT INTO trip_status_history (trip_id, from_status, to_status)
			VALUES ($1, NULL, 'invalid-status')
		`, tripID)
		if err == nil {
			t.Fatal("expected history insert to fail")
		}

		return err
	})
	if err == nil {
		t.Fatal("expected transaction to fail")
	}

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM trips WHERE id = $1`, tripID).Scan(&count)
	if err != nil {
		t.Fatalf("check trip row: %v", err)
	}
	if count != 0 {
		t.Fatalf("trip row was not rolled back: found %d rows", count)
	}

	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM trip_status_history WHERE trip_id = $1`, tripID).Scan(&count)
	if err != nil {
		t.Fatalf("check history row: %v", err)
	}
	if count != 0 {
		t.Fatalf("history row unexpectedly exists: found %d rows", count)
	}
}
