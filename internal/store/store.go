package store

import (
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/srs"
)

// Problem mirrors the problems table.
type Problem struct {
	ID         int    `db:"id"`
	Platform   string `db:"platform"`
	Slug       string `db:"slug"`
	Title      string `db:"title"`
	Difficulty string `db:"difficulty"`
	CreatedAt  string `db:"created_at"`
}

// pickRow is used to scan a joined problems + problem_srs query result.
type pickRow struct {
	ID              int     `db:"id"`
	Platform        string  `db:"platform"`
	Slug            string  `db:"slug"`
	Title           string  `db:"title"`
	Difficulty      string  `db:"difficulty"`
	CreatedAt       string  `db:"created_at"`
	ProblemID       int     `db:"problem_id"`
	EasinessFactor  float64 `db:"easiness_factor"`
	IntervalDays    int     `db:"interval_days"`
	RepetitionCount int     `db:"repetition_count"`
	NextReviewDate  string  `db:"next_review_date"`
	MasteredBefore  int     `db:"mastered_before"`
}

func (r pickRow) problem() *Problem {
	return &Problem{
		ID:         r.ID,
		Platform:   r.Platform,
		Slug:       r.Slug,
		Title:      r.Title,
		Difficulty: r.Difficulty,
		CreatedAt:  r.CreatedAt,
	}
}

func (r pickRow) srsState() *srs.ProblemSRS {
	return &srs.ProblemSRS{
		ProblemID:       r.ProblemID,
		EasinessFactor:  r.EasinessFactor,
		IntervalDays:    r.IntervalDays,
		RepetitionCount: r.RepetitionCount,
		NextReviewDate:  r.NextReviewDate,
		MasteredBefore:  r.MasteredBefore,
	}
}

const pickCols = `
	p.id, p.platform, p.slug, p.title, p.difficulty, p.created_at,
	ps.problem_id, ps.easiness_factor, ps.interval_days, ps.repetition_count,
	ps.next_review_date, ps.mastered_before`

// PickNext returns the next problem to practice.
// If a playlistID is provided, candidates are filtered to that playlist.
// The bool return indicates whether the problem is due today (true) or upcoming (false).
// Returns nil, nil, false, nil when no problems exist.
func PickNext(db *sqlx.DB, playlistID *int) (*Problem, *srs.ProblemSRS, bool, error) {
	playlistJoin, playlistArg := playlistClause(playlistID)

	// Try due problems first (most overdue first).
	dueQuery := `
		SELECT` + pickCols + `
		FROM problems p
		JOIN problem_srs ps ON ps.problem_id = p.id` +
		playlistJoin + `
		WHERE ps.next_review_date <= date('now')
		ORDER BY ps.next_review_date ASC
		LIMIT 1`

	var row pickRow
	var err error
	if playlistID != nil {
		err = db.Get(&row, dueQuery, playlistArg)
	} else {
		err = db.Get(&row, dueQuery)
	}
	if err == nil {
		return row.problem(), row.srsState(), true, nil
	}
	if !isNotFound(err) {
		return nil, nil, false, err
	}

	// Nothing due — pick the one with the nearest upcoming review date.
	upcomingQuery := `
		SELECT` + pickCols + `
		FROM problems p
		JOIN problem_srs ps ON ps.problem_id = p.id` +
		playlistJoin + `
		WHERE ps.next_review_date > date('now')
		ORDER BY ps.next_review_date ASC
		LIMIT 1`

	if playlistID != nil {
		err = db.Get(&row, upcomingQuery, playlistArg)
	} else {
		err = db.Get(&row, upcomingQuery)
	}
	if err == nil {
		return row.problem(), row.srsState(), false, nil
	}
	if isNotFound(err) {
		return nil, nil, false, nil
	}
	return nil, nil, false, err
}

// RecordAttempt inserts a completed attempt row.
func RecordAttempt(db *sqlx.DB, problemID int, startedAt, completedAt time.Time, result string, durationSec, quality int) error {
	_, err := db.Exec(`
		INSERT INTO attempts (problem_id, started_at, completed_at, result, duration_seconds, quality)
		VALUES (?, ?, ?, ?, ?, ?)`,
		problemID,
		startedAt.UTC().Format("2006-01-02 15:04:05"),
		completedAt.UTC().Format("2006-01-02 15:04:05"),
		result,
		durationSec,
		quality,
	)
	return err
}

// SaveSRSState updates the problem_srs row for the given state.
func SaveSRSState(db *sqlx.DB, state srs.ProblemSRS) error {
	_, err := db.Exec(`
		UPDATE problem_srs
		SET easiness_factor  = ?,
		    interval_days    = ?,
		    repetition_count = ?,
		    next_review_date = ?,
		    mastered_before  = ?
		WHERE problem_id = ?`,
		state.EasinessFactor,
		state.IntervalDays,
		state.RepetitionCount,
		state.NextReviewDate,
		state.MasteredBefore,
		state.ProblemID,
	)
	return err
}

func playlistClause(playlistID *int) (string, interface{}) {
	if playlistID == nil {
		return "", nil
	}
	return " JOIN playlist_problems pp ON pp.problem_id = p.id AND pp.playlist_id = ?", *playlistID
}

func isNotFound(err error) bool {
	return err != nil && err.Error() == "sql: no rows in result set"
}
