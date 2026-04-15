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
	ID          int     `db:"id"`
	Name        string  `db:"name"`
	Total       int     `db:"total"`
	Attempted   int     `db:"attempted"`
	Mastered    int     `db:"mastered"`
	DueToday    int     `db:"due_today"`
	Successes   int     `db:"successes"`
	Failures    int     `db:"failures"`
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

// PlaylistStats holds progress counts for a specific playlist.
type PlaylistStats struct {
	Name      string `db:"name"`
	Total     int    `db:"total"`
	Attempted int    `db:"attempted"`
	Mastered  int    `db:"mastered"`
	DueToday  int    `db:"due_today"`
}

// GetPlaylistStats returns progress counts for a specific playlist.
func GetPlaylistStats(db *sqlx.DB, playlistID int) (PlaylistStats, error) {
	var s PlaylistStats
	err := db.QueryRowx(`
		SELECT
			pl.name,
			COUNT(DISTINCT pp.problem_id) AS total,
			COUNT(DISTINCT CASE WHEN a.problem_id IS NOT NULL THEN pp.problem_id END) AS attempted,
			COALESCE(SUM(CASE WHEN ps.mastered_before = 1 THEN 1 ELSE 0 END), 0) AS mastered,
			COUNT(DISTINCT CASE WHEN ps.next_review_date <= ? THEN pp.problem_id END) AS due_today
		FROM playlists pl
		JOIN playlist_problems pp ON pp.playlist_id = pl.id
		LEFT JOIN problem_srs ps ON ps.problem_id = pp.problem_id
		LEFT JOIN (SELECT DISTINCT problem_id FROM attempts) a ON a.problem_id = pp.problem_id
		WHERE pl.id = ?
	`, localToday(), playlistID).StructScan(&s)
	return s, err
}

// GetTopicStatsById returns progress counts for a specific topic.
func GetTopicStatsById(db *sqlx.DB, topicID int) (PlaylistStats, error) {
	var s PlaylistStats
	err := db.QueryRowx(`
		SELECT
			t.name,
			COUNT(DISTINCT pt.problem_id) AS total,
			COUNT(DISTINCT CASE WHEN a.problem_id IS NOT NULL THEN pt.problem_id END) AS attempted,
			COALESCE(SUM(CASE WHEN ps.mastered_before = 1 THEN 1 ELSE 0 END), 0) AS mastered,
			COUNT(DISTINCT CASE WHEN ps.next_review_date <= ? THEN pt.problem_id END) AS due_today
		FROM topics t
		JOIN problem_topics pt ON pt.topic_id = t.id
		LEFT JOIN problem_srs ps ON ps.problem_id = pt.problem_id
		LEFT JOIN (SELECT DISTINCT problem_id FROM attempts) a ON a.problem_id = pt.problem_id
		WHERE t.id = ?
	`, localToday(), topicID).StructScan(&s)
	return s, err
}

// GetOverviewStats returns aggregate counts across all problems.
func GetOverviewStats(db *sqlx.DB) (OverviewStats, error) {
	today := localToday()
	var s OverviewStats
	err := db.QueryRowx(`
		SELECT
			(SELECT COUNT(*) FROM problems) AS total,
			(SELECT COUNT(*) FROM problem_srs WHERE mastered_before = 1) AS mastered,
			(SELECT COUNT(DISTINCT problem_id) FROM attempts) AS attempted_once,
			(SELECT COUNT(*) FROM problems
			 WHERE id NOT IN (SELECT DISTINCT problem_id FROM attempts)) AS untouched,
			(SELECT COUNT(*) FROM problem_srs WHERE next_review_date <= ?) AS due_today
	`, today).StructScan(&s)
	return s, err
}

// GetStreak returns the number of consecutive days (ending today or yesterday)
// on which at least one attempt was made.
func GetStreak(db *sqlx.DB) (int, error) {
	rows, err := db.Queryx(`
		SELECT DISTINCT date(started_at, 'localtime') AS day
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

	today := localToday()
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

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

// GetTopicStats returns per-topic stats ordered by performance ascending
// ((successes - failures) / total problems). Topics with no attempts appear last.
func GetTopicStats(db *sqlx.DB) ([]TopicStat, error) {
	today := localToday()
	var stats []TopicStat
	err := db.Select(&stats, `
		SELECT
			t.id,
			t.name,
			COUNT(DISTINCT pt.problem_id) AS total,
			COUNT(DISTINCT CASE WHEN a.problem_id IS NOT NULL THEN pt.problem_id END) AS attempted,
			COUNT(DISTINCT CASE WHEN ps.mastered_before THEN pt.problem_id END) AS mastered,
			COUNT(DISTINCT CASE WHEN ps.next_review_date <= ? THEN pt.problem_id END) AS due_today,
			COALESCE(SUM(CASE WHEN a.result = 'success' THEN 1 ELSE 0 END), 0) AS successes,
			COALESCE(SUM(CASE WHEN a.result != 'success' AND a.id IS NOT NULL THEN 1 ELSE 0 END), 0) AS failures,
			CASE WHEN COUNT(a.id) = 0 THEN -1.0
				ELSE CAST(
					SUM(CASE WHEN a.id IS NULL THEN 0 WHEN a.result = 'success' THEN 1.0 ELSE -1.0 END) AS REAL
				) / COUNT(DISTINCT pt.problem_id)
			END AS success_rate
		FROM topics t
		JOIN problem_topics pt ON pt.topic_id = t.id
		LEFT JOIN problem_srs ps ON ps.problem_id = pt.problem_id
		LEFT JOIN attempts a ON a.problem_id = pt.problem_id
		GROUP BY t.id, t.name
		ORDER BY CASE WHEN COUNT(a.id) = 0 THEN 1 ELSE 0 END, success_rate ASC
	`, today)
	return stats, err
}

// GetPlaylistPerfStats returns per-playlist stats ordered by performance ascending
// ((successes - failures) / total problems). Playlists with no attempts appear last.
func GetPlaylistPerfStats(db *sqlx.DB) ([]TopicStat, error) {
	today := localToday()
	var stats []TopicStat
	err := db.Select(&stats, `
		SELECT
			pl.id,
			pl.name,
			COUNT(DISTINCT pp.problem_id) AS total,
			COUNT(DISTINCT CASE WHEN a.problem_id IS NOT NULL THEN pp.problem_id END) AS attempted,
			COUNT(DISTINCT CASE WHEN ps.mastered_before THEN pp.problem_id END) AS mastered,
			COUNT(DISTINCT CASE WHEN ps.next_review_date <= ? THEN pp.problem_id END) AS due_today,
			COALESCE(SUM(CASE WHEN a.result = 'success' THEN 1 ELSE 0 END), 0) AS successes,
			COALESCE(SUM(CASE WHEN a.result != 'success' AND a.id IS NOT NULL THEN 1 ELSE 0 END), 0) AS failures,
			CASE WHEN COUNT(a.id) = 0 THEN -1.0
				ELSE CAST(
					SUM(CASE WHEN a.id IS NULL THEN 0 WHEN a.result = 'success' THEN 1.0 ELSE -1.0 END) AS REAL
				) / COUNT(DISTINCT pp.problem_id)
			END AS success_rate
		FROM playlists pl
		JOIN playlist_problems pp ON pp.playlist_id = pl.id
		LEFT JOIN problem_srs ps ON ps.problem_id = pp.problem_id
		LEFT JOIN attempts a ON a.problem_id = pp.problem_id
		GROUP BY pl.id, pl.name
		ORDER BY CASE WHEN COUNT(a.id) = 0 THEN 1 ELSE 0 END, success_rate ASC
	`, today)
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

// TodayStats holds the number of attempts and successes for today.
type TodayStats struct {
	Attempted    int `db:"attempted"`
	Succeeded    int `db:"succeeded"`
	DueRemaining int `db:"due_remaining"`
}

// GetTodayStats returns how many problems were attempted and solved today (local time),
// plus how many problems are due today across the entire library.
func GetTodayStats(db *sqlx.DB) (TodayStats, error) {
	today := localToday()
	var s TodayStats
	err := db.Get(&s, `
		SELECT
			(SELECT COUNT(*) FROM attempts WHERE date(started_at, 'localtime') = ?) AS attempted,
			(SELECT COALESCE(SUM(CASE WHEN result = 'success' THEN 1 ELSE 0 END), 0)
			 FROM attempts WHERE date(started_at, 'localtime') = ?) AS succeeded,
			(SELECT COUNT(*) FROM problem_srs
			 WHERE next_review_date <= ?
			 AND problem_id NOT IN (
				 SELECT DISTINCT problem_id FROM attempts
				 WHERE date(started_at, 'localtime') = ?
			 )) AS due_remaining
	`, today, today, today, today)
	return s, err
}

// GetTodayStatsFiltered returns today's attempt stats filtered by playlist or topic,
// plus how many problems are due today within that filter scope.
func GetTodayStatsFiltered(db *sqlx.DB, filter PracticeFilter) (TodayStats, error) {
	today := localToday()

	var attemptJoin, dueJoin, excludeJoin string
	var args []any

	if filter.PlaylistID != nil {
		attemptJoin = ` JOIN playlist_problems pp ON pp.problem_id = a.problem_id AND pp.playlist_id = ?`
		dueJoin = ` JOIN playlist_problems pp ON pp.problem_id = ps.problem_id AND pp.playlist_id = ?`
		excludeJoin = ` JOIN playlist_problems pp ON pp.problem_id = a.problem_id AND pp.playlist_id = ?`
	}
	if filter.TopicID != nil {
		attemptJoin += ` JOIN problem_topics pt ON pt.problem_id = a.problem_id AND pt.topic_id = ?`
		dueJoin += ` JOIN problem_topics pt ON pt.problem_id = ps.problem_id AND pt.topic_id = ?`
		excludeJoin += ` JOIN problem_topics pt ON pt.problem_id = a.problem_id AND pt.topic_id = ?`
	}

	addFilterArgs := func() {
		if filter.PlaylistID != nil {
			args = append(args, *filter.PlaylistID)
		}
		if filter.TopicID != nil {
			args = append(args, *filter.TopicID)
		}
	}

	// attempted subquery args
	addFilterArgs()
	args = append(args, today)
	// succeeded subquery args
	addFilterArgs()
	args = append(args, today)
	// due_remaining subquery args (due join + exclude join + two dates)
	addFilterArgs()
	args = append(args, today)
	addFilterArgs()
	args = append(args, today)

	var s TodayStats
	err := db.Get(&s, `
		SELECT
			(SELECT COUNT(*) FROM attempts a`+attemptJoin+`
			 WHERE date(a.started_at, 'localtime') = ?) AS attempted,
			(SELECT COALESCE(SUM(CASE WHEN a.result = 'success' THEN 1 ELSE 0 END), 0)
			 FROM attempts a`+attemptJoin+`
			 WHERE date(a.started_at, 'localtime') = ?) AS succeeded,
			(SELECT COUNT(*) FROM problem_srs ps`+dueJoin+`
			 WHERE ps.next_review_date <= ?
			 AND ps.problem_id NOT IN (
				 SELECT DISTINCT a.problem_id FROM attempts a`+excludeJoin+`
				 WHERE date(a.started_at, 'localtime') = ?
			 )) AS due_remaining
	`, args...)
	return s, err
}

// GetAllProblems returns all problems with SRS state and attempt counts,
// ordered by next review date then title.
func GetAllProblems(db *sqlx.DB) ([]ProblemRow, error) {
	today := localToday()
	var rows []ProblemRow
	err := db.Select(&rows, `
		SELECT
			p.id, p.title, p.difficulty, p.platform, p.slug,
			COALESCE(ps.mastered_before, 0) AS mastered,
			COALESCE(SUM(CASE WHEN a.result = 'success' THEN 1 ELSE 0 END), 0) AS successes,
			COALESCE(COUNT(a.id), 0) AS attempt_count,
			COALESCE(ps.next_review_date, ?) AS next_review_date,
			CASE WHEN COALESCE(ps.next_review_date, ?) < ? THEN 1 ELSE 0 END AS is_overdue
		FROM problems p
		LEFT JOIN problem_srs ps ON ps.problem_id = p.id
		LEFT JOIN attempts a ON a.problem_id = p.id
		GROUP BY p.id
		ORDER BY ps.next_review_date ASC, p.title ASC
	`, today, today, today)
	return rows, err
}
