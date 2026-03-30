package platform

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseURL extracts platform and slug from a LeetCode or NeetCode URL.
// Returns error for unrecognized URLs.
// Examples:
//
//	https://leetcode.com/problems/two-sum/        → "leetcode", "two-sum"
//	https://leetcode.com/problems/two-sum/description/ → "leetcode", "two-sum"
//	https://neetcode.io/problems/two-sum/question → "neetcode", "two-sum"
func ParseURL(rawURL string) (platform, slug string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	switch u.Hostname() {
	case "leetcode.com", "www.leetcode.com":
		platform = "leetcode"
	case "neetcode.io", "www.neetcode.io":
		platform = "neetcode"
	default:
		return "", "", fmt.Errorf("unrecognized platform host: %s", u.Hostname())
	}

	// Path looks like /problems/<slug>[/<extra>...]
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "problems" || parts[1] == "" {
		return "", "", fmt.Errorf("URL path does not contain a problem slug: %s", u.Path)
	}

	return platform, parts[1], nil
}
