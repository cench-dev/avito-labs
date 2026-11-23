package positions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/cench-dev/avito-labs/internal/observability"
)

type pendingJob struct {
	id      uint64
	started bool
}

type Worker struct {
	pushGateway    pushGateway
	queue          chan positionJob
	workers        int
	requestTimeout time.Duration
	shutdownCtx    context.Context
	metrics        *observability.PositionMetrics
	logger         *slog.Logger

	mu            sync.Mutex
	pending       map[string]pendingJob
	nextJobID     uint64
	activeCancels map[uint64]context.CancelFunc
}

func NewWorker(gateway pushGateway, workers, queueSize int, requestTimeout time.Duration, metrics *observability.PositionMetrics, logger *slog.Logger) *Worker {
	return &Worker{
		pushGateway:    gateway,
		queue:          make(chan positionJob, queueSize),
		workers:        workers,
		requestTimeout: requestTimeout,
		pending:        make(map[string]pendingJob),
		metrics:        metrics,
		logger:         logger,
		activeCancels:  make(map[uint64]context.CancelFunc),
	}
}

func (w *Worker) SetShutdownContext(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.shutdownCtx = ctx

	for _, cancel := range w.activeCancels {
		context.AfterFunc(ctx, cancel)
	}
}

func (w *Worker) Enqueue(ctx context.Context, tripID, driverID string) {
	w.EnqueueWithResult(ctx, tripID, driverID)
}

func (w *Worker) Cancel(tripID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	job, exists := w.pending[tripID]
	if exists && !job.started {
		delete(w.pending, tripID)
	}
}

func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup

	for i := 0; i < w.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.runWorker(ctx)
		}()
	}

	<-ctx.Done()
	wg.Wait()
}

func (w *Worker) runWorker(ctx context.Context) {
	w.metrics.AddActiveWorkers(context.Background(), 1)
	defer w.metrics.AddActiveWorkers(context.Background(), -1)

	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			shutdownCtx := w.shutdownCtx
			w.mu.Unlock()

			if shutdownCtx == nil {
				return
			}

			for {
				if shutdownCtx.Err() != nil {
					return
				}

				select {
				case job := <-w.queue:
					w.metrics.AddQueueSize(context.Background(), -1)
					w.process(shutdownCtx, job)
				default:
					return
				}
			}

		case job := <-w.queue:
			w.metrics.AddQueueSize(context.Background(), -1)
			w.process(ctx, job)
		}
	}
}

func (w *Worker) process(ctx context.Context, job positionJob) {
	tripID := job.tripID

	w.mu.Lock()
	pending, exists := w.pending[tripID]

	if !exists || pending.id != job.id || pending.started {
		w.mu.Unlock()
		return
	}

	pending.started = true
	w.pending[tripID] = pending

	jobCtx, cancelJob := context.WithCancel(context.WithoutCancel(ctx))
	w.activeCancels[job.id] = cancelJob

	shutdownCtx := w.shutdownCtx
	w.mu.Unlock()

	if shutdownCtx != nil {
		context.AfterFunc(shutdownCtx, cancelJob)
	}

	defer func() {
		cancelJob()

		w.mu.Lock()
		delete(w.activeCancels, job.id)

		if current, exists := w.pending[tripID]; exists && current.id == job.id {
			delete(w.pending, tripID)
		}
		w.mu.Unlock()
	}()

	tracer := otel.Tracer("trip-service/positions")

	var spanOptions []trace.SpanStartOption
	if job.spanContext.IsValid() {
		spanOptions = append(spanOptions, trace.WithLinks(trace.Link{
			SpanContext: job.spanContext,
		}))
	}

	jobCtx, span := tracer.Start(
		jobCtx,
		"positions.worker.process",
		append([]trace.SpanStartOption{trace.WithNewRoot()}, spanOptions...)...,
	)
	defer span.End()

	start := time.Now()

	requestCtx, cancel := context.WithTimeout(jobCtx, w.requestTimeout)
	err := w.pushGateway.Send(requestCtx, tripID, job.driverID)
	cancel()

	result := "success"
	if err != nil {
		result = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, "push request failed")
	}

	w.metrics.RecordPositionRequest(jobCtx, result, time.Since(start))

	if err != nil {
		w.logger.Error(
			"failed to send position request",
			"trip_id", tripID,
			"error", err,
		)
	}
}

func (w *Worker) EnqueueWithResult(ctx context.Context, tripID, driverID string) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.pending[tripID]; exists {
		return "duplicate"
	}

	w.nextJobID++
	jobID := w.nextJobID

	select {
	case w.queue <- positionJob{
		id:          jobID,
		tripID:      tripID,
		driverID:    driverID,
		spanContext: trace.SpanContextFromContext(ctx),
	}:
		w.pending[tripID] = pendingJob{id: jobID}
		w.metrics.AddQueueSize(context.Background(), 1)
		return "queued"
	default:
		return "dropped"
	}
}

type positionJob struct {
	id          uint64
	tripID      string
	driverID    string
	spanContext trace.SpanContext
}
