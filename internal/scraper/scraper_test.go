package scraper

import (
	"testing"
)

func TestParseLeetCodeNextData(t *testing.T) {
	t.Run("valid with dashed topics", func(t *testing.T) {
		raw := `{
			"props": {"pageProps": {"dehydratedState": {"queries": [{
				"state": {"data": {"question": {
					"title": "Two Sum",
					"difficulty": "Easy",
					"topicTags": [
						{"name": "Array"},
						{"name": "Hash-Table"},
						{"name": "Breadth-First-Search"}
					]
				}}}
			}]}}}
		}`
		meta, err := parseLeetCodeNextData(raw)
		if err != nil {
			t.Fatal(err)
		}
		if meta.Title != "Two Sum" {
			t.Errorf("Title = %q, want %q", meta.Title, "Two Sum")
		}
		if meta.Difficulty != "easy" {
			t.Errorf("Difficulty = %q, want %q", meta.Difficulty, "easy")
		}
		expected := []string{"array", "hash table", "breadth first search"}
		if len(meta.Topics) != len(expected) {
			t.Fatalf("Topics = %v, want %v", meta.Topics, expected)
		}
		for i, got := range meta.Topics {
			if got != expected[i] {
				t.Errorf("Topics[%d] = %q, want %q", i, got, expected[i])
			}
		}
	})

	t.Run("deduplicates topics", func(t *testing.T) {
		raw := `{
			"props": {"pageProps": {"dehydratedState": {"queries": [{
				"state": {"data": {"question": {
					"title": "Test",
					"difficulty": "Medium",
					"topicTags": [
						{"name": "Array"},
						{"name": "Array"},
						{"name": "array"}
					]
				}}}
			}]}}}
		}`
		meta, err := parseLeetCodeNextData(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(meta.Topics) != 1 {
			t.Errorf("Topics = %v, want single deduplicated entry", meta.Topics)
		}
	})

	t.Run("empty title returns error", func(t *testing.T) {
		raw := `{
			"props": {"pageProps": {"dehydratedState": {"queries": [{
				"state": {"data": {"question": {
					"title": "",
					"difficulty": "Easy",
					"topicTags": []
				}}}
			}]}}}
		}`
		_, err := parseLeetCodeNextData(raw)
		if err == nil {
			t.Error("expected error for empty title")
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		_, err := parseLeetCodeNextData(`{not valid json`)
		if err == nil {
			t.Error("expected error for malformed JSON")
		}
	})
}

func TestParseNeetCodeNgState(t *testing.T) {
	t.Run("valid problem key", func(t *testing.T) {
		raw := `{
			"some-other-key": {},
			"problem-two-sum": {"name": "Two Sum", "difficulty": "Easy"}
		}`
		meta, err := parseNeetCodeNgState(raw)
		if err != nil {
			t.Fatal(err)
		}
		if meta.Title != "Two Sum" {
			t.Errorf("Title = %q, want %q", meta.Title, "Two Sum")
		}
		if meta.Difficulty != "easy" {
			t.Errorf("Difficulty = %q, want %q", meta.Difficulty, "easy")
		}
	})

	t.Run("no problem keys returns error", func(t *testing.T) {
		raw := `{"user-data": {"name": "test"}, "config": {}}`
		_, err := parseNeetCodeNgState(raw)
		if err == nil {
			t.Error("expected error when no problem-* keys exist")
		}
	})

	t.Run("empty name skipped", func(t *testing.T) {
		raw := `{"problem-empty": {"name": "", "difficulty": "Easy"}}`
		_, err := parseNeetCodeNgState(raw)
		if err == nil {
			t.Error("expected error when problem name is empty")
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		_, err := parseNeetCodeNgState(`not json`)
		if err == nil {
			t.Error("expected error for malformed JSON")
		}
	})
}

func TestDedup(t *testing.T) {
	t.Run("removes duplicates", func(t *testing.T) {
		got := dedup([]string{"a", "b", "a", "c", "b"})
		want := []string{"a", "b", "c"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := dedup([]string{})
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})

	t.Run("no duplicates unchanged", func(t *testing.T) {
		got := dedup([]string{"x", "y", "z"})
		if len(got) != 3 {
			t.Errorf("got %v, want 3 elements", got)
		}
	})
}
