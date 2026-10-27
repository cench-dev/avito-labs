package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/cench-dev/avito-labs/internal/config"
	tripCreate "github.com/cench-dev/avito-labs/internal/http/trip/create"
	tripFinish "github.com/cench-dev/avito-labs/internal/http/trip/finish"
	tripGet "github.com/cench-dev/avito-labs/internal/http/trip/get"
	"github.com/cench-dev/avito-labs/internal/httpserver"
	"github.com/cench-dev/avito-labs/internal/observability"
	"github.com/cench-dev/avito-labs/internal/postgres"
	"github.com/cench-dev/avito-labs/internal/repository"
	"github.com/cench-dev/avito-labs/internal/txmanager"
	"github.com/cench-dev/avito-labs/internal/usecase/trip/create"
	"github.com/cench-dev/avito-labs/internal/usecase/trip/finish"
	"github.com/cench-dev/avito-labs/internal/usecase/trip/get"
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

	defer pool.Close()
	tripRepository := repository.NewTripRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := txmanager.NewTxManager(pool)
	createTripUseCase := create.NewUseCase(tripRepository, txManager)
	getTripUseCase := get.NewUseCase(tripRepository)
	finishTripUseCase := finish.NewUseCase(tripRepository, txManager)
	createTripHandler := tripCreate.NewHandler(createTripUseCase)
	getTripHandler := tripGet.NewHandler(getTripUseCase)
	finishTripHandler := tripFinish.NewHandler(finishTripUseCase, getTripUseCase)
	server := httpserver.NewServer(cfg.HTTPAddr, cfg.ShutdownTimeout, 10*time.Second,
		10*time.Second, 10*time.Second, 10*time.Second,
		pool, createTripHandler, getTripHandler, finishTripHandler, logger)

	go func() {
		err := server.Start()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
		}
	}()

	if err := server.WaitForShutdown(); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
}
