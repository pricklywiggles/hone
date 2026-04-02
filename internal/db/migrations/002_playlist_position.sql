-- +goose Up
ALTER TABLE playlist_problems ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

-- Backfill existing rows in rowid order so they get sequential positions per playlist.
UPDATE playlist_problems
SET position = (
    SELECT COUNT(*)
    FROM playlist_problems pp2
    WHERE pp2.playlist_id = playlist_problems.playlist_id
      AND pp2.rowid < playlist_problems.rowid
);

-- +goose Down
ALTER TABLE playlist_problems DROP COLUMN position;
