package store

import (
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

// GetProblemBySlug returns the problem matching platform+slug, or nil if not found.
func GetProblemBySlug(db *sqlx.DB, platform, slug string) (*Problem, error) {
	var p Problem
	err := db.Get(&p,
		`SELECT id, platform, slug, title, difficulty, created_at FROM problems WHERE platform = ? AND slug = ?`,
		platform, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// InsertProblem inserts a problem and its topics, returning the new problem ID.
// Topics are upserted (INSERT OR IGNORE) and linked via problem_topics.
// Returns error if (platform, slug) already exists.
func InsertProblem(db *sqlx.DB, platform, slug, title, difficulty string, topics []string) (int64, error) {
	tx, err := db.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO problems (platform, slug, title, difficulty) VALUES (?, ?, ?, ?)`,
		platform, slug, title, difficulty,
	)
	if err != nil {
		return 0, err
	}

	problemID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, topic := range topics {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO topics (name) VALUES (?)`, topic); err != nil {
			return 0, err
		}

		var topicID int64
		if err := tx.QueryRow(`SELECT id FROM topics WHERE name = ?`, topic).Scan(&topicID); err != nil {
			return 0, err
		}

		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO problem_topics (problem_id, topic_id) VALUES (?, ?)`,
			problemID, topicID,
		); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return problemID, nil
}
