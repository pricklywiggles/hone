package store

import (
	"testing"

	"github.com/pricklywiggles/hone/internal/db"
)

func TestInsertProblem_HappyPath(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id, err := InsertProblem(d, "leetcode", "two-sum", "Two Sum", "easy", []string{"array", "hash-table"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero problem ID")
	}

	var count int
	d.QueryRow(`SELECT COUNT(*) FROM problem_topics WHERE problem_id = ?`, id).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 topic links, got %d", count)
	}

	// Trigger should have auto-created the SRS row.
	var srsCount int
	d.QueryRow(`SELECT COUNT(*) FROM problem_srs WHERE problem_id = ?`, id).Scan(&srsCount)
	if srsCount != 1 {
		t.Errorf("expected SRS row to be auto-created by trigger, got %d", srsCount)
	}
}

func TestInsertProblem_Duplicate(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if _, err := InsertProblem(d, "leetcode", "two-sum", "Two Sum", "easy", nil); err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = InsertProblem(d, "leetcode", "two-sum", "Two Sum", "easy", nil)
	if err == nil {
		t.Error("expected error for duplicate (platform, slug), got nil")
	}
}

func TestInsertProblem_NoTopics(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id, err := InsertProblem(d, "neetcode", "eating-bananas", "Eating Bananas", "medium", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int
	d.QueryRow(`SELECT COUNT(*) FROM problem_topics WHERE problem_id = ?`, id).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 topic links, got %d", count)
	}
}

func TestInsertProblem_SharedTopics(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	_, err = InsertProblem(d, "leetcode", "two-sum", "Two Sum", "easy", []string{"array", "hash-table"})
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = InsertProblem(d, "leetcode", "four-sum", "Four Sum", "hard", []string{"array", "sorting"})
	if err != nil {
		t.Fatalf("second insert failed: %v", err)
	}

	// "array" should appear only once in the topics table.
	var count int
	d.QueryRow(`SELECT COUNT(*) FROM topics WHERE name = 'array'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected topic 'array' to exist exactly once, got %d", count)
	}

	// Total distinct topics: array, hash-table, sorting = 3
	d.QueryRow(`SELECT COUNT(*) FROM topics`).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 total topics, got %d", count)
	}
}
