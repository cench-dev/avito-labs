package positions

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/cench-dev/avito-labs/internal/observability"
	"github.com/cench-dev/avito-labs/internal/repository"
)

const (
	testTripID    = "trip-1"
	testTripID2   = "trip-2"
	testDriverID  = "driver-1"
	testDriverID2 = "driver-2"
)

type mockStaleTripRepository struct {
	calls      atomic.Int32
	trips      []repository.StaleTrip
	err        error
	staleAfter time.Duration
}

type mockScannerTicker struct {
	ticks chan time.Time
}

func (t *mockScannerTicker) C() <-chan time.Time {
	return t.ticks
}

func (t *mockScannerTicker) Stop() {}

func (m *mockStaleTripRepository) GetStaleActiveTrips(_ context.Context, staleAfter time.Duration) ([]repository.StaleTrip, error) {
	m.calls.Add(1)
	m.staleAfter = staleAfter

	return m.trips, m.err
}

func TestScannerEnqueuesStaleTrips(t *testing.T) {
	metrics := newTestPositionMetrics(t)

	repo := &mockStaleTripRepository{
		trips: []repository.StaleTrip{
			{ID: testTripID, DriverID: testDriverID},
			{ID: testTripID2, DriverID: testDriverID2},
		},
	}

	worker := NewWorker(&mockPushGateway{}, 1, 2, time.Second, metrics, slog.Default())

	scanner := NewScanner(repo, worker, time.Hour, time.Minute, slog.Default(), metrics)

	scanner.scan(context.Background())

	if got := repo.calls.Load(); got != 1 {
		t.Fatalf("repository calls = %d, want 1", got)
	}

	if got := len(worker.queue); got != 2 {
		t.Fatalf("queue length = %d, want 2", got)
	}

	if repo.staleAfter != time.Minute {
		t.Fatalf("staleAfter = %s, want %s", repo.staleAfter, time.Minute)
	}
}

func TestScannerStopsOnContextCancellation(t *testing.T) {
	provider := sdkmetric.NewMeterProvider()
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})

	metrics, err := observability.NewPositionMetrics(provider)
	if err != nil {
		t.Fatalf("create metrics: %v", err)
	}

	repo := &mockStaleTripRepository{}
	worker := NewWorker(&mockPushGateway{}, 1, 1, time.Second, metrics, slog.Default())

	scanner := NewScanner(repo, worker, time.Hour, time.Minute, slog.Default(), metrics)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner.Run(ctx)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop after context cancellation")
	}

	if got := repo.calls.Load(); got != 0 {
		t.Fatalf("repository calls = %d, want 0", got)
	}
}

func TestScannerDoesNotEnqueueDuplicates(t *testing.T) {
	metrics := newTestPositionMetrics(t)

	repo := &mockStaleTripRepository{
		trips: []repository.StaleTrip{
			{ID: testTripID, DriverID: testDriverID},
			{ID: testTripID, DriverID: testDriverID},
		},
	}

	worker := NewWorker(&mockPushGateway{}, 1, 2, time.Second, metrics, slog.Default())
	scanner := NewScanner(repo, worker, time.Hour, time.Minute, slog.Default(), metrics)

	scanner.scan(context.Background())

	if got := len(worker.queue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}

	if _, exists := worker.pending[testTripID]; !exists {
		t.Fatal(testTripID + " is not pending")
	}
}

func TestScannerDoesNotBlockWhenQueueIsFull(t *testing.T) {
	metrics := newTestPositionMetrics(t)

	repo := &mockStaleTripRepository{
		trips: []repository.StaleTrip{
			{ID: testTripID, DriverID: testDriverID},
			{ID: testTripID2, DriverID: testDriverID2},
		},
	}

	worker := NewWorker(&mockPushGateway{}, 1, 1, time.Second, metrics, slog.Default())
	scanner := NewScanner(repo, worker, time.Hour, time.Minute, slog.Default(), metrics)

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner.scan(context.Background())
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("scanner blocked when queue was full")
	}

	if got := len(worker.queue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}

	if _, exists := worker.pending[testTripID]; !exists {
		t.Fatal(testTripID + " should be pending")
	}

	if _, exists := worker.pending[testTripID2]; exists {
		t.Fatal(testTripID2 + " should not be pending")
	}
}

func TestScannerHandlesRepositoryError(t *testing.T) {
	metrics := newTestPositionMetrics(t)

	repo := &mockStaleTripRepository{
		err: errors.New("database unavailable"),
	}

	worker := NewWorker(&mockPushGateway{}, 1, 2, time.Second, metrics, slog.Default())
	scanner := NewScanner(repo, worker, time.Hour, time.Minute, slog.Default(), metrics)

	scanner.scan(context.Background())

	if got := repo.calls.Load(); got != 1 {
		t.Fatalf("repository calls = %d, want 1", got)
	}

	if got := len(worker.queue); got != 0 {
		t.Fatalf("queue length = %d, want 0", got)
	}
}

func TestScannerRunsOnTick(t *testing.T) {
	metrics := newTestPositionMetrics(t)
	repo := &mockStaleTripRepository{}

	worker := NewWorker(
		&mockPushGateway{},
		1,
		1,
		time.Second,
		metrics,
		slog.Default(),
	)

	scanner := NewScanner(
		repo,
		worker,
		time.Hour,
		time.Minute,
		slog.Default(),
		metrics,
	)

	ticker := &mockScannerTicker{
		ticks: make(chan time.Time, 1),
	}
	scanner.newTicker = func(time.Duration) scannerTicker {
		return ticker
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		scanner.Run(ctx)
	}()

	ticker.ticks <- time.Now()

	deadline := time.After(time.Second)
	for repo.calls.Load() == 0 {
		select {
		case <-deadline:
			cancel()
			t.Fatal("scanner did not run after tick")
		case <-time.After(time.Millisecond):
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop")
	}

	if got := repo.calls.Load(); got != 1 {
		t.Fatalf("repository calls = %d, want 1", got)
	}
}
