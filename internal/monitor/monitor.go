package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
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

// Monitor opens a headful browser at problemURL using the persistent profile,
// polls the DOM every second for a submission result, and sends exactly one
// Result on the returned channel before closing it. The channel is also closed
// (without a value) if ctx is cancelled.
func Monitor(ctx context.Context, platformName, problemURL, profileDir string) <-chan Result {
	ch := make(chan Result, 1)
	go func() {
		defer close(ch)
		run(ctx, platformName, problemURL, profileDir, ch)
	}()
	return ch
}

func run(ctx context.Context, platformName, problemURL, profileDir string, ch chan<- Result) {
	p, err := platform.Get(platformName)
	if err != nil {
		err = fmt.Errorf("monitor: %w", err)
		debuglog.Log("%v", err)
		ch <- Result{Err: err}
		return
	}

	chromePath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	u := launcher.NewUserMode().
		Bin(chromePath).
		UserDataDir(profileDir).
		MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect().NoDefaultDevice()
	defer browser.MustClose()

	page := browser.MustPage(problemURL)
	if err := page.WaitLoad(); err != nil {
		err = fmt.Errorf("monitor: page load failed: %w", err)
		debuglog.Log("%v", err)
		ch <- Result{Err: err}
		return
	}

	initial := p.ResultIndicatorText(page)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if p.ResultIndicatorText(page) == initial {
				continue
			}
			if success, found := p.DetectResult(page); found {
				ch <- Result{Success: success, CompletedAt: time.Now()}
				return
			}
		}
	}
}
