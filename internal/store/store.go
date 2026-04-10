package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/srs"
)

const datetimeFormat = "2006-01-02 15:04:05"

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

// PracticeFilter holds the optional playlist or topic filter for picking problems.
type PracticeFilter struct {
	PlaylistID *int
	TopicID    *int
}

// PickNext returns the next problem to practice.
// The filter restricts candidates to a playlist or topic if set.
// The bool return indicates whether the problem is due today (true) or upcoming (false).
// Returns nil, nil, false, nil when no problems exist.
func PickNext(db *sqlx.DB, filter PracticeFilter) (*Problem, *srs.ProblemSRS, bool, error) {
	filterJoin, filterArgs := filterClauses(filter)
	orderBy := pickOrderBy(filter)

	// Try due problems first (most overdue first).
	dueQuery := `
		SELECT` + pickCols + `
		FROM problems p
		JOIN problem_srs ps ON ps.problem_id = p.id` +
		filterJoin + `
		WHERE ps.next_review_date <= date('now')
		ORDER BY ` + orderBy + `
		LIMIT 1`

	var row pickRow
	err := db.Get(&row, dueQuery, filterArgs...)
	if err == nil {
		return row.problem(), row.srsState(), true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, false, err
	}

	// Nothing due — pick the one with the nearest upcoming review date.
	upcomingQuery := `
		SELECT` + pickCols + `
		FROM problems p
		JOIN problem_srs ps ON ps.problem_id = p.id` +
		filterJoin + `
		WHERE ps.next_review_date > date('now')
		ORDER BY ` + orderBy + `
		LIMIT 1`

	err = db.Get(&row, upcomingQuery, filterArgs...)
	if err == nil {
		return row.problem(), row.srsState(), false, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, false, nil
	}
	return nil, nil, false, err
}

// Candidate is a problem with its next review date, used for debug display.
type Candidate struct {
	Title          string `db:"title"`
	Difficulty     string `db:"difficulty"`
	NextReviewDate string `db:"next_review_date"`
}

// ListCandidates returns all problems in pick order (same as PickNext but unbounded).
func ListCandidates(db *sqlx.DB, filter PracticeFilter) ([]Candidate, error) {
	filterJoin, filterArgs := filterClauses(filter)
	orderBy := pickOrderBy(filter)

	query := `
		SELECT p.title, p.difficulty, ps.next_review_date
		FROM problems p
		JOIN problem_srs ps ON ps.problem_id = p.id` +
		filterJoin + `
		ORDER BY ` + orderBy

	var rows []Candidate
	err := db.Select(&rows, query, filterArgs...)
	return rows, err
}

// QueueEntry pairs a problem with its SRS state and due status for the practice queue.
type QueueEntry struct {
	Problem Problem
	SRS     srs.ProblemSRS
	IsDue   bool
}

// ListPickQueue returns all candidate problems in pick order: due problems first,
// then upcoming. Used to pre-compile the practice session queue.
func ListPickQueue(db *sqlx.DB, filter PracticeFilter) ([]QueueEntry, error) {
	filterJoin, filterArgs := filterClauses(filter)
	orderBy := pickOrderBy(filter)

	buildQuery := func(dueCond string) string {
		return `
			SELECT` + pickCols + `
			FROM problems p
			JOIN problem_srs ps ON ps.problem_id = p.id` +
			filterJoin + `
			WHERE ` + dueCond + `
			ORDER BY ` + orderBy
	}

	var entries []QueueEntry
	for _, stage := range []struct {
		cond  string
		isDue bool
	}{
		{"ps.next_review_date <= date('now')", true},
		{"ps.next_review_date > date('now')", false},
	} {
		var rows []pickRow
		if err := db.Select(&rows, buildQuery(stage.cond), filterArgs...); err != nil {
			return nil, err
		}
		for _, r := range rows {
			entries = append(entries, QueueEntry{
				Problem: *r.problem(),
				SRS:     *r.srsState(),
				IsDue:   stage.isDue,
			})
		}
	}
	return entries, nil
}

const difficultyOrder = `CASE p.difficulty WHEN 'easy' THEN 1 WHEN 'medium' THEN 2 WHEN 'hard' THEN 3 ELSE 4 END`

func pickOrderBy(f PracticeFilter) string {
	tertiary := "RANDOM()"
	if f.PlaylistID != nil {
		tertiary = "pp.position ASC"
	}
	return "ps.next_review_date ASC, " + difficultyOrder + ", " + tertiary
}

// GetSRSState returns the SRS state for a problem.
func GetSRSState(db *sqlx.DB, problemID int) (*srs.ProblemSRS, error) {
	var s srs.ProblemSRS
	err := db.Get(&s, `
		SELECT problem_id, easiness_factor, interval_days, repetition_count,
		       next_review_date, mastered_before
		FROM problem_srs WHERE problem_id = ?`, problemID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// RecordAttempt inserts a completed attempt row.
func RecordAttempt(db *sqlx.DB, problemID int, startedAt, completedAt time.Time, result string, durationSec, quality int) error {
	_, err := db.Exec(`
		INSERT INTO attempts (problem_id, started_at, completed_at, result, duration_seconds, quality)
		VALUES (?, ?, ?, ?, ?, ?)`,
		problemID,
		startedAt.UTC().Format(datetimeFormat),
		completedAt.UTC().Format(datetimeFormat),
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

func filterClauses(f PracticeFilter) (string, []interface{}) {
	var join string
	var args []interface{}
	if f.PlaylistID != nil {
		join += " JOIN playlist_problems pp ON pp.problem_id = p.id AND pp.playlist_id = ?"
		args = append(args, *f.PlaylistID)
	}
	if f.TopicID != nil {
		join += " JOIN problem_topics pt ON pt.problem_id = p.id AND pt.topic_id = ?"
		args = append(args, *f.TopicID)
	}
	return join, args
}
