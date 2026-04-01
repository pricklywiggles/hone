# Adding a New Platform

hone currently supports **NeetCode** and **LeetCode**. Adding a new platform requires changes in four places.

---

## 1. URL parser (`internal/platform/platform.go`)

Add a case to the `switch` in `ParseURL`:

```go
case "newplatform.com", "www.newplatform.com":
    platform = "newplatform"
```

Make sure the path parsing logic still applies. The current parser expects `/problems/<slug>[/<extra>...]` — if the new platform uses a different path structure, add a platform-specific branch:

```go
case "newplatform":
    // path: /challenge/<slug>
    if len(parts) < 2 || parts[0] != "challenge" {
        return "", "", fmt.Errorf("unrecognized path: %s", u.Path)
    }
    return platform, parts[1], nil
```

Add table-driven tests to `internal/platform/platform_test.go`.

---

## 2. Scraper selectors (`internal/scraper/scraper.go`)

Add a case to the platform switch in `Scrape`, and a corresponding `scrapeNewPlatform(page)` function. Extract the JSON/DOM parsing into a pure function (`parseNewPlatform(raw string)`) so it can be unit-tested without a browser:

```go
case "newplatform":
    return scrapeNewPlatform(page)
```

```go
func scrapeNewPlatform(page *rod.Page) (ProblemMeta, error) {
    // extract raw data from the page
    raw := page.MustElement(`script#data`).MustText()
    return parseNewPlatform(raw)
}

func parseNewPlatform(raw string) (ProblemMeta, error) {
    // pure parsing logic, testable without a browser
}
```

Topic names must normalize dashes to spaces (e.g. `"breadth-first-search"` → `"breadth first search"`) to stay consistent with existing platforms and avoid duplicate topics.

Add unit tests for the parse function in `internal/scraper/scraper_test.go`. Selectors should also be verified manually against live pages.

---

## 3. Monitor selectors (`internal/monitor/monitor.go`)

Add a case in the polling loop for submission result detection:

```go
case "newplatform":
    // Detect accepted submission
    el, err := page.Element(".result-accepted")
    if err == nil && el != nil {
        return Result{Success: true, Timestamp: time.Now()}
    }
    // Detect failed submission
    el, err = page.Element(".result-wrong-answer")
    if err == nil && el != nil {
        return Result{Success: false, Timestamp: time.Now()}
    }
```

The monitor polls the DOM every second and returns as soon as a result is detected. Context cancellation (user presses `q`) is checked on each iteration.

---

## 4. URL template config (`internal/config/config.go`)

Add a default in `setDefaults`:

```go
viper.SetDefault("platforms.newplatform.url_template",
    "https://newplatform.com/challenge/{{slug}}/")
```

This template is used by `config.BuildURL(platform, slug)` when constructing URLs for export and display. The `{{slug}}` placeholder is replaced at runtime.

---

## Checklist

- [ ] `ParseURL` case in `internal/platform/platform.go`
- [ ] Tests in `internal/platform/platform_test.go`
- [ ] `Scrape` case and extracted parse function in `internal/scraper/scraper.go`
- [ ] Parse function tests in `internal/scraper/scraper_test.go`
- [ ] Topic names normalize dashes to spaces
- [ ] `Monitor` case in `internal/monitor/monitor.go`
- [ ] Default URL template in `internal/config/config.go`
- [ ] Manual verification: add a problem, run a practice session, check export output
