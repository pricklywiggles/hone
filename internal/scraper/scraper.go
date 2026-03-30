package scraper

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/spf13/viper"
)

type ProblemMeta struct {
	Title      string
	Difficulty string   // "easy", "medium", or "hard"
	Topics     []string // e.g. ["array", "hash-table"]
}

// Scrape fetches problem metadata from the platform page for the given slug.
// Uses headless Rod. Timeout: 30 seconds.
func Scrape(platform, slug string) (ProblemMeta, error) {
	tmpl := viper.GetString("platforms." + platform + ".url_template")
	if tmpl == "" {
		return ProblemMeta{}, fmt.Errorf("no URL template configured for platform %q", platform)
	}
	pageURL := strings.ReplaceAll(tmpl, "{{slug}}", slug)

	u := launcher.New().Headless(true).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(pageURL)
	page = page.Timeout(30 * time.Second)

	if err := page.WaitLoad(); err != nil {
		return ProblemMeta{}, fmt.Errorf("page load timeout: %w", err)
	}
	_ = page.WaitIdle(2 * time.Second)

	switch platform {
	case "leetcode":
		return scrapeLeetCode(page)
	case "neetcode":
		return scrapeNeetCode(page)
	default:
		return ProblemMeta{}, fmt.Errorf("unsupported platform: %s", platform)
	}
}

func scrapeLeetCode(page *rod.Page) (ProblemMeta, error) {
	var meta ProblemMeta

	// Title: try data-cy attribute first, fall back to h1 in the title area
	var title string
	if err := rod.Try(func() {
		title = page.MustElement(`div[data-cy="question-title"]`).MustText()
	}); err != nil {
		if err2 := rod.Try(func() {
			title = page.MustElement(`.mr-2.text-label-1`).MustText()
		}); err2 != nil {
			rod.Try(func() {
				title = page.MustElement(`h1`).MustText()
			})
		}
	}
	meta.Title = strings.TrimSpace(title)

	// Difficulty
	for _, d := range []string{"easy", "medium", "hard"} {
		if err := rod.Try(func() {
			el := page.MustElement(`.text-difficulty-` + d)
			_ = el.MustText()
			meta.Difficulty = d
		}); err == nil {
			break
		}
	}

	// Topics
	var topics []string
	rod.Try(func() {
		els := page.MustElements(`a[href*="/tag/"]`)
		for _, el := range els {
			t := strings.TrimSpace(strings.ToLower(el.MustText()))
			if t != "" {
				topics = append(topics, t)
			}
		}
	})
	meta.Topics = dedup(topics)

	if meta.Title == "" {
		return meta, fmt.Errorf("could not extract title from LeetCode page")
	}
	return meta, nil
}

func scrapeNeetCode(page *rod.Page) (ProblemMeta, error) {
	var meta ProblemMeta

	rod.Try(func() {
		meta.Title = strings.TrimSpace(page.MustElement(`h1`).MustText())
	})

	// Difficulty: look for a span/div containing "Easy", "Medium", or "Hard"
	for _, d := range []string{"Easy", "Medium", "Hard"} {
		if err := rod.Try(func() {
			els := page.MustElements(`span, div`)
			for _, el := range els {
				if strings.TrimSpace(el.MustText()) == d {
					meta.Difficulty = strings.ToLower(d)
					return
				}
			}
		}); err == nil && meta.Difficulty != "" {
			break
		}
	}

	// Topics: links or spans near a "Topics" label
	var topics []string
	rod.Try(func() {
		els := page.MustElements(`a[href*="/problems/"]`)
		for _, el := range els {
			t := strings.TrimSpace(strings.ToLower(el.MustText()))
			if t != "" && t != meta.Title {
				topics = append(topics, t)
			}
		}
	})
	meta.Topics = dedup(topics)

	if meta.Title == "" {
		return meta, fmt.Errorf("could not extract title from NeetCode page")
	}
	return meta, nil
}

func dedup(ss []string) []string {
	seen := make(map[string]struct{}, len(ss))
	out := ss[:0]
	for _, s := range ss {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
