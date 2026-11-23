package positions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/cench-dev/avito-labs/internal/observability"
)

const enqueueResultQueued = "queued"

type mockPushGateway struct {
	send func(ctx context.Context, tripID, driverID string) error
}

func (m *mockPushGateway) Send(ctx context.Context, tripID, driverID string) error {
	if m.send != nil {
		return m.send(ctx, tripID, driverID)
	}
	return nil
}

func TestWorkerEnqueueDeduplicatesTrip(t *testing.T) {
	worker := NewWorker(&mockPushGateway{}, 1, 2, time.Second, newTestPositionMetrics(t), nil)

	if got := worker.EnqueueWithResult(context.Background(), "trip-1", "driver-1"); got != "queued" {
		t.Fatalf("first enqueue = %q, want %q", got, enqueueResultQueued)
	}

	if got := worker.EnqueueWithResult(context.Background(), "trip-1", "driver-1"); got != "duplicate" {
		t.Fatalf("second enqueue = %q, want duplicate", got)
	}

	if got := len(worker.queue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}
}

func TestWorkerEnqueueDropsWhenQueueIsFull(t *testing.T) {
	worker := NewWorker(&mockPushGateway{}, 1, 1, time.Second, newTestPositionMetrics(t), nil)

	if got := worker.EnqueueWithResult(context.Background(), "trip-1", "driver-1"); got != enqueueResultQueued {
		t.Fatalf("first enqueue = %q, want %q", got, enqueueResultQueued)
	}

	start := time.Now()
	if got := worker.EnqueueWithResult(context.Background(), "trip-2", "driver-2"); got != "dropped" {
		t.Fatalf("second enqueue = %q, want dropped", got)
	}

	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("enqueue blocked for %s", elapsed)
	}
}

func TestWorkerContinuesAfterPushError(t *testing.T) {
	calls := make(chan string, 2)

	gateway := &mockPushGateway{
		send: func(_ context.Context, tripID, _ string) error {
			calls <- tripID
			if tripID == "trip-1" {
				return errors.New("push failed")
			}
			return nil
		},
	}

	worker := NewWorker(gateway, 1, 2, time.Second, newTestPositionMetrics(t), slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	worker.Enqueue(context.Background(), "trip-1", "driver-1")
	worker.Enqueue(context.Background(), "trip-2", "driver-2")

	for i := 0; i < 2; i++ {
		select {
		case <-calls:
		case <-time.After(time.Second):
			t.Fatal("worker did not process queued jobs")
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func newTestPositionMetrics(t *testing.T) *observability.PositionMetrics {
	t.Helper()

	provider := sdkmetric.NewMeterProvider()
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})

	metrics, err := observability.NewPositionMetrics(provider)
	if err != nil {
		t.Fatalf("create position metrics: %v", err)
	}

	return metrics
}

func TestWorkerCancelRemovesPendingTrip(t *testing.T) {
	worker := NewWorker(&mockPushGateway{}, 1, 2, time.Second, newTestPositionMetrics(t), slog.Default())

	if got := worker.EnqueueWithResult(context.Background(), "trip-1", "driver-1"); got != enqueueResultQueued {
		t.Fatalf("first enqueue = %q, want %q", got, enqueueResultQueued)
	}

	worker.Cancel("trip-1")

	if got := worker.EnqueueWithResult(context.Background(), "trip-1", "driver-1"); got != enqueueResultQueued {
		t.Fatalf("enqueue after cancel = %q, want %q", got, enqueueResultQueued)
	}

	if got := len(worker.queue); got != 2 {
		t.Fatalf("queue length = %d, want 2", got)
	}
}

func TestWorkerLimitsConcurrency(t *testing.T) {
	const workers = 2
	const jobs = 5

	var mu sync.Mutex
	active := 0
	maxActive := 0

	started := make(chan struct{}, jobs)
	release := make(chan struct{})

	gateway := &mockPushGateway{
		send: func(ctx context.Context, _, _ string) error {
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()

			started <- struct{}{}

			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}

			mu.Lock()
			active--
			mu.Unlock()

			return nil
		},
	}

	worker := NewWorker(gateway, workers, jobs, 5*time.Second, newTestPositionMetrics(t), slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	for i := 0; i < jobs; i++ {
		if result := worker.EnqueueWithResult(context.Background(), fmt.Sprintf("trip-%d", i), fmt.Sprintf("driver-%d", i)); result != enqueueResultQueued {
			t.Fatalf("enqueue job %d = %q, want %q", i, result, enqueueResultQueued)
		}
	}

	for i := 0; i < workers; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			cancel()
			t.Fatal("workers did not start jobs")
		}
	}

	mu.Lock()
	got := maxActive
	mu.Unlock()

	close(release)

	if got != workers {
		cancel()
		t.Fatalf("max concurrent jobs = %d, want %d", got, workers)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker pool did not stop")
	}
}

func TestWorkerDrainsQueueOnShutdown(t *testing.T) {
	calls := make(chan string, 2)

	gateway := &mockPushGateway{
		send: func(_ context.Context, tripID, _ string) error {
			calls <- tripID
			return nil
		},
	}

	worker := NewWorker(gateway, 1, 2, time.Second, newTestPositionMetrics(t), slog.Default())

	worker.Enqueue(context.Background(), "trip-1", "driver-1")
	worker.Enqueue(context.Background(), "trip-2", "driver-2")

	ctx, cancel := context.WithCancel(context.Background())
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	worker.SetShutdownContext(shutdownCtx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	cancel()

	for i := 0; i < 2; i++ {
		select {
		case <-calls:
		case <-time.After(2 * time.Second):
			t.Fatal("worker did not drain queue")
		}
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorkerSkipsCanceledQueuedTrip(t *testing.T) {
	metrics := newTestPositionMetrics(t)
	called := make(chan string, 1)

	gateway := &mockPushGateway{
		send: func(_ context.Context, tripID, _ string) error {
			called <- tripID
			return nil
		},
	}

	worker := NewWorker(gateway, 1, 2, time.Second, metrics, slog.Default())

	worker.Enqueue(context.Background(), "trip-1", "driver-1")
	worker.Cancel("trip-1")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	select {
	case tripID := <-called:
		t.Fatalf("canceled trip was sent: %s", tripID)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}
