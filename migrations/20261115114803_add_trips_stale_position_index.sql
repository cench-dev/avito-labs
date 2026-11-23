-- +goose Up
CREATE INDEX trips_status_stale_position_idx
    ON trips (status, (COALESCE(last_position_at, started_at)), id);

-- +goose Down
DROP INDEX trips_status_stale_position_idx;