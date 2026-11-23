package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cench-dev/avito-labs/internal/config"
	"github.com/cench-dev/avito-labs/internal/gateway/push"
	tripCreate "github.com/cench-dev/avito-labs/internal/http/trip/create"
	tripFinish "github.com/cench-dev/avito-labs/internal/http/trip/finish"
	tripGet "github.com/cench-dev/avito-labs/internal/http/trip/get"
	tripPositions "github.com/cench-dev/avito-labs/internal/http/trip/positions"
	"github.com/cench-dev/avito-labs/internal/httpserver"
	"github.com/cench-dev/avito-labs/internal/observability"
	"github.com/cench-dev/avito-labs/internal/postgres"
	"github.com/cench-dev/avito-labs/internal/repository"
	"github.com/cench-dev/avito-labs/internal/txmanager"
	"github.com/cench-dev/avito-labs/internal/usecase/trip/create"
	"github.com/cench-dev/avito-labs/internal/usecase/trip/finish"
	"github.com/cench-dev/avito-labs/internal/usecase/trip/get"
	positionsUseCase "github.com/cench-dev/avito-labs/internal/usecase/trip/positions"
	positionsWorker "github.com/cench-dev/avito-labs/internal/worker/positions"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return
	}

	logger, err := observability.NewLogger(cfg.LogLevel)
	if err != nil {
		slog.Error("failed to initialize logger", "error", err)
		return
	}

	logger.Info("starting service", "service", "trip-service")

	otel, err := observability.InitOTel(context.Background(), observability.OTelConfig{
		Endpoint: cfg.OTLPEndpoint,
		Protocol: cfg.OTLPProtocol,
		Sampler:  cfg.TracesSampler,
	})
	if err != nil {
		logger.Error("failed to initialize OpenTelemetry", "error", err)
		return
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := otel.Shutdown(ctx); err != nil {
			logger.Error("failed to shutdown OpenTelemetry", "error", err)
		}
	}()

	businessMetrics, err := observability.NewBusinessMetrics(otel.MeterProvider)
	if err != nil {
		logger.Error("failed to create business metrics", "error", err)
		return
	}

	httpMetrics, err := observability.NewHTTPMetrics(otel.MeterProvider)
	if err != nil {
		logger.Error("failed to create HTTP metrics", "error", err)
		return
	}

	positionMetrics, err := observability.NewPositionMetrics(otel.MeterProvider)
	if err != nil {
		logger.Error("failed to create position metrics", "error", err)
		return
	}

	pool, err := postgres.CreatePool(
		context.Background(),
		cfg.DatabaseURL,
		cfg.DatabaseMaxConns,
		cfg.DatabaseMinConns,
		cfg.DatabaseConnectTimeout,
		cfg.DatabaseMaxConnLifetime,
	)
	if err != nil {
		logger.Error("failed to create postgres pool", "error", err)
		return
	}

	tripRepository := repository.NewTripRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := txmanager.NewTxManager(pool)

	createTripUseCase := create.NewUseCase(tripRepository, txManager, businessMetrics)
	getTripUseCase := get.NewUseCase(tripRepository)
	finishTripUseCase := finish.NewUseCase(tripRepository, txManager, businessMetrics)

	createTripHandler := tripCreate.NewHandler(createTripUseCase)
	getTripHandler := tripGet.NewHandler(getTripUseCase)
	finishTripHandler := tripFinish.NewHandler(finishTripUseCase, getTripUseCase)
	positionsUC := positionsUseCase.NewUseCase(tripRepository, txManager, positionMetrics)

	pushClient := push.NewClient(cfg.PushHTTPURL, cfg.PushRequestTimeout)

	worker := positionsWorker.NewWorker(
		pushClient,
		cfg.PositionWorkers,
		cfg.PositionQueueSize,
		cfg.PushRequestTimeout,
		positionMetrics,
		logger,
	)

	scanner := positionsWorker.NewScanner(
		tripRepository,
		worker,
		cfg.PositionCheckInterval,
		cfg.PositionStaleAfter,
		logger,
		positionMetrics,
	)

	positionsHandler := tripPositions.NewHandler(positionsUC, worker)

	server := httpserver.NewServer(
		cfg.HTTPAddr,
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
		pool,
		createTripHandler,
		getTripHandler,
		finishTripHandler,
		httpMetrics,
		logger,
		positionsHandler,
	)

	go func() {
		err := server.Start()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
		}
	}()

	workerCtx, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})

	go func() {
		defer close(workerDone)
		worker.Run(workerCtx)
	}()

	scannerCtx, stopScanner := context.WithCancel(context.Background())
	scannerDone := make(chan struct{})

	go func() {
		defer close(scannerDone)
		scanner.Run(scannerCtx)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	<-sigs

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
	}

	stopScanner()

	select {
	case <-scannerDone:
	case <-ctx.Done():
		logger.Error("scanner shutdown timeout", "error", ctx.Err())
	}

	worker.SetShutdownContext(ctx)
	stopWorker()

	select {
	case <-workerDone:
	case <-ctx.Done():
		logger.Error("worker shutdown timeout", "error", ctx.Err())
	}

	pushClient.Close()

	poolClosed := make(chan struct{})

	go func() {
		pool.Close()
		close(poolClosed)
	}()

	select {
	case <-poolClosed:
	case <-ctx.Done():
		logger.Error("database pool close timeout", "error", ctx.Err())
	}
}
