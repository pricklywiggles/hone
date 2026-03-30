package importer

import (
	"os"
	"testing"
)

func TestParseImportFile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ImportGroup
	}{
		{
			name:  "empty file",
			input: "",
		},
		{
			name:  "only blanks and comments",
			input: "\n// comment\n\n// another\n",
		},
		{
			name:  "urls with no playlist header",
			input: "https://leetcode.com/problems/two-sum/\nhttps://leetcode.com/problems/add-two-numbers/\n",
			expected: []ImportGroup{
				{Playlist: "", URLs: []string{
					"https://leetcode.com/problems/two-sum/",
					"https://leetcode.com/problems/add-two-numbers/",
				}},
			},
		},
		{
			name:  "single playlist",
			input: "# Favorites\nhttps://leetcode.com/problems/two-sum/\nhttps://neetcode.io/problems/valid-anagram/question\n",
			expected: []ImportGroup{
				{Playlist: "Favorites", URLs: []string{
					"https://leetcode.com/problems/two-sum/",
					"https://neetcode.io/problems/valid-anagram/question",
				}},
			},
		},
		{
			name: "urls before and after playlist",
			input: "https://leetcode.com/problems/two-sum/\n# Week 1\nhttps://neetcode.io/problems/valid-anagram/question\n",
			expected: []ImportGroup{
				{Playlist: "", URLs: []string{"https://leetcode.com/problems/two-sum/"}},
				{Playlist: "Week 1", URLs: []string{"https://neetcode.io/problems/valid-anagram/question"}},
			},
		},
		{
			name: "multiple playlists",
			input: "# Favorites\nhttps://leetcode.com/problems/two-sum/\n\n# Week 1\n// a comment\nhttps://neetcode.io/problems/valid-anagram/question\nhttps://leetcode.com/problems/add-two-numbers/\n",
			expected: []ImportGroup{
				{Playlist: "Favorites", URLs: []string{"https://leetcode.com/problems/two-sum/"}},
				{Playlist: "Week 1", URLs: []string{
					"https://neetcode.io/problems/valid-anagram/question",
					"https://leetcode.com/problems/add-two-numbers/",
				}},
			},
		},
		{
			name:  "playlist header with no urls is skipped",
			input: "# Empty\n# Favorites\nhttps://leetcode.com/problems/two-sum/\n",
			expected: []ImportGroup{
				{Playlist: "Favorites", URLs: []string{"https://leetcode.com/problems/two-sum/"}},
			},
		},
		{
			name:  "playlist name is trimmed",
			input: "#  My List  \nhttps://leetcode.com/problems/two-sum/\n",
			expected: []ImportGroup{
				{Playlist: "My List", URLs: []string{"https://leetcode.com/problems/two-sum/"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "import-test-*.txt")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())
			if _, err := f.WriteString(tc.input); err != nil {
				t.Fatal(err)
			}
			f.Close()

			got, err := ParseImportFile(f.Name())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tc.expected) {
				t.Fatalf("got %d groups, want %d\n  got:  %+v\n  want: %+v", len(got), len(tc.expected), got, tc.expected)
			}
			for i, g := range got {
				exp := tc.expected[i]
				if g.Playlist != exp.Playlist {
					t.Errorf("group %d playlist: got %q, want %q", i, g.Playlist, exp.Playlist)
				}
				if len(g.URLs) != len(exp.URLs) {
					t.Errorf("group %d URL count: got %d, want %d", i, len(g.URLs), len(exp.URLs))
					continue
				}
				for j, u := range g.URLs {
					if u != exp.URLs[j] {
						t.Errorf("group %d URL %d: got %q, want %q", i, j, u, exp.URLs[j])
					}
				}
			}
		})
	}
}

func TestParseImportFile_NotFound(t *testing.T) {
	_, err := ParseImportFile("/nonexistent/path.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
