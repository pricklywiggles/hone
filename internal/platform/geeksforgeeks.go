package platform

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pricklywiggles/hone/internal/debuglog"
)

type GeeksForGeeks struct{}

func init() { Register(&GeeksForGeeks{}) }

func (GeeksForGeeks) Name() string { return "geeksforgeeks" }

func (GeeksForGeeks) Hostnames() []string {
	return []string{"geeksforgeeks.org", "www.geeksforgeeks.org", "practice.geeksforgeeks.org"}
}

func (GeeksForGeeks) SlugFromPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] != "problems" || parts[1] == "" {
		return "", fmt.Errorf("URL path does not contain a problem slug: %s", path)
	}
	return parts[1], nil
}

func (GeeksForGeeks) URLTemplate() string {
	return "https://www.geeksforgeeks.org/problems/{{slug}}/1"
}

func (GeeksForGeeks) LoginURL() string {
	return "https://auth.geeksforgeeks.org/"
}

func (GeeksForGeeks) ExtraWait(page *rod.Page) error {
	time.Sleep(2 * time.Second)
	return nil
}

func (GeeksForGeeks) Scrape(page *rod.Page) (ProblemMeta, error) {
	var raw string
	if err := rod.Try(func() {
		raw = page.MustElement(`script#__NEXT_DATA__`).MustText()
	}); err != nil {
		debuglog.Log("geeksforgeeks: __NEXT_DATA__ not found")
		return ProblemMeta{}, fmt.Errorf("could not find __NEXT_DATA__ on GeeksForGeeks page")
	}

	meta, err := parseGeeksForGeeks(raw)
	if err != nil {
		debuglog.Log("geeksforgeeks: __NEXT_DATA__ found but parsing failed: %v", err)
		return meta, err
	}
	debuglog.Log("geeksforgeeks: __NEXT_DATA__ succeeded (%s)", meta.Title)
	return meta, nil
}

// ── Monitor selectors ───────────────────────────────────────────────────────
//
// GFG shows an Output Window overlay after submission with an h3 containing
// either "Problem Solved Successfully" or an error like "Wrong Answer. !!!".
// Icon classes (Semantic UI) serve as fallbacks if the hashed container class changes.

var gfgFailureKeywords = []string{
	"wrong answer",
	"compilation error",
	"time limit exceeded",
	"runtime error",
	"memory limit exceeded",
}

var gfgResult = []string{
	"[class*='problems_output_window'] h3",
	"h3:has(> i.check.circle.icon)",
	"h3:has(> i.exclamation.circle.icon)",
}

func (GeeksForGeeks) DetectResult(page *rod.Page) (bool, bool) {
	return classifyGfgResult(TextOf(page, gfgResult...))
}

func classifyGfgResult(text string) (success, found bool) {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "problem solved successfully") {
		return true, true
	}
	for _, fail := range gfgFailureKeywords {
		if strings.Contains(lower, fail) {
			return false, true
		}
	}
	return false, false
}

func (GeeksForGeeks) ResultIndicatorText(page *rod.Page) string {
	return TextOf(page, gfgResult...)
}

// ── Pure parsing (testable without browser) ─────────────────────────────────

func parseGeeksForGeeks(raw string) (ProblemMeta, error) {
	var nextData struct {
		Props struct {
			PageProps struct {
				InitialState struct {
					ProblemData struct {
						AllData struct {
							ProbData struct {
								ProblemName string `json:"problem_name"`
								Difficulty  string `json:"difficulty"`
								Tags        struct {
									TopicTags []string `json:"topic_tags"`
								} `json:"tags"`
							} `json:"probData"`
						} `json:"allData"`
					} `json:"problemData"`
				} `json:"initialState"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal([]byte(raw), &nextData); err != nil {
		return ProblemMeta{}, fmt.Errorf("failed to parse __NEXT_DATA__: %w", err)
	}

	prob := nextData.Props.PageProps.InitialState.ProblemData.AllData.ProbData
	if prob.ProblemName == "" {
		return ProblemMeta{}, fmt.Errorf("could not extract title from GeeksForGeeks __NEXT_DATA__")
	}

	meta := ProblemMeta{
		Title:      strings.TrimSpace(prob.ProblemName),
		Difficulty: strings.ToLower(strings.TrimSpace(prob.Difficulty)),
	}

	for _, tag := range prob.Tags.TopicTags {
		name := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(tag)), "-", " ")
		if name != "" {
			meta.Topics = append(meta.Topics, name)
		}
	}
	meta.Topics = Dedup(meta.Topics)

	return meta, nil
}
