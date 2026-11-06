package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cench-dev/avito-labs/internal/config"
	"github.com/cench-dev/avito-labs/internal/httpserver"
	"github.com/cench-dev/avito-labs/internal/postgres"
	"github.com/cench-dev/avito-labs/internal/repository"
	"github.com/cench-dev/avito-labs/internal/txmanager"
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

	tripRepository := repository.NewTripRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := txmanager.NewTxManager(pool)

	server := httpserver.NewServer(cfg.HTTPAddr, cfg.ShutdownTimeout, 10*time.Second, 10*time.Second, 10*time.Second, 10*time.Second, pool, tripRepository, txManager)

	go func() {
		err := server.Start()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("http server:", err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	<-sigs

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		fmt.Println("shutdown:", err)
	}

	poolClosed := make(chan struct{})

	go func() {
		pool.Close()
		close(poolClosed)
	}()

	select {
	case <-poolClosed:
	case <-ctx.Done():
		fmt.Println("database pool close timeout:", ctx.Err())
	}
}
