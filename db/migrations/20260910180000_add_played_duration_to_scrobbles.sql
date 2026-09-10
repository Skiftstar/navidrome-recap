-- +goose Up
ALTER TABLE scrobbles ADD COLUMN played_duration_ms INTEGER;

-- +goose Down
ALTER TABLE scrobbles DROP COLUMN played_duration_ms;
