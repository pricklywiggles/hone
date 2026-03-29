-- +goose Up
CREATE TABLE problems (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    platform   TEXT    NOT NULL,
    slug       TEXT    NOT NULL,
    title      TEXT    NOT NULL,
    difficulty TEXT    NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    UNIQUE (platform, slug)
);

CREATE TABLE topics (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL UNIQUE
);

CREATE TABLE problem_topics (
    problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    topic_id   INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    PRIMARY KEY (problem_id, topic_id)
);

CREATE TABLE playlists (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    created_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE playlist_problems (
    playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    problem_id  INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    PRIMARY KEY (playlist_id, problem_id)
);

CREATE TABLE attempts (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    problem_id       INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    started_at       TEXT    NOT NULL,
    completed_at     TEXT,
    result           TEXT    CHECK (result IN ('success', 'fail')),
    duration_seconds INTEGER,
    quality          INTEGER CHECK (quality BETWEEN 1 AND 5)
);

CREATE TABLE problem_srs (
    problem_id       INTEGER PRIMARY KEY REFERENCES problems(id) ON DELETE CASCADE,
    easiness_factor  REAL    NOT NULL DEFAULT 2.5,
    interval_days    INTEGER NOT NULL DEFAULT 1,
    repetition_count INTEGER NOT NULL DEFAULT 0,
    next_review_date TEXT    NOT NULL DEFAULT (date('now')),
    mastered_before  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_problem_srs_next_review ON problem_srs(next_review_date);
CREATE INDEX idx_attempts_problem_id ON attempts(problem_id);

-- +goose StatementBegin
CREATE TRIGGER trg_init_problem_srs
AFTER INSERT ON problems
BEGIN
    INSERT INTO problem_srs (problem_id) VALUES (NEW.id);
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_init_problem_srs;
DROP TABLE IF EXISTS problem_srs;
DROP TABLE IF EXISTS attempts;
DROP TABLE IF EXISTS playlist_problems;
DROP TABLE IF EXISTS playlists;
DROP TABLE IF EXISTS problem_topics;
DROP TABLE IF EXISTS topics;
DROP TABLE IF EXISTS problems;
