# hone — Full Implementation Roadmap

## Context

The hone project foundation is complete: Go module, Cobra command skeleton (all stubs), Viper config, SQLite with goose migrations (full schema), and the SM-2 `UpdateEF` function with tests. This plan covers all remaining work to reach a fully functional CLI.

Phases are ordered by dependency — each builds on the previous. Within a phase, steps can often be parallelized. Each phase ends with a working, testable increment.

---

## Phase 1: SRS Engine (core logic, no UI)

**Goal:** Complete the spaced repetition engine so it can pick problems and update state after attempts. Pure logic + database queries, fully testable without TUI or browser.

**Files:** `internal/srs/srs.go`, `internal/srs/srs_test.go`

### 1.1 Quality mapping
- `QualityFromDuration(difficulty string, durationMin int) int` — returns 3/4/5 based on thresholds from Viper config
- Failure quality is handled by the caller (hardcoded to 1), not this function

### 1.2 SRS state update
- `UpdateSRS(state ProblemSRS, result string, durationMin int, difficulty string) ProblemSRS` — applies the full algorithm:
  - On success: increment rep count, compute interval (rep 1→1, rep 2→6, rep 3+→interval×EF), mastered_before boost, update EF
  - On failure: reset rep count to 0, interval to 1, update EF with quality=1, clamp EF≥1.3
  - Set next_review_date = today + interval_days
  - If interval > 60 days, set mastered_before = true
- Uses existing `UpdateEF`

### 1.3 Picker queries
- `PickNext(db *sqlx.DB, playlistID *int) (*Problem, bool, error)` — returns next problem + whether it's due or upcoming
  - Filter by active playlist (if set)
  - Due: `next_review_date <= today`, pick most overdue
  - If nothing due: pick nearest future review date
  - Return bool indicating due vs. upcoming (for TUI messaging)

### 1.4 Database helpers
- `RecordAttempt(db, problemID, startedAt, completedAt, result, durationSec, quality)` — insert attempt row
- `SaveSRSState(db, ProblemSRS)` — update problem_srs row

### 1.5 Tests
- Table-driven tests for `QualityFromDuration` (all difficulty×duration combos + boundaries)
- Tests for `UpdateSRS` covering: first success, second success, third+ success, failure, failure after mastery, mastered_before boost
- Picker tests using an in-memory SQLite DB with seeded data

---

## Phase 2: `hone add` — Problem Ingestion

**Goal:** Parse a URL, scrape problem metadata with Rod, and insert into the database.

**Files:** `internal/platform/platform.go`, `internal/scraper/scraper.go`, `cmd/add.go`

### 2.1 URL parser (`internal/platform/platform.go`)
- `ParseURL(rawURL string) (platform, slug string, err error)`
- Recognize LeetCode and NeetCode URL patterns, extract slug
- Return error for unrecognized URLs
- `BuildURL(platform, slug string) string` — construct URL from platform templates in Viper config

### 2.2 Scraper (`internal/scraper/scraper.go`)
- `Scrape(platform, slug string) (title, difficulty string, topics []string, err error)`
- Headless Rod: launch browser, navigate to problem URL, extract metadata from DOM
- Platform-specific selectors for LeetCode and NeetCode
- Timeout handling

### 2.3 Wire up `cmd/add.go`
- Accept URL as argument or prompt with huh form
- Parse URL → scrape → insert problem + topics (upsert topics, link via problem_topics)
- Print confirmation with problem details

### 2.4 Tests
- URL parser: table-driven tests with various URL formats
- Scraper: skip in CI (needs browser), manual verification instructions in test comments

---

## Phase 3: `hone playlist` — Playlist Management

**Goal:** Create, list, and select playlists. Store active selection in config.

**Files:** `cmd/playlist.go`, `internal/config/config.go`

### 3.1 `playlist create`
- Accept name as argument or prompt
- Insert into playlists table
- Optionally add problems to it (future: interactive picker)

### 3.2 `playlist list`
- Query all playlists with problem counts
- Print as formatted table

### 3.3 `playlist select`
- Accept name or ID
- Write active playlist to config file via `viper.Set` + `viper.WriteConfig`
- Add `config.ActivePlaylistID() *int` accessor

### 3.4 Tests
- Integration tests with in-memory DB for create/list
- Config write/read round-trip for select

---

## Phase 4: `hone practice` — Practice Sessions

**Goal:** Pick a problem, open it in the browser, monitor for completion, record the attempt.

**Files:** `internal/monitor/monitor.go`, `cmd/practice.go`

### 4.1 Browser monitor (`internal/monitor/monitor.go`)
- `Monitor(ctx context.Context, platform, slug string) <-chan Result` — returns channel
- Headful Rod: open problem URL, poll DOM for submission result indicators
- Platform-specific selectors for success/failure detection (LeetCode, NeetCode)
- Send `Result{Success bool, Timestamp time.Time}` on channel when detected
- Respect context cancellation

### 4.2 Practice flow in `cmd/practice.go`
- Call picker to get next problem
- Display problem info (title, difficulty, topics, due/upcoming status)
- Launch Rod browser via monitor
- Record start time
- Wait for result from channel (or user quit)
- Compute duration, derive quality, call `UpdateSRS`, `RecordAttempt`
- Print summary (result, duration, next review date)

### 4.3 Tests
- Mock-based test for the practice flow (mock picker, mock monitor channel)
- Monitor: manual verification (needs browser)

---

## Phase 5: TUI — Stats Dashboard & Navigation

**Goal:** Replace stub output with Bubble Tea TUI. Stats dashboard is the hub; navigate to other views.

**Files:** `internal/tui/` (new package), modifications to all `cmd/*.go`

### 5.1 TUI architecture (`internal/tui/`)
- `tui.go` — main model with view routing (enum: stats, practice, add, playlists, topics)
- `stats.go` — stats dashboard view
- `practice.go` — practice session view (wraps Phase 4 logic)
- `add.go` — add problem view (huh form)
- `playlists.go` — playlist management view
- `topics.go` — topic list/filter view

### 5.2 Stats dashboard
- Total problems, due today, mastered count
- Per-topic breakdown table (topic name, count, due, avg EF)
- Recent attempts list
- Keybindings: `p` practice, `a` add, `l` playlists, `t` topics, `q` quit

### 5.3 Practice view
- Problem info display
- Timer (updates every second via `tea.Tick`)
- Status: waiting for browser result
- Result from Rod channel arrives via `tea.Cmd` wrapping channel receive
- Post-result summary before returning to stats

### 5.4 Add problem view
- huh form: URL input field
- Scraping spinner/status
- Confirmation with scraped details

### 5.5 Playlists view
- List with selection
- Create form (huh)
- Select active playlist

### 5.6 Topics view
- Table of topics with problem counts
- Filter to show problems for a selected topic

### 5.7 Wire commands to TUI
- `cmd/root.go` RunE → launch TUI at stats view
- `cmd/practice.go` → launch TUI at practice view
- `cmd/add.go` → launch TUI at add view
- etc.

---

## Phase 6: `hone topics` — Topic Filtering

**Goal:** List topics, show problems per topic, filter stats by topic.

**Files:** `cmd/topics.go`, queries in `internal/srs/` or `internal/db/`

### 6.1 Topic queries
- List all topics with problem counts and due counts
- List problems for a given topic with SRS state

### 6.2 Wire into TUI topics view (from Phase 5.6)

---

## Phase 7: Distribution

**Goal:** Package for Homebrew installation.

**Files:** `.goreleaser.yaml`, separate tap repo

### 7.1 GoReleaser config
- macOS arm64 + amd64 builds
- Homebrew tap configuration

### 7.2 Homebrew tap repo
- Create `homebrew-tap` repo
- GoReleaser auto-generates formula on release

### 7.3 CI/CD
- GitHub Actions workflow: test → build → release on tag

---

## Summary: implementation order

| Phase | Deliverable | Depends on |
|-------|------------|------------|
| 1 | SRS engine (pick, update, record) | Foundation ✅ |
| 2 | `hone add` (URL parse, scrape, insert) | Foundation ✅ |
| 3 | `hone playlist` (create, list, select) | Foundation ✅ |
| 4 | `hone practice` (browser + SRS) | Phase 1, Phase 2 |
| 5 | TUI (stats dashboard + navigation) | Phases 1–4 |
| 6 | `hone topics` (filtering + display) | Phase 5 |
| 7 | Distribution (GoReleaser + Homebrew) | Phase 5 |

Phases 1, 2, and 3 are independent and can be built in any order or in parallel.
