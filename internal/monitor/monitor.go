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

// resultIndicatorText returns a string representing any visible submission
// result on the page. Empty means no result is shown yet.
func resultIndicatorText(page *rod.Page, platform string) string {
	switch platform {
	case "leetcode":
		return elementExists(page, leetcodeSuccess, leetcodeFailure)
	default:
		return elementExists(page, neetcodeSuccess, neetcodeFailure)
	}
}

// ── NeetCode ─────────────────────────────────────────────────────────────────
//
// Submission result is an <h1> with one of these classes:
//
//	h1.submission-result-accepted  →  "Accepted"
//	h1.submission-result-wrong     →  "Wrong Answer" / "Time Limit Exceeded" / etc.
var neetcodeSuccess = []string{"h1.submission-result-accepted"}
var neetcodeFailure = []string{"h1.submission-result-wrong"}

func detectNeetCode(page *rod.Page) (bool, bool) {
	if elementPresent(page, neetcodeSuccess...) {
		return true, true
	}
	if elementPresent(page, neetcodeFailure...) {
		return false, true
	}
	return false, false
}

// ── LeetCode ─────────────────────────────────────────────────────────────────
//
// Tune these selectors after inspecting the live DOM on LeetCode.
var leetcodeSuccess = []string{"[data-e2e-locator='submission-result']"}
var leetcodeFailure = []string{".result-state--error", ".result-state--fail"}

func detectLeetCode(page *rod.Page) (bool, bool) {
	text := strings.ToLower(textOf(page, leetcodeSuccess...))
	if strings.Contains(text, "accepted") {
		return true, true
	}
	if elementPresent(page, leetcodeFailure...) {
		return false, true
	}
	return false, false
}

// ── DOM helpers ───────────────────────────────────────────────────────────────

// elementPresent returns true if any selector currently matches a DOM element.
// Uses page.Elements (non-blocking snapshot) instead of page.Element (which polls).
func elementPresent(page *rod.Page, selectors ...string) bool {
	for _, sel := range selectors {
		els, err := page.Elements(sel)
		if err == nil && len(els) > 0 {
			return true
		}
	}
	return false
}

// elementExists returns a non-empty string if any result selector is present now.
func elementExists(page *rod.Page, successSels, failureSels []string) string {
	if elementPresent(page, successSels...) {
		return "success"
	}
	if elementPresent(page, failureSels...) {
		return "failure"
	}
	return ""
}

// textOf returns combined text of elements currently matching each selector.
func textOf(page *rod.Page, selectors ...string) string {
	var parts []string
	for _, sel := range selectors {
		els, err := page.Elements(sel)
		if err != nil || len(els) == 0 {
			continue
		}
		t, err := els[0].Text()
		if err == nil && strings.TrimSpace(t) != "" {
			parts = append(parts, strings.TrimSpace(t))
		}
	}
	return strings.Join(parts, " ")
}
