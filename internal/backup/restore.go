package backup

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

// RestoreFromBackup inserts all data from a BackupData into an empty database.
// The DB must already exist and have migrations applied.
func RestoreFromBackup(db *sqlx.DB, data BackupData) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Map "platform/slug" → problem ID for foreign-key lookups.
	problemIDs := make(map[string]int64, len(data.Problems))

	for _, p := range data.Problems {
		res, err := tx.Exec(
			`INSERT INTO problems (platform, slug, title, difficulty, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			p.Platform, p.Slug, p.Title, p.Difficulty, p.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert problem %s/%s: %w", p.Platform, p.Slug, err)
		}
		id, _ := res.LastInsertId()
		problemIDs[p.Platform+"/"+p.Slug] = id

		// Override SRS defaults set by the trigger.
		masteredInt := 0
		if p.MasteredBefore {
			masteredInt = 1
		}
		_, err = tx.Exec(`
			UPDATE problem_srs
			SET easiness_factor  = ?,
			    interval_days    = ?,
			    repetition_count = ?,
			    next_review_date = ?,
			    mastered_before  = ?
			WHERE problem_id = ?`,
			p.EasinessFactor, p.IntervalDays, p.RepetitionCount,
			p.NextReviewDate, masteredInt, id)
		if err != nil {
			return fmt.Errorf("restore srs %s/%s: %w", p.Platform, p.Slug, err)
		}

		for _, topic := range p.Topics {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO topics (name) VALUES (?)`, topic); err != nil {
				return fmt.Errorf("insert topic %q: %w", topic, err)
			}
			var topicID int64
			if err := tx.QueryRow(`SELECT id FROM topics WHERE name = ?`, topic).Scan(&topicID); err != nil {
				return fmt.Errorf("lookup topic %q: %w", topic, err)
			}
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO problem_topics (problem_id, topic_id) VALUES (?, ?)`,
				id, topicID); err != nil {
				return fmt.Errorf("link topic: %w", err)
			}
		}
	}

	playlistIDs := make(map[string]int64, len(data.Playlists))
	for _, pl := range data.Playlists {
		res, err := tx.Exec(`INSERT INTO playlists (name) VALUES (?)`, pl.Name)
		if err != nil {
			return fmt.Errorf("insert playlist %q: %w", pl.Name, err)
		}
		plID, _ := res.LastInsertId()
		playlistIDs[pl.Name] = plID

		for _, key := range pl.Problems {
			probID, ok := problemIDs[key]
			if !ok {
				return fmt.Errorf("playlist %q references unknown problem %q", pl.Name, key)
			}
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO playlist_problems (playlist_id, problem_id) VALUES (?, ?)`,
				plID, probID); err != nil {
				return fmt.Errorf("link playlist problem: %w", err)
			}
		}
	}
	_ = playlistIDs

	for i, a := range data.Attempts {
		probID, ok := problemIDs[a.Problem]
		if !ok {
			return fmt.Errorf("attempt %d references unknown problem %q", i, a.Problem)
		}

		// Use NULL for optional fields when empty/zero.
		var completedAt, result interface{}
		var durationSec, quality interface{}

		if a.CompletedAt != "" {
			completedAt = a.CompletedAt
		}
		if a.Result != "" {
			result = a.Result
		}
		if a.DurationSeconds != 0 {
			durationSec = a.DurationSeconds
		}
		if a.Quality != 0 {
			quality = a.Quality
		}

		_, err := tx.Exec(`
			INSERT INTO attempts (problem_id, started_at, completed_at, result, duration_seconds, quality)
			VALUES (?, ?, ?, ?, ?, ?)`,
			probID, a.StartedAt, completedAt, result, durationSec, quality)
		if err != nil {
			return fmt.Errorf("insert attempt %d: %w", i, err)
		}
	}

	return tx.Commit()
}
