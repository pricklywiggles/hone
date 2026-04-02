package backup

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/pricklywiggles/hone/internal/db"
	"github.com/pricklywiggles/hone/internal/store"
	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("platforms.leetcode.url_template", "https://leetcode.com/problems/{{slug}}/")
	viper.SetDefault("platforms.neetcode.url_template", "https://neetcode.io/problems/{{slug}}/question")
}

func TestExportFullBackup_Empty(t *testing.T) {
	testDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()

	data, err := ExportFullBackup(testDB)
	if err != nil {
		t.Fatal(err)
	}
	if data.Version != backupVersion {
		t.Errorf("version: got %d, want %d", data.Version, backupVersion)
	}
	if len(data.Problems) != 0 {
		t.Errorf("expected 0 problems, got %d", len(data.Problems))
	}
}

func TestRoundTrip(t *testing.T) {
	srcDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer srcDB.Close()

	// Seed source DB.
	id1, err := store.InsertProblem(srcDB, "leetcode", "two-sum", "Two Sum", "easy", []string{"array", "hash table"})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := store.InsertProblem(srcDB, "neetcode", "valid-anagram", "Valid Anagram", "easy", []string{"string"})
	if err != nil {
		t.Fatal(err)
	}

	plID, err := store.CreatePlaylist(srcDB, "Favorites")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddProblemToPlaylist(srcDB, int(plID), int(id1)); err != nil {
		t.Fatal(err)
	}
	if err := store.AddProblemToPlaylist(srcDB, int(plID), int(id2)); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	if err := store.RecordAttempt(srcDB, int(id2), now, now.Add(10*60*1e9), "success", 600, 5); err != nil {
		t.Fatal(err)
	}

	// Export.
	data, err := ExportFullBackup(srcDB)
	if err != nil {
		t.Fatal(err)
	}

	// Round-trip through JSON.
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var data2 BackupData
	if err := json.Unmarshal(raw, &data2); err != nil {
		t.Fatal(err)
	}

	// Restore into a fresh DB.
	dstDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer dstDB.Close()

	if err := RestoreFromBackup(dstDB, data2); err != nil {
		t.Fatal(err)
	}

	// Verify counts.
	var problemCount int
	if err := dstDB.QueryRow(`SELECT COUNT(*) FROM problems`).Scan(&problemCount); err != nil {
		t.Fatal(err)
	}
	if problemCount != 2 {
		t.Errorf("problems: got %d, want 2", problemCount)
	}

	var playlistCount int
	if err := dstDB.QueryRow(`SELECT COUNT(*) FROM playlists`).Scan(&playlistCount); err != nil {
		t.Fatal(err)
	}
	if playlistCount != 1 {
		t.Errorf("playlists: got %d, want 1", playlistCount)
	}

	var ppCount int
	if err := dstDB.QueryRow(`SELECT COUNT(*) FROM playlist_problems`).Scan(&ppCount); err != nil {
		t.Fatal(err)
	}
	if ppCount != 2 {
		t.Errorf("playlist_problems: got %d, want 2", ppCount)
	}

	// Verify positions survived the round-trip.
	rows, err := dstDB.Query(`SELECT position FROM playlist_problems ORDER BY position`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var positions []int
	for rows.Next() {
		var pos int
		if err := rows.Scan(&pos); err != nil {
			t.Fatal(err)
		}
		positions = append(positions, pos)
	}
	if len(positions) != 2 || positions[0] != 0 || positions[1] != 1 {
		t.Errorf("playlist_problems positions: got %v, want [0 1]", positions)
	}

	var attemptCount int
	if err := dstDB.QueryRow(`SELECT COUNT(*) FROM attempts`).Scan(&attemptCount); err != nil {
		t.Fatal(err)
	}
	if attemptCount != 1 {
		t.Errorf("attempts: got %d, want 1", attemptCount)
	}

	// Verify topic links survived.
	var topicCount int
	if err := dstDB.QueryRow(`SELECT COUNT(*) FROM problem_topics`).Scan(&topicCount); err != nil {
		t.Fatal(err)
	}
	if topicCount != 3 { // array + hash table + string
		t.Errorf("problem_topics: got %d, want 3", topicCount)
	}
}

func TestExportPlaylistFormat(t *testing.T) {
	testDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()

	id1, _ := store.InsertProblem(testDB, "leetcode", "two-sum", "Two Sum", "easy", nil)
	id2, _ := store.InsertProblem(testDB, "neetcode", "valid-anagram", "Valid Anagram", "easy", nil)
	id3, _ := store.InsertProblem(testDB, "leetcode", "climbing-stairs", "Climbing Stairs", "easy", nil)

	plID, _ := store.CreatePlaylist(testDB, "Week 1")
	store.AddProblemToPlaylist(testDB, int(plID), int(id2))
	store.AddProblemToPlaylist(testDB, int(plID), int(id3))
	// id1 is unowned

	out, err := ExportPlaylistFormat(testDB)
	if err != nil {
		t.Fatal(err)
	}

	_ = id1
	if out == "" {
		t.Error("expected non-empty output")
	}

	// Should contain the unowned problem URL at the top.
	if !contains(out, "leetcode.com/problems/two-sum") {
		t.Errorf("missing unowned problem URL in output:\n%s", out)
	}
	// Should contain playlist header.
	if !contains(out, "# Week 1") {
		t.Errorf("missing playlist header in output:\n%s", out)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
