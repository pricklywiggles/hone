package store

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// OverviewStats holds aggregate counts for the stats dashboard.
type OverviewStats struct {
	Total         int `db:"total"`
	Mastered      int `db:"mastered"`
	AttemptedOnce int `db:"attempted_once"`
	Untouched     int `db:"untouched"`
	DueToday      int `db:"due_today"`
}

// DiffStat holds per-difficulty aggregate counts.
type DiffStat struct {
	Difficulty string `db:"difficulty"`
	Total      int    `db:"total"`
	Attempted  int    `db:"attempted"`
	Mastered   int    `db:"mastered"`
}

// TopicStat holds per-topic aggregate counts.
type TopicStat struct {
	Name        string  `db:"name"`
	Total       int     `db:"total"`
	Attempted   int     `db:"attempted"`
	Mastered    int     `db:"mastered"`
	DueToday    int     `db:"due_today"`
	SuccessRate float64 `db:"success_rate"` // -1 means no attempts yet
}

// RecentAttempt holds a single attempt row with problem metadata.
type RecentAttempt struct {
	Title       string `db:"title"`
	Difficulty  string `db:"difficulty"`
	Result      string `db:"result"`
	DurationSec int    `db:"duration_seconds"`
	StartedAt   string `db:"started_at"`
}

// ProblemRow is a denormalized view of a problem with SRS and attempt data.
type ProblemRow struct {
	ID          int    `db:"id"`
	Title       string `db:"title"`
	Difficulty  string `db:"difficulty"`
	Platform    string `db:"platform"`
	Slug        string `db:"slug"`
	Mastered    bool   `db:"mastered"`
	Successes   int    `db:"successes"`
	AttemptCount int   `db:"attempt_count"`
	NextReview  string `db:"next_review_date"`
	IsOverdue   bool   `db:"is_overdue"`
}

// GetOverviewStats returns aggregate counts across all problems.
func GetOverviewStats(db *sqlx.DB) (OverviewStats, error) {
	var s OverviewStats
	err := db.QueryRowx(`
		SELECT
			(SELECT COUNT(*) FROM problems) AS total,
			(SELECT COUNT(*) FROM problem_srs WHERE mastered_before = 1) AS mastered,
			(SELECT COUNT(DISTINCT problem_id) FROM attempts) AS attempted_once,
			(SELECT COUNT(*) FROM problems
			 WHERE id NOT IN (SELECT DISTINCT problem_id FROM attempts)) AS untouched,
			(SELECT COUNT(*) FROM problem_srs WHERE next_review_date <= date('now')) AS due_today
	`).StructScan(&s)
	return s, err
}

// GetStreak returns the number of consecutive days (ending today or yesterday)
// on which at least one attempt was made.
func GetStreak(db *sqlx.DB) (int, error) {
	rows, err := db.Queryx(`
		SELECT DISTINCT date(started_at) AS day
		FROM attempts
		ORDER BY day DESC
		LIMIT 365
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var days []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, err
		}
		days = append(days, d)
	}

	if len(days) == 0 {
		return 0, nil
	}

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	// Streak must start from today or yesterday.
	if days[0] != today && days[0] != yesterday {
		return 0, nil
	}

	streak := 1
	for i := 1; i < len(days); i++ {
		prev, _ := time.Parse("2006-01-02", days[i-1])
		curr, _ := time.Parse("2006-01-02", days[i])
		if prev.AddDate(0, 0, -1).Format("2006-01-02") == curr.Format("2006-01-02") {
			streak++
		} else {
			break
		}
	}
	return streak, nil
}

// GetDiffStats returns mastery/attempt counts grouped by difficulty.
func GetDiffStats(db *sqlx.DB) ([]DiffStat, error) {
	var stats []DiffStat
	err := db.Select(&stats, `
		SELECT
			p.difficulty,
			COUNT(*) AS total,
			COUNT(DISTINCT CASE WHEN a.problem_id IS NOT NULL THEN p.id END) AS attempted,
			COALESCE(SUM(ps.mastered_before), 0) AS mastered
		FROM problems p
		LEFT JOIN problem_srs ps ON ps.problem_id = p.id
		LEFT JOIN (SELECT DISTINCT problem_id FROM attempts) a ON a.problem_id = p.id
		GROUP BY p.difficulty
		ORDER BY CASE p.difficulty WHEN 'easy' THEN 1 WHEN 'medium' THEN 2 WHEN 'hard' THEN 3 END
	`)
	return stats, err
}

// GetTopicStats returns mastery/attempt counts grouped by topic, ordered by
// success rate ascending (weakest topics first). Topics with no attempts appear last.
func GetTopicStats(db *sqlx.DB) ([]TopicStat, error) {
	var stats []TopicStat
	err := db.Select(&stats, `
		SELECT
			t.name,
			COUNT(DISTINCT pt.problem_id) AS total,
			COUNT(DISTINCT CASE WHEN a.problem_id IS NOT NULL THEN pt.problem_id END) AS attempted,
			COALESCE(SUM(DISTINCT CASE WHEN ps.mastered_before THEN pt.problem_id END), 0) AS mastered,
			COUNT(DISTINCT CASE WHEN ps.next_review_date <= date('now') THEN pt.problem_id END) AS due_today,
			COALESCE(
				CAST(SUM(CASE WHEN a.result = 'success' THEN 1.0 ELSE 0.0 END) AS REAL)
				/ NULLIF(COUNT(a.id), 0),
				-1.0
			) AS success_rate
		FROM topics t
		JOIN problem_topics pt ON pt.topic_id = t.id
		LEFT JOIN problem_srs ps ON ps.problem_id = pt.problem_id
		LEFT JOIN attempts a ON a.problem_id = pt.problem_id
		GROUP BY t.id, t.name
		ORDER BY CASE WHEN success_rate < 0 THEN 1 ELSE 0 END, success_rate ASC
	`)
	return stats, err
}

// GetRecentAttempts returns the n most recent attempts with problem metadata.
func GetRecentAttempts(db *sqlx.DB, n int) ([]RecentAttempt, error) {
	var attempts []RecentAttempt
	err := db.Select(&attempts, `
		SELECT p.title, p.difficulty, a.result, a.duration_seconds, a.started_at
		FROM attempts a
		JOIN problems p ON p.id = a.problem_id
		ORDER BY a.started_at DESC
		LIMIT ?
	`, n)
	return attempts, err
}

// GetAllProblems returns all problems with SRS state and attempt counts,
// ordered by next review date then title.
func GetAllProblems(db *sqlx.DB) ([]ProblemRow, error) {
	var rows []ProblemRow
	err := db.Select(&rows, `
		SELECT
			p.id, p.title, p.difficulty, p.platform, p.slug,
			COALESCE(ps.mastered_before, 0) AS mastered,
			COALESCE(SUM(CASE WHEN a.result = 'success' THEN 1 ELSE 0 END), 0) AS successes,
			COALESCE(COUNT(a.id), 0) AS attempt_count,
			COALESCE(ps.next_review_date, date('now')) AS next_review_date,
			CASE WHEN COALESCE(ps.next_review_date, date('now')) < date('now') THEN 1 ELSE 0 END AS is_overdue
		FROM problems p
		LEFT JOIN problem_srs ps ON ps.problem_id = p.id
		LEFT JOIN attempts a ON a.problem_id = p.id
		GROUP BY p.id
		ORDER BY ps.next_review_date ASC, p.title ASC
	`)
	return rows, err
}
