package platform

import (
	"testing"

	"github.com/go-rod/rod"
)

// stubPlatform is a minimal Platform implementation for registry tests.
type stubPlatform struct {
	name        string
	hostnames   []string
	urlTemplate string
}

func (s *stubPlatform) Name() string                                         { return s.name }
func (s *stubPlatform) Hostnames() []string                                  { return s.hostnames }
func (s *stubPlatform) SlugFromPath(path string) (string, error)             { return "", nil }
func (s *stubPlatform) URLTemplate() string                                  { return s.urlTemplate }
func (s *stubPlatform) ExtraWait(page *rod.Page) error                       { return nil }
func (s *stubPlatform) Scrape(page *rod.Page) (ProblemMeta, error)           { return ProblemMeta{}, nil }
func (s *stubPlatform) DetectResult(page *rod.Page) (success bool, found bool) { return false, false }
func (s *stubPlatform) ResultIndicatorText(page *rod.Page) string            { return "" }

// withCleanRegistry runs fn with an empty registry, restoring the original after.
func withCleanRegistry(fn func()) {
	mu.Lock()
	origPlatforms := platforms
	origHostIndex := hostIndex
	platforms = map[string]Platform{}
	hostIndex = map[string]Platform{}
	mu.Unlock()

	defer func() {
		mu.Lock()
		platforms = origPlatforms
		hostIndex = origHostIndex
		mu.Unlock()
	}()

	fn()
}

func TestRegisterAndGet(t *testing.T) {
	withCleanRegistry(func() {
		p := &stubPlatform{name: "testplat", hostnames: []string{"testplat.com"}, urlTemplate: "https://testplat.com/{{slug}}"}
		Register(p)

		got, err := Get("testplat")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Name() != "testplat" {
			t.Errorf("Name = %q, want %q", got.Name(), "testplat")
		}
	})
}

func TestGetUnknown(t *testing.T) {
	withCleanRegistry(func() {
		_, err := Get("nonexistent")
		if err == nil {
			t.Error("expected error for unknown platform")
		}
	})
}

func TestForHost(t *testing.T) {
	withCleanRegistry(func() {
		p := &stubPlatform{name: "testplat", hostnames: []string{"testplat.com", "www.testplat.com"}}
		Register(p)

		if got := ForHost("testplat.com"); got == nil {
			t.Error("ForHost returned nil for registered hostname")
		}
		if got := ForHost("www.testplat.com"); got == nil {
			t.Error("ForHost returned nil for www hostname")
		}
		if got := ForHost("unknown.com"); got != nil {
			t.Errorf("ForHost returned %v for unknown hostname", got)
		}
	})
}

func TestRegisterDuplicateNamePanics(t *testing.T) {
	withCleanRegistry(func() {
		Register(&stubPlatform{name: "dup", hostnames: []string{"dup.com"}})

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for duplicate name")
			}
		}()
		Register(&stubPlatform{name: "dup", hostnames: []string{"dup2.com"}})
	})
}

func TestRegisterDuplicateHostnamePanics(t *testing.T) {
	withCleanRegistry(func() {
		Register(&stubPlatform{name: "plat1", hostnames: []string{"shared.com"}})

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for duplicate hostname")
			}
		}()
		Register(&stubPlatform{name: "plat2", hostnames: []string{"shared.com"}})
	})
}

func TestDefaults(t *testing.T) {
	withCleanRegistry(func() {
		Register(&stubPlatform{name: "alpha", hostnames: []string{"alpha.com"}, urlTemplate: "https://alpha.com/{{slug}}"})
		Register(&stubPlatform{name: "beta", hostnames: []string{"beta.com"}, urlTemplate: "https://beta.com/p/{{slug}}"})

		defaults := Defaults()
		if len(defaults) != 2 {
			t.Fatalf("Defaults returned %d entries, want 2", len(defaults))
		}
		if defaults["platforms.alpha.url_template"] != "https://alpha.com/{{slug}}" {
			t.Errorf("alpha template = %q", defaults["platforms.alpha.url_template"])
		}
		if defaults["platforms.beta.url_template"] != "https://beta.com/p/{{slug}}" {
			t.Errorf("beta template = %q", defaults["platforms.beta.url_template"])
		}
	})
}
