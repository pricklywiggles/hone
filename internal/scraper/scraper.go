package scraper

import (
	"encoding/json"
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

	chromePath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	u := launcher.New().Bin(chromePath).UserDataDir(profileDir).Headless(true).MustLaunch()
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

	// LeetCode embeds all problem data in a __NEXT_DATA__ script tag.
	var raw string
	if err := rod.Try(func() {
		raw = page.MustElement(`script#__NEXT_DATA__`).MustText()
	}); err != nil {
		return meta, fmt.Errorf("could not find __NEXT_DATA__ on LeetCode page")
	}

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
		return meta, fmt.Errorf("failed to parse __NEXT_DATA__: %w", err)
	}

	for _, q := range nextData.Props.PageProps.DehydratedState.Queries {
		question := q.State.Data.Question
		if question.Title == "" {
			continue
		}
		meta.Title = strings.TrimSpace(question.Title)
		meta.Difficulty = strings.ToLower(strings.TrimSpace(question.Difficulty))
		for _, tag := range question.TopicTags {
			name := strings.TrimSpace(strings.ToLower(tag.Name))
			if name != "" {
				meta.Topics = append(meta.Topics, name)
			}
		}
		break
	}
	meta.Topics = dedup(meta.Topics)

	if meta.Title == "" {
		return meta, fmt.Errorf("could not extract title from LeetCode __NEXT_DATA__")
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
						topics = append(topics, strings.ReplaceAll(slug, "-", " "))
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
