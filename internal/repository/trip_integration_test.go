//go:build integration

package repository

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
)

func newTripRepositoryIntegration(t *testing.T) (*TripRepository, *pgxpool.Pool) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return NewTripRepository(pool, 3*time.Second), pool
}

func newIntegrationTrip(driverID string) model.Trip {
	return model.Trip{
		ID:             uuid.NewString(),
		UserID:         uuid.NewString(),
		DriverID:       driverID,
		StartLatitude:  10,
		StartLongitude: 20,
		EndLatitude:    30,
		EndLongitude:   40,
		Price:          1000,
		Status:         string(tripDomain.StatusActive),
		StartedAt:      time.Now().UTC().Truncate(time.Microsecond),
	}
}

func cleanupIntegrationTrip(t *testing.T, pool *pgxpool.Pool, tripIDs ...string) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for _, id := range tripIDs {
			_, _ = pool.Exec(ctx, `DELETE FROM trip_status_history WHERE trip_id = $1`, id)
			_, _ = pool.Exec(ctx, `DELETE FROM trips WHERE id = $1`, id)
		}
	})
}

func TestTripRepository_CreateAndGetTrip(t *testing.T) {
	repo, pool := newTripRepositoryIntegration(t)
	ctx := context.Background()

	trip := newIntegrationTrip(uuid.NewString())
	cleanupIntegrationTrip(t, pool, trip.ID)

	if err := repo.CreateTrip(ctx, trip); err != nil {
		t.Fatalf("CreateTrip() error = %v", err)
	}

	got, err := repo.GetTrip(ctx, trip.ID)
	if err != nil {
		t.Fatalf("GetTrip() error = %v", err)
	}

	if got.ID != trip.ID {
		t.Errorf("ID = %q, want %q", got.ID, trip.ID)
	}
	if got.UserID != trip.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, trip.UserID)
	}
	if got.DriverID != trip.DriverID {
		t.Errorf("DriverID = %q, want %q", got.DriverID, trip.DriverID)
	}
	if got.StartLatitude != trip.StartLatitude {
		t.Errorf("StartLatitude = %v, want %v", got.StartLatitude, trip.StartLatitude)
	}
	if got.StartLongitude != trip.StartLongitude {
		t.Errorf("StartLongitude = %v, want %v", got.StartLongitude, trip.StartLongitude)
	}
	if got.EndLatitude != trip.EndLatitude {
		t.Errorf("EndLatitude = %v, want %v", got.EndLatitude, trip.EndLatitude)
	}
	if got.EndLongitude != trip.EndLongitude {
		t.Errorf("EndLongitude = %v, want %v", got.EndLongitude, trip.EndLongitude)
	}
	if got.Price != trip.Price {
		t.Errorf("Price = %d, want %d", got.Price, trip.Price)
	}
	if got.Status != trip.Status {
		t.Errorf("Status = %q, want %q", got.Status, trip.Status)
	}
	if !got.StartedAt.Equal(trip.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, trip.StartedAt)
	}
	if got.FinishedAt != nil {
		t.Errorf("FinishedAt = %v, want nil", got.FinishedAt)
	}
}

func TestTripRepository_GetTripNotFound(t *testing.T) {
	repo, _ := newTripRepositoryIntegration(t)

	_, err := repo.GetTrip(context.Background(), uuid.NewString())
	if !errors.Is(err, tripDomain.ErrNotFound) {
		t.Fatalf("GetTrip() error = %v, want %v", err, tripDomain.ErrNotFound)
	}
}

func TestTripRepository_CreateTripDriverBusy(t *testing.T) {
	repo, pool := newTripRepositoryIntegration(t)
	ctx := context.Background()

	driverID := uuid.NewString()
	first := newIntegrationTrip(driverID)
	second := newIntegrationTrip(driverID)
	cleanupIntegrationTrip(t, pool, first.ID, second.ID)

	if err := repo.CreateTrip(ctx, first); err != nil {
		t.Fatalf("CreateTrip(first) error = %v", err)
	}

	err := repo.CreateTrip(ctx, second)
	if !errors.Is(err, tripDomain.ErrDriverBusy) {
		t.Fatalf("CreateTrip(second) error = %v, want %v", err, tripDomain.ErrDriverBusy)
	}
}

func TestTripRepository_FinishTrip(t *testing.T) {
	repo, pool := newTripRepositoryIntegration(t)
	ctx := context.Background()

	trip := newIntegrationTrip(uuid.NewString())
	cleanupIntegrationTrip(t, pool, trip.ID)

	if err := repo.CreateTrip(ctx, trip); err != nil {
		t.Fatalf("CreateTrip() error = %v", err)
	}

	finishedAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := repo.FinishTrip(ctx, trip.ID, finishedAt); err != nil {
		t.Fatalf("FinishTrip() error = %v", err)
	}

	got, err := repo.GetTrip(ctx, trip.ID)
	if err != nil {
		t.Fatalf("GetTrip() after finish error = %v", err)
	}

	if got.Status != string(tripDomain.StatusCompleted) {
		t.Errorf("Status = %q, want %q", got.Status, tripDomain.StatusCompleted)
	}
	if got.FinishedAt == nil {
		t.Fatal("FinishedAt is nil after finishing trip")
	}
	if !got.FinishedAt.Equal(finishedAt) {
		t.Errorf("FinishedAt = %v, want %v", *got.FinishedAt, finishedAt)
	}
}

func TestTripRepository_FinishTripAlreadyCompleted(t *testing.T) {
	repo, pool := newTripRepositoryIntegration(t)
	ctx := context.Background()

	trip := newIntegrationTrip(uuid.NewString())
	cleanupIntegrationTrip(t, pool, trip.ID)

	if err := repo.CreateTrip(ctx, trip); err != nil {
		t.Fatalf("CreateTrip() error = %v", err)
	}

	if err := repo.FinishTrip(ctx, trip.ID, time.Now().UTC()); err != nil {
		t.Fatalf("first FinishTrip() error = %v", err)
	}

	err := repo.FinishTrip(ctx, trip.ID, time.Now().UTC())
	if !errors.Is(err, tripDomain.ErrAlreadyCompleted) {
		t.Fatalf("second FinishTrip() error = %v, want %v", err, tripDomain.ErrAlreadyCompleted)
	}
}

func TestTripRepository_FinishTripNotFound(t *testing.T) {
	repo, _ := newTripRepositoryIntegration(t)

	err := repo.FinishTrip(context.Background(), uuid.NewString(), time.Now().UTC())
	if !errors.Is(err, tripDomain.ErrNotFound) {
		t.Fatalf("FinishTrip() error = %v, want %v", err, tripDomain.ErrNotFound)
	}
}

func TestTripRepository_CreateTripStatusHistory(t *testing.T) {
	repo, pool := newTripRepositoryIntegration(t)
	ctx := context.Background()

	trip := newIntegrationTrip(uuid.NewString())
	cleanupIntegrationTrip(t, pool, trip.ID)

	if err := repo.CreateTrip(ctx, trip); err != nil {
		t.Fatalf("CreateTrip() error = %v", err)
	}

	fromStatus := string(tripDomain.StatusActive)
	reason := "integration test"
	history := model.TripStatusHistory{
		TripID:     trip.ID,
		FromStatus: &fromStatus,
		ToStatus:   string(tripDomain.StatusCompleted),
		Reason:     &reason,
	}

	if err := repo.CreateTripStatusHistory(ctx, history); err != nil {
		t.Fatalf("CreateTripStatusHistory() error = %v", err)
	}

	var (
		gotFromStatus *string
		gotToStatus   string
		gotReason     *string
	)

	err := pool.QueryRow(ctx, `
		SELECT from_status, to_status, reason
		FROM trip_status_history
		WHERE trip_id = $1
	`, trip.ID).Scan(&gotFromStatus, &gotToStatus, &gotReason)
	if err != nil {
		t.Fatalf("query history row: %v", err)
	}

	if gotFromStatus == nil || *gotFromStatus != fromStatus {
		t.Errorf("from_status = %v, want %q", gotFromStatus, fromStatus)
	}
	if gotToStatus != history.ToStatus {
		t.Errorf("to_status = %q, want %q", gotToStatus, history.ToStatus)
	}
	if gotReason == nil || *gotReason != reason {
		t.Errorf("reason = %v, want %q", gotReason, reason)
	}
}

func TestTripRepository_CreateTripStatusHistoryForeignKeyError(t *testing.T) {
	repo, _ := newTripRepositoryIntegration(t)

	history := model.TripStatusHistory{
		TripID:   uuid.NewString(),
		ToStatus: string(tripDomain.StatusCompleted),
	}

	err := repo.CreateTripStatusHistory(context.Background(), history)
	if err == nil {
		t.Fatal("CreateTripStatusHistory() error = nil, want foreign key violation")
	}
}
