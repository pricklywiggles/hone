package platform

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pricklywiggles/hone/internal/debuglog"
)

type NeetCode struct{}

func init() { Register(&NeetCode{}) }

func (NeetCode) Name() string { return "neetcode" }

func (NeetCode) Hostnames() []string {
	return []string{"neetcode.io", "www.neetcode.io"}
}

func (NeetCode) SlugFromPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] != "problems" || parts[1] == "" {
		return "", fmt.Errorf("URL path does not contain a problem slug: %s", path)
	}
	return parts[1], nil
}

func (NeetCode) URLTemplate() string {
	return "https://neetcode.io/problems/{{slug}}/question"
}

func (NeetCode) LoginURL() string {
	return "https://neetcode.io/"
}

func (NeetCode) ExtraWait(page *rod.Page) error {
	time.Sleep(3 * time.Second)
	return nil
}

func (NeetCode) Scrape(page *rod.Page) (ProblemMeta, error) {
	var ngMeta ProblemMeta
	if m, err := scrapeNeetCodeNgState(page); err == nil {
		debuglog.Log("neetcode: ng-state succeeded (%s)", m.Title)
		ngMeta = m
	} else {
		debuglog.Log("neetcode: ng-state failed (%v)", err)
	}

	var domMeta ProblemMeta
	rod.Try(func() {
		el := page.MustElement(`h1`)
		el.MustWaitVisible()
		domMeta.Title = strings.TrimSpace(el.MustText())
	})
	rod.Try(func() {
		el := page.MustElement(`.difficulty-pill`)
		el.MustWaitVisible()
		class, _ := el.Attribute("class")
		if class != nil {
			for _, d := range []string{"easy", "medium", "hard"} {
				if strings.Contains(*class, d) {
					domMeta.Difficulty = d
					break
				}
			}
		}
		if domMeta.Difficulty == "" {
			text := strings.TrimSpace(strings.ToLower(el.MustText()))
			if text == "easy" || text == "medium" || text == "hard" {
				domMeta.Difficulty = text
			}
		}
	})
	rod.Try(func() {
		summaries := page.MustElements(`summary`)
		for _, s := range summaries {
			if strings.TrimSpace(s.MustText()) != "Topics" {
				continue
			}
			parent := s.MustParent()
			links, _ := parent.Elements(`a.company-tag-reveal-btn`)
			for _, a := range links {
				href, _ := a.Attribute("href")
				if href != nil && *href != "" {
					parts := strings.Split(strings.Trim(*href, "/"), "/")
					slug := parts[len(parts)-1]
					if slug != "" {
						domMeta.Topics = append(domMeta.Topics, strings.ReplaceAll(slug, "-", " "))
					}
				}
			}
			break
		}
	})
	if domMeta.Title != "" {
		debuglog.Log("neetcode: DOM selectors succeeded (%s)", domMeta.Title)
	} else {
		debuglog.Log("neetcode: DOM selectors failed")
	}

	meta := ngMeta
	if meta.Title == "" {
		meta.Title = domMeta.Title
	}
	if meta.Difficulty == "" {
		meta.Difficulty = domMeta.Difficulty
	}
	meta.Topics = Dedup(append(meta.Topics, domMeta.Topics...))

	if meta.Title == "" {
		return meta, fmt.Errorf("could not extract title from NeetCode page")
	}
	if meta.Difficulty == "" {
		return meta, fmt.Errorf("could not extract difficulty from NeetCode page")
	}
	return meta, nil
}

func scrapeNeetCodeNgState(page *rod.Page) (ProblemMeta, error) {
	var raw string
	if err := rod.Try(func() {
		raw = page.MustElement(`script#ng-state`).MustText()
	}); err != nil {
		return ProblemMeta{}, fmt.Errorf("ng-state not found")
	}
	return parseNeetCodeNgState(raw)
}

// ── Monitor selectors ───────────────────────────────────────────────────────

var neetcodeSuccess = []string{"h1.submission-result-accepted"}
var neetcodeFailure = []string{"h1.submission-result-wrong"}

func (NeetCode) DetectResult(page *rod.Page) (bool, bool) {
	if ElementPresent(page, neetcodeSuccess...) {
		return true, true
	}
	if ElementPresent(page, neetcodeFailure...) {
		return false, true
	}
	return false, false
}

func (NeetCode) ResultIndicatorText(page *rod.Page) string {
	return ElementExists(page, neetcodeSuccess, neetcodeFailure)
}

// ── Pure parsing (testable without browser) ─────────────────────────────────

func parseNeetCodeNgState(raw string) (ProblemMeta, error) {
	var state map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return ProblemMeta{}, fmt.Errorf("failed to parse ng-state: %w", err)
	}

	for key, entry := range state {
		if !strings.HasPrefix(key, "problem-") {
			continue
		}
		debuglog.Log("neetcode: ng-state[%s] = %s", key, string(entry))
		var prob struct {
			Name       string `json:"name"`
			Difficulty string `json:"difficulty"`
		}
		if err := json.Unmarshal(entry, &prob); err != nil || prob.Name == "" {
			continue
		}
		return ProblemMeta{
			Title:      strings.TrimSpace(prob.Name),
			Difficulty: strings.ToLower(strings.TrimSpace(prob.Difficulty)),
		}, nil
	}
	return ProblemMeta{}, fmt.Errorf("no problem data in ng-state")
}
