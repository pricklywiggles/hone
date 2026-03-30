package monitor

import (
	"context"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Result is sent on the channel when a submission verdict is detected.
type Result struct {
	Success     bool
	CompletedAt time.Time
}

// Monitor opens a headful browser at problemURL using the persistent profile,
// polls the DOM every second for a submission result, and sends exactly one
// Result on the returned channel before closing it. The channel is also closed
// (without a value) if ctx is cancelled.
func Monitor(ctx context.Context, platform, problemURL, profileDir string) <-chan Result {
	ch := make(chan Result, 1)
	go func() {
		defer close(ch)
		run(ctx, platform, problemURL, profileDir, ch)
	}()
	return ch
}

func run(ctx context.Context, platform, problemURL, profileDir string, ch chan<- Result) {
	u := launcher.New().
		UserDataDir(profileDir).
		Headless(false).
		MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(problemURL)
	if err := page.WaitLoad(); err != nil {
		return
	}

	detect := detectorFor(platform)

	// Snapshot result-indicator text at startup so we don't fire on stale
	// verdicts that may already be visible from a previous session.
	initial := resultIndicatorText(page, platform)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Only check for a verdict if the result area changed since startup.
			if resultIndicatorText(page, platform) == initial {
				continue
			}
			if success, found := detect(page); found {
				ch <- Result{Success: success, CompletedAt: time.Now()}
				return
			}
		}
	}
}

// ── Platform-specific detection ──────────────────────────────────────────────
//
// Each detector returns (success, found). Update selectors here after
// inspecting the live DOM in DevTools.

type detector func(*rod.Page) (success bool, found bool)

func detectorFor(platform string) detector {
	switch platform {
	case "leetcode":
		return detectLeetCode
	default:
		return detectNeetCode
	}
}

// resultIndicatorText returns whatever result-related text is currently visible
// on the page, used to detect state changes between polls.
func resultIndicatorText(page *rod.Page, platform string) string {
	switch platform {
	case "leetcode":
		return textOf(page, leetcodeResultSelectors...)
	default:
		return textOf(page, neetcodeResultSelectors...)
	}
}

// ── NeetCode ─────────────────────────────────────────────────────────────────
//
// After submitting on NeetCode, a result panel appears below the editor.
// Tune these selectors by opening DevTools after a submission and finding
// the element that shows "Accepted" / "Wrong Answer" etc.
//
// Current best-guess selectors (update after first live test):
var neetcodeResultSelectors = []string{
	".result-container",
	".submission-result",
	"[class*='result']",
}

var neetcodeSuccess = []string{"Accepted"}
var neetcodeFailure = []string{"Wrong Answer", "Time Limit Exceeded", "Runtime Error", "Memory Limit Exceeded", "Compile Error", "Output Limit Exceeded"}

func detectNeetCode(page *rod.Page) (bool, bool) {
	text := strings.ToLower(textOf(page, neetcodeResultSelectors...))
	if text == "" {
		// Fallback: scan full body for result strings.
		text = strings.ToLower(bodyText(page))
	}
	for _, s := range neetcodeSuccess {
		if strings.Contains(text, strings.ToLower(s)) {
			return true, true
		}
	}
	for _, s := range neetcodeFailure {
		if strings.Contains(text, strings.ToLower(s)) {
			return false, true
		}
	}
	return false, false
}

// ── LeetCode ─────────────────────────────────────────────────────────────────
//
// LeetCode shows verdicts in a dedicated result panel.
// Tune selectors below after inspecting the live DOM.
var leetcodeResultSelectors = []string{
	"[data-e2e-locator='submission-result']",
	".result-state",
	"[class*='result-state']",
}

var leetcodeSuccess = []string{"Accepted"}
var leetcodeFailure = []string{"Wrong Answer", "Time Limit Exceeded", "Runtime Error", "Memory Limit Exceeded", "Compile Error", "Output Limit Exceeded"}

func detectLeetCode(page *rod.Page) (bool, bool) {
	text := strings.ToLower(textOf(page, leetcodeResultSelectors...))
	if text == "" {
		text = strings.ToLower(bodyText(page))
	}
	for _, s := range leetcodeSuccess {
		if strings.Contains(text, strings.ToLower(s)) {
			return true, true
		}
	}
	for _, s := range leetcodeFailure {
		if strings.Contains(text, strings.ToLower(s)) {
			return false, true
		}
	}
	return false, false
}

// ── DOM helpers ───────────────────────────────────────────────────────────────

// textOf returns combined text content of the first matching element for each
// selector, silently skipping selectors that match nothing.
func textOf(page *rod.Page, selectors ...string) string {
	var parts []string
	for _, sel := range selectors {
		rod.Try(func() {
			t := strings.TrimSpace(page.MustElement(sel).MustText())
			if t != "" {
				parts = append(parts, t)
			}
		})
	}
	return strings.Join(parts, " ")
}

func bodyText(page *rod.Page) string {
	var t string
	rod.Try(func() {
		t = page.MustElement("body").MustText()
	})
	return t
}
