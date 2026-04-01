package platform

import (
	"testing"
)

func TestGeeksForGeeksName(t *testing.T) {
	gfg := GeeksForGeeks{}
	if gfg.Name() != "geeksforgeeks" {
		t.Errorf("Name = %q, want %q", gfg.Name(), "geeksforgeeks")
	}
}

func TestGeeksForGeeksHostnames(t *testing.T) {
	gfg := GeeksForGeeks{}
	hosts := gfg.Hostnames()
	want := map[string]bool{
		"geeksforgeeks.org":          true,
		"www.geeksforgeeks.org":      true,
		"practice.geeksforgeeks.org": true,
	}
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

func TestGeeksForGeeksSlugFromPath(t *testing.T) {
	gfg := GeeksForGeeks{}
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{"with trailing number", "/problems/reverse-a-linked-list/1", "reverse-a-linked-list", false},
		{"without trailing number", "/problems/two-sum/", "two-sum", false},
		{"no problems prefix", "/dsa/some-article/", "", true},
		{"empty slug", "/problems//", "", true},
		{"too short", "/problems", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := gfg.SlugFromPath(tc.path)
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

func TestParseGeeksForGeeks(t *testing.T) {
	t.Run("valid with topics", func(t *testing.T) {
		raw := `{
			"props": {"pageProps": {"initialState": {"problemData": {"allData": {"probData": {
				"problem_name": "Reverse a linked list",
				"difficulty": "Easy",
				"tags": {
					"topic_tags": ["Linked List", "Data Structures"]
				}
			}}}}}}
		}`
		meta, err := parseGeeksForGeeks(raw)
		if err != nil {
			t.Fatal(err)
		}
		if meta.Title != "Reverse a linked list" {
			t.Errorf("Title = %q, want %q", meta.Title, "Reverse a linked list")
		}
		if meta.Difficulty != "easy" {
			t.Errorf("Difficulty = %q, want %q", meta.Difficulty, "easy")
		}
		expected := []string{"linked list", "data structures"}
		if len(meta.Topics) != len(expected) {
			t.Fatalf("Topics = %v, want %v", meta.Topics, expected)
		}
		for i, got := range meta.Topics {
			if got != expected[i] {
				t.Errorf("Topics[%d] = %q, want %q", i, got, expected[i])
			}
		}
	})

	t.Run("normalizes dashed topics", func(t *testing.T) {
		raw := `{
			"props": {"pageProps": {"initialState": {"problemData": {"allData": {"probData": {
				"problem_name": "Test Problem",
				"difficulty": "Medium",
				"tags": {
					"topic_tags": ["Breadth-First-Search", "depth-first-search"]
				}
			}}}}}}
		}`
		meta, err := parseGeeksForGeeks(raw)
		if err != nil {
			t.Fatal(err)
		}
		expected := []string{"breadth first search", "depth first search"}
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
			"props": {"pageProps": {"initialState": {"problemData": {"allData": {"probData": {
				"problem_name": "Test",
				"difficulty": "Hard",
				"tags": {
					"topic_tags": ["Array", "Array", "array"]
				}
			}}}}}}
		}`
		meta, err := parseGeeksForGeeks(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(meta.Topics) != 1 {
			t.Errorf("Topics = %v, want single deduplicated entry", meta.Topics)
		}
	})

	t.Run("empty title returns error", func(t *testing.T) {
		raw := `{
			"props": {"pageProps": {"initialState": {"problemData": {"allData": {"probData": {
				"problem_name": "",
				"difficulty": "Easy",
				"tags": {"topic_tags": []}
			}}}}}}
		}`
		_, err := parseGeeksForGeeks(raw)
		if err == nil {
			t.Error("expected error for empty title")
		}
	})

	t.Run("no topics is valid", func(t *testing.T) {
		raw := `{
			"props": {"pageProps": {"initialState": {"problemData": {"allData": {"probData": {
				"problem_name": "No Tags Problem",
				"difficulty": "Easy",
				"tags": {"topic_tags": []}
			}}}}}}
		}`
		meta, err := parseGeeksForGeeks(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(meta.Topics) != 0 {
			t.Errorf("Topics = %v, want empty", meta.Topics)
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		_, err := parseGeeksForGeeks(`{not valid json`)
		if err == nil {
			t.Error("expected error for malformed JSON")
		}
	})
}
