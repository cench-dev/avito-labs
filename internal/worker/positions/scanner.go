package positions

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/cench-dev/avito-labs/internal/observability"
	"github.com/cench-dev/avito-labs/internal/repository"
)

type staleTripRepository interface {
	GetStaleActiveTrips(ctx context.Context, staleAfter time.Duration) ([]repository.StaleTrip, error)
}

type scannerTicker interface {
	C() <-chan time.Time
	Stop()
}

type realScannerTicker struct {
	ticker *time.Ticker
}

func (t *realScannerTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t *realScannerTicker) Stop() {
	t.ticker.Stop()
}

type Scanner struct {
	repository staleTripRepository
	worker     *Worker
	interval   time.Duration
	staleAfter time.Duration
	logger     *slog.Logger
	metrics    *observability.PositionMetrics
	newTicker  func(time.Duration) scannerTicker
}

func NewScanner(repo staleTripRepository, worker *Worker, interval, staleAfter time.Duration, logger *slog.Logger, metrics *observability.PositionMetrics) *Scanner {
	return &Scanner{
		repository: repo,
		worker:     worker,
		interval:   interval,
		staleAfter: staleAfter,
		logger:     logger,
		metrics:    metrics,
		newTicker: func(d time.Duration) scannerTicker {
			return &realScannerTicker{ticker: time.NewTicker(d)}
		},
	}
}

func (s *Scanner) Run(ctx context.Context) {
	ticker := s.newTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			s.scan(ctx)
		}
	}
}

func (s *Scanner) scan(ctx context.Context) {
	tracer := otel.Tracer("trip-service/positions")
	scanCtx, span := tracer.Start(ctx, "positions.scan")
	defer span.End()

	trips, err := s.repository.GetStaleActiveTrips(scanCtx, s.staleAfter)
	if err != nil {
		span.RecordError(err)
		s.logger.Error("failed to find stale trips", "error", err)
		return
	}

	for _, trip := range trips {
		result := s.worker.EnqueueWithResult(scanCtx, trip.ID, trip.DriverID)

		s.metrics.RecordStaleCheck(scanCtx, result)

		if result == "dropped" {
			s.logger.Warn("stale trip was not queued", "trip_id", trip.ID)
			s.metrics.RecordPositionRequestResult(scanCtx, "dropped")
		}
	}
}
