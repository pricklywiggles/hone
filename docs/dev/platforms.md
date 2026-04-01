# Adding a New Platform

hone currently supports **LeetCode**, **NeetCode**, and **GeeksForGeeks**. Each platform is a single Go file in `internal/platform/` that implements the `Platform` interface and self-registers via `init()`.

---

## How to add a platform

Create a new file `internal/platform/newplatform.go`:

```go
package platform

type NewPlatform struct{}

func init() { Register(&NewPlatform{}) }
```

Implement all methods of the `Platform` interface:

| Method | Purpose |
|--------|---------|
| `Name()` | Canonical lowercase name (e.g. `"newplatform"`) |
| `Hostnames()` | All hostname variants (e.g. `["newplatform.com", "www.newplatform.com"]`) |
| `SlugFromPath(path)` | Extract problem slug from URL path |
| `URLTemplate()` | Default URL template with `{{slug}}` placeholder |
| `ExtraWait(page)` | Post-load delay if needed (no-op for most platforms) |
| `Scrape(page)` | Extract title, difficulty, topics from a loaded page |
| `DetectResult(page)` | Check for submission verdict (success/failure) |
| `ResultIndicatorText(page)` | Snapshot of result area for change detection |

### Scraping pattern

Extract raw data from the page (embedded JSON or DOM selectors), then delegate to a private pure function for parsing:

```go
func (NewPlatform) Scrape(page *rod.Page) (ProblemMeta, error) {
    raw := page.MustElement(`script#__NEXT_DATA__`).MustText()
    return parseNewPlatform(raw)
}

func parseNewPlatform(raw string) (ProblemMeta, error) {
    // pure parsing, testable without a browser
}
```

Topic names must normalize dashes to spaces and lowercase (e.g. `"Breadth-First-Search"` → `"breadth first search"`). Use `Dedup()` to remove duplicates.

### Monitor pattern

Define success/failure CSS selector slices, then use the shared DOM helpers:

```go
var newplatformSuccess = []string{".result-accepted"}
var newplatformFailure = []string{".result-wrong"}

func (NewPlatform) DetectResult(page *rod.Page) (bool, bool) {
    if ElementPresent(page, newplatformSuccess...) { return true, true }
    if ElementPresent(page, newplatformFailure...) { return false, true }
    return false, false
}

func (NewPlatform) ResultIndicatorText(page *rod.Page) string {
    return ElementExists(page, newplatformSuccess, newplatformFailure)
}
```

---

## Tests

Create `internal/platform/newplatform_test.go` with:

- `TestParseNewPlatform` — valid data, dashed topic normalization, empty title error, malformed input
- `TestNewPlatformSlugFromPath` — happy path and error cases
- `TestNewPlatformName`, `TestNewPlatformHostnames` — identity checks

Add URL test cases to `internal/platform/platform_test.go` for `ParseURL`.

Monitor selectors need manual verification against the live site.

---

## Checklist

- [ ] `internal/platform/newplatform.go` — implements `Platform`, calls `Register` in `init()`
- [ ] `internal/platform/newplatform_test.go` — parse function + slug + identity tests
- [ ] URL test cases in `internal/platform/platform_test.go`
- [ ] Topic names normalize dashes to spaces
- [ ] Manual verification: add a problem, run a practice session, check export output
