//go:build integration

package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	"github.com/cench-dev/avito-labs/internal/model"
	"github.com/cench-dev/avito-labs/internal/repository"
	"github.com/cench-dev/avito-labs/internal/txmanager"
	positionsUseCase "github.com/cench-dev/avito-labs/internal/usecase/trip/positions"
)

var errUpdateLastPosition = errors.New("forced update error")

type failingPositionRepository struct {
	*repository.TripRepository
}

func (r *failingPositionRepository) UpdateLastPositionAt(context.Context, string, time.Time) error {
	return errUpdateLastPosition
}

func TestCreateRollsBackPositionWhenLastPositionUpdateFails(t *testing.T) {
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

	tripID := uuid.NewString()
	tripRepo := repository.NewTripRepository(pool, 3*time.Second)

	trip := model.Trip{
		ID:             tripID,
		UserID:         uuid.NewString(),
		DriverID:       uuid.NewString(),
		StartLatitude:  10,
		StartLongitude: 20,
		EndLatitude:    30,
		EndLongitude:   40,
		Price:          1000,
		Status:         string(tripDomain.StatusActive),
		StartedAt:      time.Now().UTC(),
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM trips WHERE id = $1`, tripID)
	})

	if err := tripRepo.CreateTrip(ctx, trip); err != nil {
		t.Fatalf("CreateTrip() error = %v", err)
	}

	failingRepo := &failingPositionRepository{
		TripRepository: tripRepo,
	}
	uc := positionsUseCase.NewUseCase(failingRepo, txmanager.NewTxManager(pool), nil)

	position := model.TripPosition{
		TripID:     tripID,
		Latitude:   55.75,
		Longitude:  37.61,
		RecordedAt: time.Now().UTC(),
	}

	_, err = uc.Create(ctx, position)
	if !errors.Is(err, errUpdateLastPosition) {
		t.Fatalf("Create() error = %v, want %v", err, errUpdateLastPosition)
	}

	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM trip_positions
		WHERE trip_id = $1
	`, tripID).Scan(&count)
	if err != nil {
		t.Fatalf("count trip positions: %v", err)
	}
	if count != 0 {
		t.Fatalf("position was not rolled back: found %d rows", count)
	}

	var lastPositionAt *time.Time
	err = pool.QueryRow(ctx, `
		SELECT last_position_at
		FROM trips
		WHERE id = $1
	`, tripID).Scan(&lastPositionAt)
	if err != nil {
		t.Fatalf("get last_position_at: %v", err)
	}
	if lastPositionAt != nil {
		t.Fatalf("last_position_at = %v, want nil", lastPositionAt)
	}
}
