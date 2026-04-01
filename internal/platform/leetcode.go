package platform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	"github.com/pricklywiggles/hone/internal/debuglog"
)

type LeetCode struct{}

func init() { Register(&LeetCode{}) }

func (LeetCode) Name() string { return "leetcode" }

func (LeetCode) Hostnames() []string {
	return []string{"leetcode.com", "www.leetcode.com"}
}

func (LeetCode) SlugFromPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] != "problems" || parts[1] == "" {
		return "", fmt.Errorf("URL path does not contain a problem slug: %s", path)
	}
	return parts[1], nil
}

func (LeetCode) URLTemplate() string {
	return "https://leetcode.com/problems/{{slug}}/"
}

func (LeetCode) LoginURL() string {
	return "https://leetcode.com/accounts/login/"
}

func (LeetCode) ExtraWait(page *rod.Page) error { return nil }

func (LeetCode) Scrape(page *rod.Page) (ProblemMeta, error) {
	var raw string
	if err := rod.Try(func() {
		raw = page.MustElement(`script#__NEXT_DATA__`).MustText()
	}); err != nil {
		debuglog.Log("leetcode: __NEXT_DATA__ not found")
		return ProblemMeta{}, fmt.Errorf("could not find __NEXT_DATA__ on LeetCode page")
	}

	meta, err := parseLeetCodeNextData(raw)
	if err != nil {
		debuglog.Log("leetcode: __NEXT_DATA__ found but parsing failed: %v", err)
		return meta, err
	}
	debuglog.Log("leetcode: __NEXT_DATA__ succeeded (%s)", meta.Title)
	return meta, nil
}

// ── Monitor selectors ───────────────────────────────────────────────────────

var leetcodeResult = []string{
	"h3.text-green-60",
	"h3.text-red-60",
	"[data-e2e-locator='submission-result']",
	"[data-e2e-locator='console-result']",
}

func (LeetCode) DetectResult(page *rod.Page) (bool, bool) {
	text := strings.ToLower(TextOf(page, leetcodeResult...))
	if strings.Contains(text, "accepted") {
		return true, true
	}
	if text != "" {
		return false, true
	}
	return false, false
}

func (LeetCode) ResultIndicatorText(page *rod.Page) string {
	return TextOf(page, leetcodeResult...)
}

// ── Pure parsing (testable without browser) ─────────────────────────────────

func parseLeetCodeNextData(raw string) (ProblemMeta, error) {
	var nextData struct {
		Props struct {
			PageProps struct {
				DehydratedState struct {
					Queries []struct {
						State struct {
							Data struct {
								Question struct {
									Title      string `json:"title"`
									Difficulty string `json:"difficulty"`
									TopicTags  []struct {
										Name string `json:"name"`
									} `json:"topicTags"`
								} `json:"question"`
							} `json:"data"`
						} `json:"state"`
					} `json:"queries"`
				} `json:"dehydratedState"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal([]byte(raw), &nextData); err != nil {
		return ProblemMeta{}, fmt.Errorf("failed to parse __NEXT_DATA__: %w", err)
	}

	var meta ProblemMeta
	for _, q := range nextData.Props.PageProps.DehydratedState.Queries {
		question := q.State.Data.Question
		if question.Title == "" {
			continue
		}
		meta.Title = strings.TrimSpace(question.Title)
		meta.Difficulty = strings.ToLower(strings.TrimSpace(question.Difficulty))
		for _, tag := range question.TopicTags {
			name := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(tag.Name)), "-", " ")
			if name != "" {
				meta.Topics = append(meta.Topics, name)
			}
		}
		break
	}
	meta.Topics = Dedup(meta.Topics)

	if meta.Title == "" {
		return meta, fmt.Errorf("could not extract title from LeetCode __NEXT_DATA__")
	}
	return meta, nil
}
