package platform

import (
	"strings"

	"github.com/go-rod/rod"
)

// Dedup removes duplicate strings, preserving order.
func Dedup(ss []string) []string {
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

// ElementPresent returns true if any selector currently matches a DOM element.
func ElementPresent(page *rod.Page, selectors ...string) bool {
	for _, sel := range selectors {
		els, err := page.Elements(sel)
		if err == nil && len(els) > 0 {
			return true
		}
	}
	return false
}

// ElementExists returns a non-empty string if any result selector is present.
func ElementExists(page *rod.Page, successSels, failureSels []string) string {
	if ElementPresent(page, successSels...) {
		return "success"
	}
	if ElementPresent(page, failureSels...) {
		return "failure"
	}
	return ""
}

// TextOf returns combined text of elements currently matching each selector.
func TextOf(page *rod.Page, selectors ...string) string {
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
