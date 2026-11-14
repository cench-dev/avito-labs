package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreatePool(ctx context.Context, databaseURL string, databaseMaxConns int32,
	databaseMinConns int32, databaseConnectTimeout time.Duration, databaseMaxConnLifetime time.Duration) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	cfg.MaxConnLifetime = databaseMaxConnLifetime
	cfg.MaxConns = databaseMaxConns
	cfg.MinConns = databaseMinConns
	ctxWithTimeout, cancel := context.WithTimeout(ctx, databaseConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctxWithTimeout, cfg)
	if err != nil {
		return nil, err
	}

	err = pool.Ping(ctxWithTimeout)
	if err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
