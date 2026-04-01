package platform

import (
	"testing"
)

func TestLeetCodeName(t *testing.T) {
	lc := LeetCode{}
	if lc.Name() != "leetcode" {
		t.Errorf("Name = %q, want %q", lc.Name(), "leetcode")
	}
}

func TestLeetCodeHostnames(t *testing.T) {
	lc := LeetCode{}
	hosts := lc.Hostnames()
	want := map[string]bool{"leetcode.com": true, "www.leetcode.com": true}
	for _, h := range hosts {
		if !want[h] {
			t.Errorf("unexpected hostname %q", h)
		}
		delete(want, h)
	}
	for h := range want {
		t.Errorf("missing hostname %q", h)
	}
}

func TestLeetCodeSlugFromPath(t *testing.T) {
	lc := LeetCode{}
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{"basic", "/problems/two-sum/", "two-sum", false},
		{"with trailing segment", "/problems/two-sum/description/", "two-sum", false},
		{"no problems prefix", "/contest/two-sum/", "", true},
		{"empty slug", "/problems//", "", true},
		{"too short", "/problems", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lc.SlugFromPath(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got slug=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("slug = %q, want %q", got, tc.want)
			}
		})
	}
}

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
