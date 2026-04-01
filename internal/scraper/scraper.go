package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pricklywiggles/hone/internal/debuglog"
	"github.com/spf13/viper"
)

type ProblemMeta struct {
	Title      string
	Difficulty string   // "easy", "medium", or "hard"
	Topics     []string // e.g. ["array", "hash-table"]
}

// launchChrome starts Chrome as a normal process (headful) with a remote
// debugging port, then returns the DevTools WebSocket URL for Rod to connect.
// We launch externally (not via Rod's launcher) so Chrome can access the macOS
// Keychain to decrypt cookies saved during `hone auth`.
func launchChrome(chromePath, profileDir string) (wsURL string, cleanup func(), err error) {
	port, err := freePort()
	if err != nil {
		return "", nil, fmt.Errorf("find free port: %w", err)
	}

	cmd := exec.Command(chromePath,
		fmt.Sprintf("--remote-debugging-port=%d", port),
		fmt.Sprintf("--user-data-dir=%s", profileDir),
		"--no-first-run",
		"--no-default-browser-check",
	)
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start chrome: %w", err)
	}

	debugURL := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	var ws string
	for range 30 {
		time.Sleep(200 * time.Millisecond)
		resp, err := http.Get(debugURL)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var info struct {
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		if json.Unmarshal(body, &info) == nil && info.WebSocketDebuggerURL != "" {
			ws = info.WebSocketDebuggerURL
			break
		}
	}
	if ws == "" {
		cmd.Process.Kill()
		cmd.Wait()
		return "", nil, fmt.Errorf("chrome did not expose debug endpoint on port %d", port)
	}

	debuglog.Log("scrape: chrome started on port %d", port)
	return ws, func() {
		cmd.Process.Kill()
		cmd.Wait()
	}, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// Scrape fetches problem metadata from the platform page for the given slug.
// Launches Chrome headful with the persistent browser profile. Timeout: 30 seconds.
func Scrape(platform, slug, profileDir string) (ProblemMeta, error) {
	tmpl := viper.GetString("platforms." + platform + ".url_template")
	if tmpl == "" {
		return ProblemMeta{}, fmt.Errorf("no URL template configured for platform %q", platform)
	}
	pageURL := strings.ReplaceAll(tmpl, "{{slug}}", slug)

	chromePath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	u, killChrome, err := launchChrome(chromePath, profileDir)
	if err != nil {
		return ProblemMeta{}, fmt.Errorf("launch browser: %w", err)
	}
	defer killChrome()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(pageURL)
	page = page.Timeout(30 * time.Second)

	if err := page.WaitLoad(); err != nil {
		return ProblemMeta{}, fmt.Errorf("page load timeout: %w", err)
	}
	_ = page.WaitIdle(5 * time.Second)
	time.Sleep(3 * time.Second)

	debuglog.Log("scrape: start %s/%s", platform, slug)

	if os.Getenv("HONE_DEBUG") != "" {
		html, _ := page.HTML()
		homeDir, _ := os.UserHomeDir()
		os.WriteFile(filepath.Join(homeDir, ".local", "share", "hone", "debug-page.html"), []byte(html), 0644)
		debuglog.Log("scrape: saved page HTML to debug-page.html")
	}

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

	var raw string
	if err := rod.Try(func() {
		raw = page.MustElement(`script#__NEXT_DATA__`).MustText()
	}); err != nil {
		debuglog.Log("leetcode: __NEXT_DATA__ not found")
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
		debuglog.Log("leetcode: __NEXT_DATA__ found but no title extracted")
		return meta, fmt.Errorf("could not extract title from LeetCode __NEXT_DATA__")
	}
	debuglog.Log("leetcode: __NEXT_DATA__ succeeded (%s)", meta.Title)
	return meta, nil
}

func scrapeNeetCode(page *rod.Page) (ProblemMeta, error) {
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

	// Merge: prefer ng-state, fill gaps from DOM.
	meta := ngMeta
	if meta.Title == "" {
		meta.Title = domMeta.Title
	}
	if meta.Difficulty == "" {
		meta.Difficulty = domMeta.Difficulty
	}
	meta.Topics = dedup(append(meta.Topics, domMeta.Topics...))

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
