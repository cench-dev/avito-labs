package postgres

import (
	"context"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreatePool(ctx context.Context, databaseURL string, databaseMaxConns, databaseMinConns int32,
	databaseConnectTimeout, databaseMaxConnLifetime time.Duration,
) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	cfg.ConnConfig.Tracer = otelpgx.NewTracer()

	cfg.MaxConnLifetime = databaseMaxConnLifetime
	cfg.MaxConns = databaseMaxConns
	cfg.MinConns = databaseMinConns

	ctxWithTimeout, cancel := context.WithTimeout(ctx, databaseConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctxWithTimeout, cfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctxWithTimeout); err != nil {
		pool.Close()
		return nil, err
	}

	if err := otelpgx.RecordStats(pool); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
