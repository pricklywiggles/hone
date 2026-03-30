package store

import (
	"database/sql"
	"testing"

	"github.com/pricklywiggles/hone/internal/db"
)

func TestCreatePlaylist(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		d, err := db.OpenMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()

		id, err := CreatePlaylist(d, "blind-75")
		if err != nil {
			t.Fatal(err)
		}
		if id == 0 {
			t.Error("expected non-zero ID")
		}
	})

	t.Run("duplicate name returns error", func(t *testing.T) {
		d, err := db.OpenMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()

		if _, err := CreatePlaylist(d, "blind-75"); err != nil {
			t.Fatal(err)
		}
		_, err = CreatePlaylist(d, "blind-75")
		if err == nil {
			t.Error("expected error for duplicate name, got nil")
		}
	})
}

func TestListPlaylists(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		d, err := db.OpenMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()

		playlists, err := ListPlaylists(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(playlists) != 0 {
			t.Errorf("expected 0 playlists, got %d", len(playlists))
		}
	})

	t.Run("problem counts are correct", func(t *testing.T) {
		d, err := db.OpenMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()

		d.MustExec(`INSERT INTO problems (platform, slug, title, difficulty) VALUES (?, ?, ?, ?)`, "leetcode", "two-sum", "Two Sum", "easy")
		d.MustExec(`INSERT INTO problems (platform, slug, title, difficulty) VALUES (?, ?, ?, ?)`, "leetcode", "add-two-numbers", "Add Two Numbers", "medium")

		if _, err := CreatePlaylist(d, "my-list"); err != nil {
			t.Fatal(err)
		}
		if _, err := CreatePlaylist(d, "empty-list"); err != nil {
			t.Fatal(err)
		}

		d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id) VALUES (1, 1)`)
		d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id) VALUES (1, 2)`)

		playlists, err := ListPlaylists(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(playlists) != 2 {
			t.Fatalf("expected 2 playlists, got %d", len(playlists))
		}

		// ListPlaylists orders by name; "empty-list" < "my-list"
		if playlists[0].Name != "empty-list" || playlists[0].ProblemCount != 0 {
			t.Errorf("empty-list: got name=%q count=%d", playlists[0].Name, playlists[0].ProblemCount)
		}
		if playlists[1].Name != "my-list" || playlists[1].ProblemCount != 2 {
			t.Errorf("my-list: got name=%q count=%d", playlists[1].Name, playlists[1].ProblemCount)
		}
	})
}

func TestGetPlaylistByName(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		d, err := db.OpenMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()

		if _, err := CreatePlaylist(d, "blind-75"); err != nil {
			t.Fatal(err)
		}

		p, err := GetPlaylistByName(d, "blind-75")
		if err != nil {
			t.Fatal(err)
		}
		if p.Name != "blind-75" {
			t.Errorf("expected name=blind-75, got %q", p.Name)
		}
	})

	t.Run("not found returns sql.ErrNoRows", func(t *testing.T) {
		d, err := db.OpenMemory()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()

		_, err = GetPlaylistByName(d, "nonexistent")
		if err != sql.ErrNoRows {
			t.Errorf("expected sql.ErrNoRows, got %v", err)
		}
	})
}

func TestPlaylistProblemCount(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.MustExec(`INSERT INTO problems (platform, slug, title, difficulty) VALUES (?, ?, ?, ?)`, "leetcode", "two-sum", "Two Sum", "easy")
	d.MustExec(`INSERT INTO problems (platform, slug, title, difficulty) VALUES (?, ?, ?, ?)`, "leetcode", "add-two-numbers", "Add Two Numbers", "medium")

	if _, err := CreatePlaylist(d, "my-list"); err != nil {
		t.Fatal(err)
	}
	d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id) VALUES (1, 1)`)
	d.MustExec(`INSERT INTO playlist_problems (playlist_id, problem_id) VALUES (1, 2)`)

	p, err := GetPlaylistByName(d, "my-list")
	if err != nil {
		t.Fatal(err)
	}
	if p.ProblemCount != 2 {
		t.Errorf("expected problem count 2, got %d", p.ProblemCount)
	}
}
