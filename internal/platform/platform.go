package platform

import (
	"fmt"
	"net/url"

	"github.com/go-rod/rod"
)

// ProblemMeta holds the metadata scraped from a problem page.
type ProblemMeta struct {
	Title      string
	Difficulty string   // "easy", "medium", or "hard"
	Topics     []string // normalized: lowercase, dashes replaced with spaces
}

// Platform defines the behaviour each coding-problem site must implement.
type Platform interface {
	Name() string
	Hostnames() []string
	SlugFromPath(path string) (string, error)
	URLTemplate() string
	LoginURL() string
	ExtraWait(page *rod.Page) error
	Scrape(page *rod.Page) (ProblemMeta, error)
	DetectResult(page *rod.Page) (success bool, found bool)
	ResultIndicatorText(page *rod.Page) string
}

// ParseURL extracts the platform name and problem slug from a URL.
// Returns error for unrecognized hosts or invalid paths.
func ParseURL(rawURL string) (platformName, slug string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	p := ForHost(u.Hostname())
	if p == nil {
		return "", "", fmt.Errorf("unrecognized platform host: %s", u.Hostname())
	}

	slug, err = p.SlugFromPath(u.Path)
	if err != nil {
		return "", "", err
	}

	return p.Name(), slug, nil
}
