# Testing

## Running tests

```sh
go test ./...              # all packages
go test ./internal/srs     # single package
go test -v ./internal/store # verbose output
go test -run TestPickNext  # specific test
```

---

## In-memory SQLite

All tests that need a database use `db.OpenMemory()`:

```go
func TestSomething(t *testing.T) {
    testDB, err := db.OpenMemory()
    if err != nil {
        t.Fatal(err)
    }
    defer testDB.Close()

    // testDB has all migrations applied and is empty
}
```

Each test gets a fresh, isolated database. No file cleanup required.

---

## Test patterns

### Pure function tests (`internal/srs`)

SM-2 logic has no dependencies — use table-driven tests:

```go
func TestUpdateSRS(t *testing.T) {
    tests := []struct {
        name       string
        state      ProblemSRS
        result     string
        duration   int
        difficulty string
        wantReps   int
        wantInterval int
    }{
        {"first success", defaultState, "success", 8, "easy", 1, 1},
        {"second success", afterFirstSuccess, "success", 8, "easy", 2, 6},
        {"failure resets", afterSecondSuccess, "fail", 0, "easy", 0, 1},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got := UpdateSRS(tc.state, tc.result, tc.duration, tc.difficulty)
            // assert ...
        })
    }
}
```

### Store / integration tests (`internal/store`)

Seed the in-memory DB and assert on query results:

```go
func TestPickNext(t *testing.T) {
    db, _ := db.OpenMemory()
    defer db.Close()

    store.InsertProblem(db, "leetcode", "two-sum", "Two Sum", "easy", nil)
    // set next_review_date to past to make it due
    db.Exec(`UPDATE problem_srs SET next_review_date = '2020-01-01'`)

    p, _, isDue, err := store.PickNext(db, store.PracticeFilter{})
    if err != nil || p == nil {
        t.Fatal("expected a problem")
    }
    if !isDue {
        t.Error("expected isDue=true")
    }
}
```

### Parser tests (`internal/importer`, `internal/platform`)

Use `os.CreateTemp` for file-based parsers, table-driven for URL parsing:

```go
func TestParseURL(t *testing.T) {
    tests := []struct {
        url      string
        platform string
        slug     string
        wantErr  bool
    }{
        {"https://leetcode.com/problems/two-sum/", "leetcode", "two-sum", false},
        {"https://neetcode.io/problems/two-sum/question", "neetcode", "two-sum", false},
        {"https://unknown.com/problems/foo", "", "", true},
    }
    for _, tc := range tests {
        t.Run(tc.url, func(t *testing.T) {
            plat, slug, err := platform.ParseURL(tc.url)
            // assert ...
        })
    }
}
```

---

## What not to test automatically

**Scraper and monitor tests** require a real browser and live problem sites. Don't write automated tests for these. Instead:

1. Add a `//go:build manual` build tag to any test that needs a browser
2. Document manual verification steps as a comment in the test file:

```go
//go:build manual

// To run: go test -tags manual -v ./internal/scraper
// Requires Chrome and a valid NeetCode session in the browser profile.
```

---

## CI conventions

- All tests in `go test ./...` must pass with no build tags
- No network calls in regular tests
- No file system side effects outside of `t.TempDir()` or `os.CreateTemp`
