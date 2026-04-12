-- +goose Up
CREATE TABLE settings (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    active_playlist_id INTEGER REFERENCES playlists(id) ON DELETE SET NULL,
    active_topic_id    INTEGER REFERENCES topics(id)    ON DELETE SET NULL
);

INSERT INTO settings (id) VALUES (1);

-- +goose Down
DROP TABLE IF EXISTS settings;
