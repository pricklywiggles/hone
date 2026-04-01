package platform

import (
	"testing"
)

func TestNeetCodeName(t *testing.T) {
	nc := NeetCode{}
	if nc.Name() != "neetcode" {
		t.Errorf("Name = %q, want %q", nc.Name(), "neetcode")
	}
}

func TestNeetCodeHostnames(t *testing.T) {
	nc := NeetCode{}
	hosts := nc.Hostnames()
	want := map[string]bool{"neetcode.io": true, "www.neetcode.io": true}
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

func TestNeetCodeSlugFromPath(t *testing.T) {
	nc := NeetCode{}
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{"basic", "/problems/two-sum/question", "two-sum", false},
		{"without trailing segment", "/problems/eating-bananas/", "eating-bananas", false},
		{"no problems prefix", "/challenges/two-sum/", "", true},
		{"empty slug", "/problems//", "", true},
		{"too short", "/problems", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nc.SlugFromPath(tc.path)
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
