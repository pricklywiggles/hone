package platform

import (
	"testing"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPlatform string
		wantSlug     string
		wantErr      bool
	}{
		{
			name:         "leetcode basic",
			input:        "https://leetcode.com/problems/two-sum/",
			wantPlatform: "leetcode",
			wantSlug:     "two-sum",
		},
		{
			name:         "leetcode with trailing segment",
			input:        "https://leetcode.com/problems/two-sum/description/",
			wantPlatform: "leetcode",
			wantSlug:     "two-sum",
		},
		{
			name:         "leetcode www subdomain",
			input:        "https://www.leetcode.com/problems/longest-substring-without-repeating-characters/",
			wantPlatform: "leetcode",
			wantSlug:     "longest-substring-without-repeating-characters",
		},
		{
			name:         "neetcode basic",
			input:        "https://neetcode.io/problems/two-sum/question",
			wantPlatform: "neetcode",
			wantSlug:     "two-sum",
		},
		{
			name:         "neetcode without trailing segment",
			input:        "https://neetcode.io/problems/eating-bananas/",
			wantPlatform: "neetcode",
			wantSlug:     "eating-bananas",
		},
		{
			name:    "unknown host",
			input:   "https://hackerrank.com/problems/two-sum/",
			wantErr: true,
		},
		{
			name:    "malformed URL",
			input:   "://not-a-url",
			wantErr: true,
		},
		{
			name:    "no path",
			input:   "https://leetcode.com",
			wantErr: true,
		},
		{
			name:    "path without problems segment",
			input:   "https://leetcode.com/contest/two-sum/",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPlatform, gotSlug, err := ParseURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got platform=%q slug=%q", gotPlatform, gotSlug)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPlatform != tc.wantPlatform {
				t.Errorf("platform = %q, want %q", gotPlatform, tc.wantPlatform)
			}
			if gotSlug != tc.wantSlug {
				t.Errorf("slug = %q, want %q", gotSlug, tc.wantSlug)
			}
		})
	}
}
