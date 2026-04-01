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
	"github.com/pricklywiggles/hone/internal/platform"
	"github.com/spf13/viper"
)

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
func Scrape(b *Browser, platformName, slug string) (platform.ProblemMeta, error) {
	p, err := platform.Get(platformName)
	if err != nil {
		return platform.ProblemMeta{}, err
	}

	tmpl := viper.GetString("platforms." + platformName + ".url_template")
	pageURL := strings.ReplaceAll(tmpl, "{{slug}}", slug)

	var page *rod.Page
	if err := rod.Try(func() {
		page = b.browser.MustPage(pageURL)
	}); err != nil {
		return platform.ProblemMeta{}, fmt.Errorf("create page: %w", err)
	}
	defer page.Close()

	page = page.Timeout(30 * time.Second)

	if err := page.WaitLoad(); err != nil {
		return platform.ProblemMeta{}, fmt.Errorf("page load timeout: %w", err)
	}
	_ = page.WaitIdle(5 * time.Second)

	if err := p.ExtraWait(page); err != nil {
		return platform.ProblemMeta{}, fmt.Errorf("extra wait: %w", err)
	}

	debuglog.Log("scrape: start %s/%s", platformName, slug)

	if os.Getenv("HONE_DEBUG") != "" {
		html, _ := page.HTML()
		homeDir, _ := os.UserHomeDir()
		os.WriteFile(filepath.Join(homeDir, ".local", "share", "hone", "debug-page.html"), []byte(html), 0644)
		debuglog.Log("scrape: saved page HTML to debug-page.html")
	}

	return p.Scrape(page)
}
