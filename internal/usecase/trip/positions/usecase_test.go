package positions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cench-dev/avito-labs/internal/model"
)

var errUpdatePosition = errors.New("update last position failed")

type mockTripRepository struct {
	lockErr   error
	createID  int64
	createErr error
	updateErr error
}

func (m *mockTripRepository) LockActiveTrip(context.Context, string) error {
	return m.lockErr
}

func (m *mockTripRepository) CreateTripPosition(context.Context, model.TripPosition) (int64, error) {
	return m.createID, m.createErr
}

func (m *mockTripRepository) UpdateLastPositionAt(context.Context, string, time.Time) error {
	return m.updateErr
}

func (m *mockTripRepository) GetTripPositions(context.Context, string) ([]model.TripPosition, error) {
	return nil, nil
}

func (m *mockTripRepository) GetTrip(context.Context, string) (model.Trip, error) {
	return model.Trip{}, nil
}

type mockTxManager struct {
	committed  bool
	rolledBack bool
}

func (m *mockTxManager) Do(ctx context.Context, fn func(context.Context) error) error {
	err := fn(ctx)
	if err != nil {
		m.rolledBack = true
		return err
	}

	m.committed = true
	return nil
}

func TestCreateRollsBackWhenUpdatingLastPositionFails(t *testing.T) {
	repo := &mockTripRepository{
		createID:  42,
		updateErr: errUpdatePosition,
	}
	tx := &mockTxManager{}

	uc := NewUseCase(repo, tx, nil)

	position := model.TripPosition{
		TripID:     "trip-123",
		Latitude:   55.75,
		Longitude:  37.61,
		RecordedAt: time.Now(),
	}

	id, err := uc.Create(context.Background(), position)

	if !errors.Is(err, errUpdatePosition) {
		t.Fatalf("Create() error = %v, want %v", err, errUpdatePosition)
	}

	if id != 0 {
		t.Errorf("Create() id = %d, want 0", id)
	}

	if !tx.rolledBack {
		t.Error("transaction was not rolled back")
	}

	if tx.committed {
		t.Error("transaction was committed after an error")
	}
}
