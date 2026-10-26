package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cench-dev/avito-labs/internal/config"
	tripCreate "github.com/cench-dev/avito-labs/internal/http/trip/create"
	tripFinish "github.com/cench-dev/avito-labs/internal/http/trip/finish"
	tripGet "github.com/cench-dev/avito-labs/internal/http/trip/get"
	"github.com/cench-dev/avito-labs/internal/httpserver"
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
		fmt.Println(err)
		return
	}
	pool, err := postgres.CreatePool(context.Background(), cfg.DatabaseURL, cfg.DatabaseMaxConns, cfg.DatabaseMinConns, cfg.DatabaseConnectTimeout, cfg.DatabaseMaxConnLifetime)
	if err != nil {
		fmt.Println(err)
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
		pool, createTripHandler, getTripHandler, finishTripHandler)

	go func() {
		err := server.Start()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("http server:", err)
		}
	}()

	if err := server.WaitForShutdown(); err != nil {
		fmt.Println("shutdown:", err)
	}
}
