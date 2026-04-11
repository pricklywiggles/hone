package store

import (
	"fmt"
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
	// Use localToday() since next_review_date is stored in local time.
	today := localToday()
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = 1`, yesterday)
	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = 2`, today)
	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = 3`, tomorrow)

	problem, _, due, err := PickNext(d, PracticeFilter{})
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
	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = 1`, time.Now().AddDate(0, 0, 3).Format("2006-01-02"))
	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = 2`, time.Now().AddDate(0, 0, 1).Format("2006-01-02"))

	problem, _, due, err := PickNext(d, PracticeFilter{})
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

	problem, state, due, err := PickNext(d, PracticeFilter{})
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
	d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id, position) VALUES (1, 2, 0)`)

	// Both due
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	d.MustExec(`UPDATE problem_srs SET next_review_date = ?`, yesterday)

	playlistID := 1
	problem, _, _, err := PickNext(d, PracticeFilter{PlaylistID: &playlistID})
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
	if err := d.Get(&got, `SELECT * FROM problem_srs WHERE problem_id = 1`); err != nil {
		t.Fatal(err)
	}

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

func TestPickNext_TiebreakByDifficulty(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Seed in reverse difficulty order to ensure the sort isn't just insertion order.
	seedProblem(t, d, "leetcode", "hard-problem", "hard")
	seedProblem(t, d, "leetcode", "medium-problem", "medium")
	seedProblem(t, d, "leetcode", "easy-problem", "easy")

	// All share the same next_review_date (today, set by default).
	problem, _, _, err := PickNext(d, PracticeFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if problem.Difficulty != "easy" {
		t.Errorf("expected easy first, got %s (%s)", problem.Difficulty, problem.Slug)
	}

	// Advance the easy problem so it's no longer due.
	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = ?`, time.Now().AddDate(0, 0, 7).Format("2006-01-02"), problem.ID)

	problem, _, _, err = PickNext(d, PracticeFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if problem.Difficulty != "medium" {
		t.Errorf("expected medium next, got %s (%s)", problem.Difficulty, problem.Slug)
	}
}

func TestPickNext_TiebreakPlaylistOrder(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Three problems, all same difficulty.
	seedProblem(t, d, "leetcode", "alpha", "medium")   // id 1
	seedProblem(t, d, "leetcode", "beta", "medium")    // id 2
	seedProblem(t, d, "leetcode", "gamma", "medium")   // id 3

	d.MustExec(`INSERT INTO playlists (name) VALUES ('ordered')`)
	// Insert in non-id order: 3, 1, 2
	d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id, position) VALUES (1, 3, 0)`)
	d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id, position) VALUES (1, 1, 1)`)
	d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id, position) VALUES (1, 2, 2)`)

	playlistID := 1
	filter := PracticeFilter{PlaylistID: &playlistID}

	problem, _, _, err := PickNext(d, filter)
	if err != nil {
		t.Fatal(err)
	}
	if problem.Slug != "gamma" {
		t.Errorf("expected gamma (position 0) first, got %s", problem.Slug)
	}

	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = ?`, time.Now().AddDate(0, 0, 7).Format("2006-01-02"), problem.ID)

	problem, _, _, err = PickNext(d, filter)
	if err != nil {
		t.Fatal(err)
	}
	if problem.Slug != "alpha" {
		t.Errorf("expected alpha (position 1) next, got %s", problem.Slug)
	}
}

func TestPickNext_TiebreakRandomWithoutPlaylist(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Five problems, all same difficulty and same review date.
	for i := 0; i < 5; i++ {
		seedProblem(t, d, "leetcode", fmt.Sprintf("prob-%d", i), "medium")
	}

	seen := make(map[int]bool)
	for range 30 {
		problem, _, _, err := PickNext(d, PracticeFilter{})
		if err != nil {
			t.Fatal(err)
		}
		seen[problem.ID] = true
	}

	if len(seen) < 2 {
		t.Errorf("expected random tiebreaking to pick at least 2 different problems across 30 calls, but only saw %d", len(seen))
	}
}

func TestListPickQueue(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	seedProblem(t, d, "leetcode", "two-sum", "easy")
	seedProblem(t, d, "leetcode", "add-two-numbers", "medium")
	seedProblem(t, d, "leetcode", "longest-substring", "hard")

	// problem 1 = yesterday (due), problem 2 = today (due), problem 3 = tomorrow (upcoming)
	today := localToday()
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = 1`, yesterday)
	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = 2`, today)
	d.MustExec(`UPDATE problem_srs SET next_review_date = ? WHERE problem_id = 3`, tomorrow)

	queue, err := ListPickQueue(d, PracticeFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(queue))
	}

	// Due problems first, ordered by date (most overdue first)
	if queue[0].Problem.Slug != "two-sum" || !queue[0].IsDue {
		t.Errorf("queue[0]: expected two-sum (due), got %s (due=%v)", queue[0].Problem.Slug, queue[0].IsDue)
	}
	if queue[1].Problem.Slug != "add-two-numbers" || !queue[1].IsDue {
		t.Errorf("queue[1]: expected add-two-numbers (due), got %s (due=%v)", queue[1].Problem.Slug, queue[1].IsDue)
	}
	// Upcoming last
	if queue[2].Problem.Slug != "longest-substring" || queue[2].IsDue {
		t.Errorf("queue[2]: expected longest-substring (upcoming), got %s (due=%v)", queue[2].Problem.Slug, queue[2].IsDue)
	}
}

func TestListPickQueue_Empty(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	queue, err := ListPickQueue(d, PracticeFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 0 {
		t.Errorf("expected empty queue, got %d entries", len(queue))
	}
}

func TestGetTodayStats(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	seedProblem(t, d, "leetcode", "two-sum", "easy")
	seedProblem(t, d, "leetcode", "add-two-numbers", "medium")
	seedProblem(t, d, "leetcode", "longest-substring", "hard")

	// All three due today.
	today := localToday()
	d.MustExec(`UPDATE problem_srs SET next_review_date = ?`, today)

	// No attempts yet: 3 due remaining.
	s, err := GetTodayStats(d)
	if err != nil {
		t.Fatal(err)
	}
	if s.DueRemaining != 3 {
		t.Errorf("DueRemaining = %d, want 3", s.DueRemaining)
	}
	if s.Attempted != 0 || s.Succeeded != 0 {
		t.Errorf("expected 0 attempted/succeeded, got %d/%d", s.Attempted, s.Succeeded)
	}

	// Attempt problem 1 (success).
	now := time.Now().UTC()
	RecordAttempt(d, 1, now, now.Add(5*time.Minute), "success", 300, 5)

	s, err = GetTodayStats(d)
	if err != nil {
		t.Fatal(err)
	}
	if s.Attempted != 1 {
		t.Errorf("Attempted = %d, want 1", s.Attempted)
	}
	if s.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", s.Succeeded)
	}
	if s.DueRemaining != 2 {
		t.Errorf("DueRemaining = %d, want 2", s.DueRemaining)
	}

	// Retry problem 1 (fail) — Attempted goes up but DueRemaining stays at 2.
	RecordAttempt(d, 1, now, now.Add(10*time.Minute), "fail", 600, 1)

	s, err = GetTodayStats(d)
	if err != nil {
		t.Fatal(err)
	}
	if s.Attempted != 2 {
		t.Errorf("Attempted = %d, want 2 (retries count)", s.Attempted)
	}
	if s.DueRemaining != 2 {
		t.Errorf("DueRemaining = %d, want 2 (retry shouldn't decrement)", s.DueRemaining)
	}
}

func TestGetTodayStatsFiltered(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	seedProblem(t, d, "leetcode", "two-sum", "easy")       // id 1
	seedProblem(t, d, "leetcode", "add-two-numbers", "medium") // id 2

	today := localToday()
	d.MustExec(`UPDATE problem_srs SET next_review_date = ?`, today)

	d.MustExec(`INSERT INTO playlists (name) VALUES ('my-list')`)
	d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id, position) VALUES (1, 1, 0)`)

	playlistID := 1
	filter := PracticeFilter{PlaylistID: &playlistID}

	// Only problem 1 is in the playlist.
	s, err := GetTodayStatsFiltered(d, filter)
	if err != nil {
		t.Fatal(err)
	}
	if s.DueRemaining != 1 {
		t.Errorf("DueRemaining = %d, want 1 (only 1 problem in playlist)", s.DueRemaining)
	}

	// Attempt problem 1.
	now := time.Now().UTC()
	RecordAttempt(d, 1, now, now.Add(5*time.Minute), "success", 300, 5)

	s, err = GetTodayStatsFiltered(d, filter)
	if err != nil {
		t.Fatal(err)
	}
	if s.Attempted != 1 {
		t.Errorf("Attempted = %d, want 1", s.Attempted)
	}
	if s.DueRemaining != 0 {
		t.Errorf("DueRemaining = %d, want 0", s.DueRemaining)
	}

	// Attempt problem 2 (not in playlist) — filtered stats shouldn't change.
	RecordAttempt(d, 2, now, now.Add(5*time.Minute), "success", 300, 5)

	s, err = GetTodayStatsFiltered(d, filter)
	if err != nil {
		t.Fatal(err)
	}
	if s.Attempted != 1 {
		t.Errorf("Attempted = %d, want 1 (problem 2 not in playlist)", s.Attempted)
	}
}

func TestResolveFilterName(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if name := ResolveFilterName(d, PracticeFilter{}); name != "" {
		t.Errorf("empty filter: got %q, want empty", name)
	}

	d.MustExec(`INSERT INTO playlists (name) VALUES ('binary trees')`)
	playlistID := 1
	if name := ResolveFilterName(d, PracticeFilter{PlaylistID: &playlistID}); name != "binary trees" {
		t.Errorf("playlist filter: got %q, want %q", name, "binary trees")
	}

	seedProblem(t, d, "leetcode", "two-sum", "easy")
	d.MustExec(`INSERT INTO topics (name) VALUES ('arrays')`)
	topicID := 1
	if name := ResolveFilterName(d, PracticeFilter{TopicID: &topicID}); name != "arrays" {
		t.Errorf("topic filter: got %q, want %q", name, "arrays")
	}
}

func seedProblem(t *testing.T, d *sqlx.DB, platform, slug, difficulty string) {
	t.Helper()
	d.MustExec(
		`INSERT INTO problems (platform, slug, title, difficulty) VALUES (?, ?, ?, ?)`,
		platform, slug, slug, difficulty,
	)
}
