package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/pricklywiggles/hone/internal/browser"
	"github.com/pricklywiggles/hone/internal/debuglog"
	"github.com/pricklywiggles/hone/internal/platform"
)

// Result is sent on the channel when a submission verdict is detected.
// If Err is non-nil, the monitor failed to start and no result was obtained.
type Result struct {
	Success     bool
	CompletedAt time.Time
	Err         error
}

// Session holds a long-lived browser instance that is reused across practice
// problems. Create one with NewSession, call Monitor for each problem, and
// Close when the practice session ends.
type Session struct {
	browser    *rod.Browser
	profileDir string
	mu         sync.Mutex
	closed     bool
}

// NewSession launches a headful Chrome instance using the persistent profile.
func NewSession(profileDir string) (*Session, error) {
	chromePath, err := browser.ChromePath()
	if err != nil {
		return nil, fmt.Errorf("monitor: %w", err)
	}

	var u string
	err = rod.Try(func() {
		u = launcher.NewUserMode().
			Bin(chromePath).
			UserDataDir(profileDir).
			MustLaunch()
	})
	if err != nil {
		return nil, fmt.Errorf("monitor: launch chrome: %w", err)
	}

	var browser *rod.Browser
	err = rod.Try(func() {
		browser = rod.New().ControlURL(u).MustConnect().NoDefaultDevice()
	})
	if err != nil {
		return nil, fmt.Errorf("monitor: connect to chrome: %w", err)
	}

	return &Session{browser: browser, profileDir: profileDir}, nil
}

// Monitor opens a new tab at problemURL, polls the DOM every second for a
// submission result, and sends exactly one Result on the returned channel
// before closing it. The browser stays open after a result is detected.
// The channel is closed without a value if ctx is cancelled.
func (s *Session) Monitor(ctx context.Context, platformName, problemURL string) <-chan Result {
	ch := make(chan Result, 1)
	go func() {
		defer close(ch)
		s.poll(ctx, platformName, problemURL, ch)
	}()
	return ch
}

// Close shuts down the browser. Safe to call multiple times or after a crash.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	rod.Try(func() { s.browser.MustClose() })
}

// reconnect tears down the stale browser handle and launches a fresh one.
// Called when the DevTools TCP connection has gone stale (e.g., after idle overnight).
func (s *Session) reconnect() error {
	rod.Try(func() { s.browser.MustClose() })

	chromePath, err := browser.ChromePath()
	if err != nil {
		return fmt.Errorf("reconnect: %w", err)
	}

	var u2 string
	err = rod.Try(func() {
		u2 = launcher.NewUserMode().
			Bin(chromePath).
			UserDataDir(s.profileDir).
			MustLaunch()
	})
	if err != nil {
		return fmt.Errorf("reconnect: launch chrome: %w", err)
	}

	var b *rod.Browser
	err = rod.Try(func() {
		b = rod.New().ControlURL(u2).MustConnect().NoDefaultDevice()
	})
	if err != nil {
		return fmt.Errorf("reconnect: connect to chrome: %w", err)
	}

	s.browser = b
	return nil
}

func (s *Session) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Session) poll(ctx context.Context, platformName, problemURL string, ch chan<- Result) {
	if s.isClosed() {
		ch <- Result{Err: fmt.Errorf("monitor: session already closed")}
		return
	}

	p, err := platform.Get(platformName)
	if err != nil {
		err = fmt.Errorf("monitor: %w", err)
		debuglog.Log("%v", err)
		ch <- Result{Err: err}
		return
	}

	var page *rod.Page
	err = rod.Try(func() {
		page = s.browser.MustPage(problemURL)
	})
	if err != nil {
		debuglog.Log("monitor: page open failed, attempting reconnect: %v", err)
		if reconErr := s.reconnect(); reconErr != nil {
			ch <- Result{Err: fmt.Errorf("monitor: reconnect failed: %w", reconErr)}
			return
		}
		err = rod.Try(func() {
			page = s.browser.MustPage(problemURL)
		})
		if err != nil {
			ch <- Result{Err: fmt.Errorf("monitor: open page after reconnect: %w", err)}
			return
		}
	}

	err = rod.Try(func() {
		page.MustWaitLoad()
	})
	if err != nil {
		err = fmt.Errorf("monitor: page load failed: %w", err)
		debuglog.Log("%v", err)
		ch <- Result{Err: err}
		return
	}

	if err := p.ExtraWait(page); err != nil {
		err = fmt.Errorf("monitor: platform wait failed: %w", err)
		debuglog.Log("%v", err)
		ch <- Result{Err: err}
		return
	}

	var initial string
	err = rod.Try(func() {
		initial = p.ResultIndicatorText(page)
	})
	if err != nil {
		err = fmt.Errorf("monitor: read initial indicator: %w", err)
		debuglog.Log("%v", err)
		ch <- Result{Err: err}
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var currentText string
			var readErr error
			readErr = rod.Try(func() {
				currentText = p.ResultIndicatorText(page)
			})
			if readErr != nil {
				err := fmt.Errorf("monitor: browser disconnected: %w", readErr)
				debuglog.Log("%v", err)
				ch <- Result{Err: err}
				return
			}
			if currentText == initial {
				continue
			}

			var success, found bool
			readErr = rod.Try(func() {
				success, found = p.DetectResult(page)
			})
			if readErr != nil {
				err := fmt.Errorf("monitor: browser disconnected: %w", readErr)
				debuglog.Log("%v", err)
				ch <- Result{Err: err}
				return
			}
			if found {
				ch <- Result{Success: success, CompletedAt: time.Now()}
				return
			}
		}
	}
}
