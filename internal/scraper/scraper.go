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
// Uses headless Rod with the persistent browser profile. Timeout: 30 seconds.
func Scrape(platform, slug, profileDir string) (ProblemMeta, error) {
	tmpl := viper.GetString("platforms." + platform + ".url_template")
	if tmpl == "" {
		return ProblemMeta{}, fmt.Errorf("no URL template configured for platform %q", platform)
	}
	pageURL := strings.ReplaceAll(tmpl, "{{slug}}", slug)

	u := launcher.New().UserDataDir(profileDir).Headless(true).MustLaunch()
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
		el := page.MustElement(`h1`)
		el.MustWaitVisible()
		meta.Title = strings.TrimSpace(el.MustText())
	})

	// Difficulty: <span class="difficulty-pill medium">Medium</span>
	// Wait for the element to appear (JS-rendered), then check class attribute and text.
	rod.Try(func() {
		el := page.MustElement(`.difficulty-pill`)
		el.MustWaitVisible()
		class, _ := el.Attribute("class")
		if class != nil {
			for _, d := range []string{"easy", "medium", "hard"} {
				if strings.Contains(*class, d) {
					meta.Difficulty = d
					break
				}
			}
		}
		if meta.Difficulty == "" {
			text := strings.TrimSpace(strings.ToLower(el.MustText()))
			if text == "easy" || text == "medium" || text == "hard" {
				meta.Difficulty = text
			}
		}
	})

	// Topics: find the <summary> with text "Topics", get its parent <details>,
	// then collect a.company-tag-reveal-btn links within it.
	// (company-tags-container is reused elsewhere so we anchor on the summary.)
	var topics []string
	rod.Try(func() {
		summaries := page.MustElements(`summary`)
		for _, s := range summaries {
			if strings.TrimSpace(s.MustText()) != "Topics" {
				continue
			}
			parent := s.MustParent()
			links, _ := parent.Elements(`a.company-tag-reveal-btn`)
			for _, a := range links {
				// href="/practice/problem-list/binary-search" → "binary-search"
				href, _ := a.Attribute("href")
				if href != nil && *href != "" {
					parts := strings.Split(strings.Trim(*href, "/"), "/")
					slug := parts[len(parts)-1]
					if slug != "" {
						topics = append(topics, slug)
					}
				}
			}
			break
		}
	})
	meta.Topics = dedup(topics)

	if meta.Title == "" {
		return meta, fmt.Errorf("could not extract title from NeetCode page")
	}
	if meta.Difficulty == "" {
		return meta, fmt.Errorf("could not extract difficulty from NeetCode page")
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
