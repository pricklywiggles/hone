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

// Browser wraps a Chrome process and Rod connection for reuse across scrapes.
type Browser struct {
	browser    *rod.Browser
	killChrome func()
}

// NewBrowser launches a headless Chrome process and connects Rod to it.
// Call Close() when done to kill Chrome and clean up profile locks.
func NewBrowser(profileDir string) (*Browser, error) {
	chromePath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	wsURL, kill, err := launchChrome(chromePath, profileDir)
	if err != nil {
		return nil, err
	}

	var browser *rod.Browser
	if connectErr := rod.Try(func() {
		browser = rod.New().ControlURL(wsURL).MustConnect()
	}); connectErr != nil {
		kill()
		return nil, fmt.Errorf("connect to chrome: %w", connectErr)
	}

	return &Browser{browser: browser, killChrome: kill}, nil
}

func (b *Browser) Close() {
	if b.browser != nil {
		b.browser.MustClose()
	}
	if b.killChrome != nil {
		b.killChrome()
	}
}

// launchChrome starts Chrome via exec.Command with --headless=new and a remote
// debugging port, then returns the DevTools WebSocket URL for Rod to connect.
// We launch externally (not via Rod's launcher) so Chrome can access the macOS
// Keychain to decrypt cookies saved during `hone auth`.
func launchChrome(chromePath, profileDir string) (wsURL string, cleanup func(), err error) {
	port, err := freePort()
	if err != nil {
		return "", nil, fmt.Errorf("find free port: %w", err)
	}

	cmd := exec.Command(chromePath,
		"--headless=new",
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
		for _, f := range []string{"SingletonLock", "SingletonSocket", "SingletonCookie"} {
			os.Remove(filepath.Join(profileDir, f))
		}
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

// Scrape fetches problem metadata for the given platform/slug using the
// provided browser instance. Creates a new page, scrapes, then closes the page.
func Scrape(b *Browser, platform, slug string) (ProblemMeta, error) {
	tmpl := viper.GetString("platforms." + platform + ".url_template")
	if tmpl == "" {
		return ProblemMeta{}, fmt.Errorf("no URL template configured for platform %q", platform)
	}
	pageURL := strings.ReplaceAll(tmpl, "{{slug}}", slug)

	var page *rod.Page
	if err := rod.Try(func() {
		page = b.browser.MustPage(pageURL)
	}); err != nil {
		return ProblemMeta{}, fmt.Errorf("create page: %w", err)
	}
	defer page.Close()

	page = page.Timeout(30 * time.Second)

	if err := page.WaitLoad(); err != nil {
		return ProblemMeta{}, fmt.Errorf("page load timeout: %w", err)
	}
	_ = page.WaitIdle(5 * time.Second)
	if platform == "neetcode" {
		time.Sleep(3 * time.Second)
	}

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
	meta.Topics = dedup(meta.Topics)

	if meta.Title == "" {
		return meta, fmt.Errorf("could not extract title from LeetCode __NEXT_DATA__")
	}
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
	return parseNeetCodeNgState(raw)
}

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
