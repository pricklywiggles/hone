package store

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/db"
	"github.com/pricklywiggles/hone/internal/srs"
)

func TestPickNext_DueFirst(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Two problems: one overdue, one due today, one tomorrow
	seedProblem(t, d, "leetcode", "two-sum", "easy")
	seedProblem(t, d, "leetcode", "add-two-numbers", "medium")
	seedProblem(t, d, "leetcode", "longest-substring", "hard")

	// Set next_review_date: problem 1 = yesterday, problem 2 = today, problem 3 = tomorrow
	d.MustExec(`UPDATE problem_srs SET next_review_date = date('now', '-1 day') WHERE problem_id = 1`)
	d.MustExec(`UPDATE problem_srs SET next_review_date = date('now') WHERE problem_id = 2`)
	d.MustExec(`UPDATE problem_srs SET next_review_date = date('now', '+1 day') WHERE problem_id = 3`)

	problem, _, due, err := PickNext(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !due {
		t.Error("expected due=true")
	}
	if problem.Slug != "two-sum" {
		t.Errorf("expected most overdue problem (two-sum), got %v", problem.Slug)
	}
}

func TestPickNext_UpcomingWhenNoneDue(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	seedProblem(t, d, "leetcode", "two-sum", "easy")
	seedProblem(t, d, "leetcode", "add-two-numbers", "medium")

	// Both problems scheduled in the future
	d.MustExec(`UPDATE problem_srs SET next_review_date = date('now', '+3 day') WHERE problem_id = 1`)
	d.MustExec(`UPDATE problem_srs SET next_review_date = date('now', '+1 day') WHERE problem_id = 2`)

	problem, _, due, err := PickNext(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	if due {
		t.Error("expected due=false")
	}
	// Should return the soonest upcoming problem
	if problem.Slug != "add-two-numbers" {
		t.Errorf("expected soonest upcoming (add-two-numbers), got %v", problem.Slug)
	}
}

func TestPickNext_NoProblems(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	problem, state, due, err := PickNext(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	if problem != nil || state != nil || due {
		t.Error("expected nil problem, nil state, due=false when no problems exist")
	}
}

func TestPickNext_PlaylistFilter(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	seedProblem(t, d, "leetcode", "two-sum", "easy")
	seedProblem(t, d, "leetcode", "add-two-numbers", "medium")

	// Create playlist and add only problem 2
	d.MustExec(`INSERT INTO playlists (name) VALUES ('my-list')`)
	d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id) VALUES (1, 2)`)

	// Both due
	d.MustExec(`UPDATE problem_srs SET next_review_date = date('now', '-1 day')`)

	playlistID := 1
	problem, _, _, err := PickNext(d, &playlistID)
	if err != nil {
		t.Fatal(err)
	}
	if problem.Slug != "add-two-numbers" {
		t.Errorf("expected playlist-filtered problem (add-two-numbers), got %v", problem.Slug)
	}
}

func TestRecordAttempt(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	seedProblem(t, d, "leetcode", "two-sum", "easy")

	start := time.Now().UTC()
	end := start.Add(8 * time.Minute)

	if err := RecordAttempt(d, 1, start, end, "success", 480, 5); err != nil {
		t.Fatal(err)
	}

	var count int
	d.QueryRow(`SELECT COUNT(*) FROM attempts WHERE problem_id = 1 AND result = 'success'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 attempt, got %v", count)
	}
}

func TestSaveSRSState(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	seedProblem(t, d, "leetcode", "two-sum", "easy")

	state := srs.ProblemSRS{
		ProblemID:       1,
		EasinessFactor:  2.6,
		IntervalDays:    6,
		RepetitionCount: 2,
		NextReviewDate:  "2026-04-04",
		MasteredBefore:  0,
	}

	if err := SaveSRSState(d, state); err != nil {
		t.Fatal(err)
	}

	var got srs.ProblemSRS
	d.Get(&got, `SELECT * FROM problem_srs WHERE problem_id = 1`)

	if got.IntervalDays != 6 {
		t.Errorf("IntervalDays = %v, want 6", got.IntervalDays)
	}
	if got.RepetitionCount != 2 {
		t.Errorf("RepetitionCount = %v, want 2", got.RepetitionCount)
	}
	if got.NextReviewDate != "2026-04-04" {
		t.Errorf("NextReviewDate = %v, want 2026-04-04", got.NextReviewDate)
	}
}

func seedProblem(t *testing.T, d *sqlx.DB, platform, slug, difficulty string) {
	t.Helper()
	d.MustExec(
		`INSERT INTO problems (platform, slug, title, difficulty) VALUES (?, ?, ?, ?)`,
		platform, slug, slug, difficulty,
	)
}
